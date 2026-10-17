// Package localapp owns app bundles and transactional local record operations.
package localapp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/capability"
)

type Manifest struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`
	ID      string `json:"id"`
	App     struct {
		Name string `json:"name"`
	} `json:"app"`
}
type Bundle struct {
	Dir        string
	Definition app.Definition
}
type dataset struct {
	Version    int                 `json:"version"`
	Collection string              `json:"collection"`
	Records    []capability.Record `json:"records"`
}
type Store struct {
	mu     sync.Mutex
	Bundle Bundle
	lock   *os.File
	closed bool
	write  func(context.Context, string, []byte) error
}

var appID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,79}$`)
var recordID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func readRegular(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("expected a regular file up to %d bytes", limit)
	}
	return os.ReadFile(path)
}
func atomicWrite(ctx context.Context, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return fmt.Errorf("stage file: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}
func encode(v any) []byte { b, _ := json.MarshalIndent(v, "", "  "); return append(b, '\n') }
func Read(ctx context.Context, dir string) (Bundle, error) {
	if err := ctx.Err(); err != nil {
		return Bundle{}, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return Bundle{}, fmt.Errorf("read app directory: %w", err)
	}
	if !info.IsDir() {
		return Bundle{}, fmt.Errorf("app is not a regular directory")
	}
	raw, err := readRegular(filepath.Join(dir, "manifest.json"), 1024*1024)
	if err != nil {
		return Bundle{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	if err := app.Decode(raw, &manifest); err != nil {
		return Bundle{}, err
	}
	if manifest.Kind != "declarative" || manifest.Version != 1 || !appID.MatchString(manifest.ID) || manifest.ID != filepath.Base(dir) {
		return Bundle{}, fmt.Errorf("unsupported or invalid app manifest")
	}
	raw, err = readRegular(filepath.Join(dir, "app.json"), 1024*1024)
	if err != nil {
		return Bundle{}, fmt.Errorf("read app definition: %w", err)
	}
	definition, err := app.Parse(raw, capability.Local())
	if err != nil {
		return Bundle{}, err
	}
	if manifest.App.Name != definition.Name {
		return Bundle{}, fmt.Errorf("manifest and definition names do not match")
	}
	return Bundle{Dir: dir, Definition: definition}, nil
}
func acquire(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("app lock is not a regular file")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open app lock: %w", err)
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("app is already open or cannot be locked: %w", err)
	}
	return f, nil
}
func release(f *os.File) error {
	if f == nil {
		return nil
	}
	return errors.Join(unlockFile(f), f.Close())
}
func Open(ctx context.Context, b Bundle) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.Definition.Validate(capability.Local()); err != nil {
		return nil, err
	}
	f, err := acquire(filepath.Join(b.Dir, ".lock"))
	if err != nil {
		return nil, err
	}
	store := &Store{Bundle: b, lock: f, write: atomicWrite}
	if _, err := store.read(ctx); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return release(s.lock)
}
func (s *Store) read(ctx context.Context) ([]capability.Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := readRegular(filepath.Join(s.Bundle.Dir, "data.json"), 16*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("read app data (preserved): %w", err)
	}
	var data dataset
	// Data has a separate, larger bound than definitions.
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("corrupt app data (preserved): %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("trailing corrupt app data (preserved)")
	}
	if data.Version != 1 || data.Collection != s.Bundle.Definition.Collection.ID || data.Records == nil || len(data.Records) > 10000 {
		return nil, fmt.Errorf("invalid app data version or collection (preserved)")
	}
	seen := map[string]bool{}
	for _, r := range data.Records {
		if !recordID.MatchString(r.ID) || seen[r.ID] {
			return nil, fmt.Errorf("invalid/duplicate record ID (data preserved)")
		}
		seen[r.ID] = true
		if err := s.Bundle.Definition.ValidateValues(r.Values, false); err != nil {
			return nil, fmt.Errorf("invalid record (data preserved): %w", err)
		}
	}
	return data.Records, nil
}
func (s *Store) Execute(ctx context.Context, call capability.Call) ([]capability.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, fmt.Errorf("app storage is closed")
	}
	desc, ok := capability.Local().Lookup(call.ID)
	if !ok || call.Collection != s.Bundle.Definition.Collection.ID {
		return nil, fmt.Errorf("unavailable capability or collection")
	}
	if desc.Operation == "list" && (call.RecordID != "" || len(call.Values) > 0) {
		return nil, fmt.Errorf("list does not accept a record or values")
	}
	if desc.Operation == "insert" && call.RecordID != "" {
		return nil, fmt.Errorf("insert cannot choose record IDs")
	}
	if desc.Operation == "delete" && (!call.Approved || len(call.Values) > 0) {
		return nil, fmt.Errorf("deletion requires explicit approval and no values")
	}
	if (desc.Operation == "insert" || desc.Operation == "update") && len(call.Values) == 0 {
		return nil, fmt.Errorf("record values are required")
	}
	if err := s.Bundle.Definition.ValidateValues(call.Values, true); err != nil {
		return nil, err
	}
	records, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	if desc.Operation == "list" {
		return records, nil
	}
	next := make([]capability.Record, len(records))
	for i, r := range records {
		next[i] = capability.Record{ID: r.ID, Values: maps.Clone(r.Values)}
	}
	switch desc.Operation {
	case "insert":
		if len(next) >= 10000 {
			return nil, fmt.Errorf("collection has reached its 10,000-record limit")
		}
		values := s.Bundle.Definition.Defaults()
		maps.Copy(values, call.Values)
		if err := s.Bundle.Definition.ValidateValues(values, false); err != nil {
			return nil, err
		}
		var id [16]byte
		_, _ = rand.Read(id[:])
		next = append(next, capability.Record{ID: fmt.Sprintf("%x", id), Values: values})
	case "update", "delete":
		index := -1
		for i, r := range next {
			if r.ID == call.RecordID {
				index = i
				break
			}
		}
		if index < 0 {
			return nil, fmt.Errorf("selected record no longer exists")
		}
		if desc.Operation == "delete" {
			next = append(next[:index], next[index+1:]...)
		} else {
			maps.Copy(next[index].Values, call.Values)
			if err := s.Bundle.Definition.ValidateValues(next[index].Values, false); err != nil {
				return nil, err
			}
		}
	}
	data := encode(dataset{1, s.Bundle.Definition.Collection.ID, next})
	if len(data) > 16*1024*1024 {
		return nil, fmt.Errorf("app data exceeds 16 MiB")
	}
	if err := s.write(ctx, filepath.Join(s.Bundle.Dir, "data.json"), data); err != nil {
		return nil, fmt.Errorf("save record change: %w", err)
	}
	return next, nil
}
