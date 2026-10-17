package codeapp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/lukeeskinner/tuilor/internal/filelock"
)

type revisionManifest struct {
	Kind     string `json:"kind"`
	Version  int    `json:"version"`
	Name     string `json:"name"`
	Active   string `json:"active_revision"`
	Previous string `json:"previous_revision,omitempty"`
}

func revisionID(id string) bool { return strings.HasPrefix(id, "r-") && appID.MatchString(id) }
func readRevision(root string) (revisionManifest, error) {
	raw, err := readRegular(filepath.Join(root, "manifest.json"))
	if err != nil {
		return revisionManifest{}, err
	}
	var m revisionManifest
	if json.Unmarshal(raw, &m) != nil || m.Kind != "go" || m.Version != 2 || !revisionID(m.Active) || (m.Previous != "" && !revisionID(m.Previous)) {
		return m, fmt.Errorf("invalid revision manifest; preserved")
	}
	return m, nil
}
func openRevision(ctx context.Context, root string) (*Project, error) {
	m, err := readRevision(root)
	if err != nil {
		return nil, err
	}
	parent, statErr := os.Lstat(filepath.Join(root, "revisions"))
	if statErr != nil || !parent.IsDir() {
		return nil, fmt.Errorf("revision storage is missing or not a directory")
	}
	dir := filepath.Join(root, "revisions", m.Active)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("active app revision is missing or not a directory")
	}
	p, err := Open(ctx, dir)
	if err != nil {
		return nil, err
	}
	if p.Spec.DataSchema == "" {
		return nil, fmt.Errorf("revision has no data contract")
	}
	p.Root, err = filepath.Abs(root)
	p.BaseRevision = m.Active
	return p, err
}
func (p *Project) Path() string {
	if p.Root != "" && !p.Draft {
		return p.Root
	}
	return p.Dir
}
func (p *Project) Acquire() (func(), error) {
	directory := p.Dir
	if p.Root != "" {
		directory = p.Root
	}
	filename := filepath.Join(directory, ".lock")
	if info, err := os.Lstat(filename); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("app lock is not a regular file")
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open app lock: %w", err)
	}
	if err = filelock.Lock(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("app is already open or being changed: %w", err)
	}
	return func() { _ = filelock.Unlock(f); _ = f.Close() }, nil
}
func (p *Project) checkActive(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.Root == "" || p.Draft {
		return nil
	}
	m, err := readRevision(p.Root)
	if err != nil {
		return err
	}
	if filepath.Base(p.Dir) != m.Active {
		return fmt.Errorf("app changed in another session; reopen it")
	}
	return nil
}
func (p *Project) CopyTrial(ctx context.Context, old *Project) error {
	if old == nil {
		return nil
	}
	if old.Spec.DataSchema != "" && !sameSchema(p.Spec.DataSchema, old.Spec.DataSchema) {
		return fmt.Errorf("revision changed the data schema; migrations are not supported")
	}
	release, err := old.Acquire()
	if err != nil {
		return err
	}
	defer release()
	if err = old.checkActive(ctx); err != nil {
		return err
	}
	if err = old.ValidateData(); err != nil {
		return err
	}
	if err = CopyData(ctx, filepath.Join(old.Dir, "data"), filepath.Join(p.Dir, "data")); err != nil {
		return err
	}
	if err = p.ValidateData(); err != nil {
		return err
	}
	p.Bindings = maps.Clone(old.Bindings)
	if old.Root != "" {
		p.Root = old.Root
		p.BaseRevision = old.BaseRevision
		p.BaseData, err = fileHash(filepath.Join(old.Dir, "data", "state.json"))
		if err != nil {
			return err
		}
	}
	if old.Draft {
		p.BaseData = old.BaseData
	}
	return p.writeManifest()
}
func copyProject(ctx context.Context, p *Project, directory string) error {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	for _, name := range p.sourceFiles() {
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := readRegular(filepath.Join(p.Dir, name))
		if err != nil {
			return err
		}
		if err = atomicFile(filepath.Join(directory, name), b, 0600); err != nil {
			return err
		}
	}
	if err := CopyData(ctx, filepath.Join(p.Dir, "data"), filepath.Join(directory, "data")); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(directory, ".bin"), 0700); err != nil {
		return err
	}
	b, err := readRegular(p.binary())
	if err != nil {
		return err
	}
	return atomicFile(filepath.Join(directory, ".bin", filepath.Base(p.binary())), b, 0700)
}
func publishPointer(root string, m revisionManifest) error {
	b, _ := json.MarshalIndent(m, "", "  ")
	return atomicFile(filepath.Join(root, "manifest.json"), b, 0600)
}
func (p *Project) saveVersioned(ctx context.Context, directory, id string) (*Project, error) {
	if !p.Ready() {
		return nil, fmt.Errorf("compile before saving")
	}
	if err := p.ValidateData(); err != nil {
		return nil, err
	}
	if p.Root != "" {
		return p.publishRevision(ctx)
	}
	if id == "" {
		id = "go-" + strings.ToLower(rand.Text())
	}
	root, err := filepath.Abs(filepath.Join(directory, id))
	if err != nil {
		return nil, err
	}
	if _, err = os.Lstat(root); !os.IsNotExist(err) {
		return nil, fmt.Errorf("app ID already exists or cannot be checked")
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(directory, ".pending-go-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	revision := "r-" + strings.ToLower(rand.Text())
	target := filepath.Join(stage, "revisions", revision)
	if err = copyProject(ctx, p, target); err != nil {
		return nil, err
	}
	if err = publishPointer(stage, revisionManifest{"go", 2, p.Spec.Name, revision, ""}); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if _, err = os.Lstat(root); !os.IsNotExist(err) {
		return nil, fmt.Errorf("app ID already exists or cannot be checked")
	}
	if err = os.Rename(stage, root); err != nil {
		return nil, fmt.Errorf("publish app: %w", err)
	}
	p.Dir = filepath.Join(root, "revisions", revision)
	p.Root = root
	p.BaseRevision = revision
	p.Draft = false
	return p, nil
}
func (p *Project) publishRevision(ctx context.Context) (*Project, error) {
	release, err := p.Acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	m, err := readRevision(p.Root)
	if err != nil {
		return nil, err
	}
	if m.Active != p.BaseRevision {
		return nil, fmt.Errorf("app changed during trial; reopen it before revising")
	}
	current := filepath.Join(p.Root, "revisions", m.Active, "data", "state.json")
	hash, err := fileHash(current)
	if err != nil || hash != p.BaseData {
		return nil, fmt.Errorf("saved data changed during trial; revision was not published")
	}
	id := "r-" + strings.ToLower(rand.Text())
	target := filepath.Join(p.Root, "revisions", id)
	if err = copyProject(ctx, p, target); err != nil {
		_ = os.RemoveAll(target)
		return nil, err
	}
	if err = ctx.Err(); err == nil {
		err = publishPointer(p.Root, revisionManifest{"go", 2, p.Spec.Name, id, m.Active})
	}
	if err != nil {
		_ = os.RemoveAll(target)
		return nil, err
	}
	p.Dir = target
	p.Draft = false
	p.BaseRevision = id
	p.BaseData = ""
	return p, nil
}
func (p *Project) Previous() bool {
	if p.Root == "" || p.Draft {
		return false
	}
	m, err := readRevision(p.Root)
	return err == nil && m.Previous != ""
}
func (p *Project) Restore(ctx context.Context, workspace string) (*Project, error) {
	if p.Root == "" || p.Draft {
		return nil, fmt.Errorf("no saved revision to restore")
	}
	m, err := readRevision(p.Root)
	if err != nil {
		return nil, err
	}
	if m.Previous == "" {
		return nil, fmt.Errorf("no previous revision")
	}
	old, err := Open(ctx, filepath.Join(p.Root, "revisions", m.Previous))
	if err != nil {
		return nil, err
	}
	draft, err := Create(ctx, filepath.Join(workspace, "restore-"+rand.Text()), old.Spec)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(draft.Dir)
		}
	}()
	if err = draft.CopyTrial(ctx, p); err != nil {
		return nil, err
	}
	tool, err := Preflight(ctx)
	if err != nil {
		return nil, err
	}
	if err = draft.Build(ctx, tool); err != nil {
		return nil, err
	}
	saved, err := draft.publishRevision(ctx)
	success = err == nil
	return saved, err
}
func (p *Project) SaveBindings(ctx context.Context) error {
	if err := p.checkActive(ctx); err != nil {
		return err
	}
	return p.writeManifest()
}
