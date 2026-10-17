package codeapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Spec struct {
	Name         string       `json:"name"`
	Source       string       `json:"source,omitempty"`
	Dependencies []Dependency `json:"dependencies"`
}

type Project struct {
	Dir        string `json:"-"`
	Draft      bool   `json:"-"`
	Kind       string `json:"kind"`
	Version    int    `json:"version"`
	Spec       Spec   `json:"app"`
	GoVersion  string `json:"go_version"`
	BuildKey   string `json:"build_key"`
	BinaryHash string `json:"binary_hash"`
	ModuleKey  string `json:"module_key"`
}

func (s Spec) Validate() error {
	if strings.TrimSpace(s.Name) == "" || len(s.Name) > 200 || strings.TrimSpace(s.Source) == "" || len(s.Source) > 256*1024 {
		return fmt.Errorf("generated app requires a name (up to 200 bytes) and source (up to 256 KiB)")
	}
	return validateDependencies(s.Dependencies)
}

func Create(ctx context.Context, directory string, spec Spec) (*Project, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return nil, fmt.Errorf("create draft project: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(directory)
		}
	}()
	p := &Project{Dir: directory, Draft: true, Kind: "go", Version: 1, Spec: spec}
	if err := p.writeSource(); err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(directory, "data"), 0700); err != nil {
		return nil, err
	}
	if err := p.writeManifest(); err != nil {
		return nil, err
	}
	complete = true
	return p, nil
}

func (p *Project) writeSource() error {
	source := []byte(p.Spec.Source)
	if formatted, err := format.Source(source); err == nil {
		source = formatted
	}
	for name, data := range map[string][]byte{"app.go": source, "main.go": []byte(Starter)} {
		if err := atomicFile(filepath.Join(p.Dir, name), data, 0600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

func (p *Project) writeManifest() error {
	metadata := *p
	metadata.Spec.Source = "" // app.go is the source of truth, not a duplicate in JSON.
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return atomicFile(filepath.Join(p.Dir, "manifest.json"), data, 0600)
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("expected regular file: %s", path)
	}
	return os.ReadFile(path)
}

func Open(ctx context.Context, directory string) (*Project, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, fmt.Errorf("open generated app: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("generated app must be a directory")
	}
	data, err := readRegular(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read generated project: %w", err)
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode project manifest: %w", err)
	}
	if p.Kind != "go" || p.Version != 1 {
		return nil, fmt.Errorf("unsupported generated project format")
	}
	source, err := readRegular(filepath.Join(directory, "app.go"))
	if err != nil {
		return nil, err
	}
	// Source is editable; the cache key detects changes before it can run.
	p.Spec.Source = string(source)
	if err := p.Spec.Validate(); err != nil {
		return nil, err
	}
	p.Dir, err = filepath.Abs(directory)
	return &p, err
}

func (p *Project) binary() string {
	name := "app"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(p.Dir, ".bin", name)
}

func fileHash(path string) (string, error) {
	data, err := readRegular(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (p *Project) fingerprint() (string, error) {
	h := sha256.New()
	io.WriteString(h, Starter+"\x00"+runtime.GOOS+"/"+runtime.GOARCH+"\x00"+p.GoVersion)
	deps, _ := json.Marshal(p.Spec.Dependencies)
	h.Write(deps)
	for _, name := range []string{"main.go", "app.go", "go.mod", "go.sum"} {
		data, err := readRegular(filepath.Join(p.Dir, name))
		if err != nil {
			return "", err
		}
		io.WriteString(h, "\x00"+name+"\x00")
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (p *Project) moduleFingerprint() (string, error) {
	h := sha256.New()
	deps, _ := json.Marshal(p.Spec.Dependencies)
	h.Write(deps)
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := readRegular(filepath.Join(p.Dir, name))
		if err != nil {
			return "", err
		}
		io.WriteString(h, "\x00"+name+"\x00")
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (p *Project) Ready() bool {
	if p == nil || p.BuildKey == "" || p.BinaryHash == "" {
		return false
	}
	key, err := p.fingerprint()
	if err != nil || key != p.BuildKey {
		return false
	}
	hash, err := fileHash(p.binary())
	return err == nil && hash == p.BinaryHash
}

func atomicFile(path string, data []byte, mode fs.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".write-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// CopyData also serves repair: source changes must not discard trial records.
func CopyData(ctx context.Context, from, to string) error {
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("app data contains a non-regular file: %s", rel)
		}
		data, err := readRegular(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
}

func (p *Project) Save(ctx context.Context, directory string) (*Project, error) {
	if !p.Draft {
		return p, nil
	}
	if !p.Ready() {
		return nil, fmt.Errorf("compile successfully before saving")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(directory, ".pending-go-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	for _, name := range []string{"manifest.json", "app.go", "main.go", "go.mod", "go.sum"} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := readRegular(filepath.Join(p.Dir, name))
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(stage, name), data, 0600); err != nil {
			return nil, err
		}
	}
	if err := CopyData(ctx, filepath.Join(p.Dir, "data"), filepath.Join(stage, "data")); err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(stage, ".bin"), 0700); err != nil {
		return nil, err
	}
	data, err := readRegular(p.binary())
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stage, ".bin", filepath.Base(p.binary())), data, 0700); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(filepath.Join(directory, "go-"+rand.Text()))
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.Rename(stage, path); err != nil {
		return nil, fmt.Errorf("publish generated app: %w", err)
	}
	next := *p
	next.Dir, next.Draft = path, false
	return &next, nil
}
