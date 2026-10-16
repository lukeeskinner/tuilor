// Package localapp owns app-scoped records and filesystem persistence.
package localapp

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lukeeskinner/tuilor/internal/app"
)

type Record struct {
	ID     string         `json:"id"`
	Values map[string]any `json:"values"`
}

type Data struct {
	Collections map[string][]Record `json:"collections"`
}

// Store snapshots are immutable to callers: Apply and Save return a replacement
// only after validation and any required disk write succeed. One TUI serializes
// writes; simultaneous editors of the same app are not supported.
type Store struct {
	Definition *app.Definition
	Path       string
	data       Data
}

func New(definition *app.Definition) (*Store, error) {
	if definition == nil || definition.Version != 2 {
		return nil, fmt.Errorf("local app requires version 2")
	}
	if err := definition.Validate(nil); err != nil {
		return nil, fmt.Errorf("validate local app: %w", err)
	}
	s := &Store{Definition: definition, data: Data{Collections: map[string][]Record{}}}
	for _, c := range definition.Collections {
		s.data.Collections[c.ID] = []Record{}
	}
	return s, nil
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

func Open(directory string) (*Store, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, fmt.Errorf("open local app: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local app must be a directory")
	}
	definitionJSON, err := readRegular(filepath.Join(directory, "app.json"))
	if err != nil {
		return nil, fmt.Errorf("read local definition: %w", err)
	}
	definition, err := app.Parse(definitionJSON, nil)
	if err != nil {
		return nil, err
	}
	s, err := New(definition)
	if err != nil {
		return nil, err
	}
	dataJSON, err := readRegular(filepath.Join(directory, "data.json"))
	if err != nil {
		return nil, fmt.Errorf("read local data (preserved): %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(dataJSON))
	decoder.DisallowUnknownFields()
	// Decode into an empty value: decoding into New's initialized maps would
	// retain default collections when corrupt files omit them.
	var data Data
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("decode local data (preserved): %w", err)
	}
	s.data = data
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing local data (preserved)")
	}
	if err := s.validateData(); err != nil {
		return nil, fmt.Errorf("invalid local data (preserved): %w", err)
	}
	s.Path, err = filepath.Abs(directory)
	return s, err
}

// Records returns a copy so rendering cannot mutate the persisted snapshot.
func (s *Store) Records(collection string) []Record {
	rows := s.data.Collections[collection]
	copyRows := make([]Record, len(rows))
	for i, r := range rows {
		values := make(map[string]any, len(r.Values))
		for k, v := range r.Values {
			values[k] = v
		}
		copyRows[i] = Record{r.ID, values}
	}
	return copyRows
}

func (s *Store) validateData() error {
	if len(s.data.Collections) != len(s.Definition.Collections) {
		return fmt.Errorf("collection set does not match definition")
	}
	for _, c := range s.Definition.Collections {
		rows, ok := s.data.Collections[c.ID]
		if !ok || rows == nil {
			return fmt.Errorf("missing collection %q", c.ID)
		}
		ids := map[string]bool{}
		for _, r := range rows {
			if strings.TrimSpace(r.ID) == "" || ids[r.ID] || len(r.Values) != len(c.Fields) {
				return fmt.Errorf("invalid record in %q", c.ID)
			}
			ids[r.ID] = true
			for _, f := range c.Fields {
				if !app.ValueMatches(f.Type, r.Values[f.ID]) {
					return fmt.Errorf("record %q: invalid field %q", r.ID, f.ID)
				}
			}
		}
	}
	return nil
}

func (s *Store) Apply(action app.LocalAction, inputs map[string]string, selected string) (*Store, error) {
	fields := map[string]bool{}
	for name := range inputs {
		fields[name] = true
	}
	if err := s.Definition.ValidateLocalAction(action, fields); err != nil {
		return nil, err
	}
	next := &Store{Definition: s.Definition, Path: s.Path, data: Data{Collections: map[string][]Record{}}}
	for id := range s.data.Collections {
		next.data.Collections[id] = s.Records(id)
	}
	rows := next.data.Collections[action.Collection]
	switch action.Kind {
	case "insert":
		values := map[string]any{}
		collection, _ := s.Definition.Collection(action.Collection)
		for _, f := range collection.Fields {
			if f.Type == "boolean" {
				values[f.ID] = false
			}
		}
		for _, b := range action.Bindings {
			values[b.Parameter] = inputs[b.Field]
		}
		rows = append(rows, Record{rand.Text(), values})
	case "update", "delete":
		index := -1
		for i, r := range rows {
			if r.ID == selected {
				index = i
				break
			}
		}
		if index < 0 {
			return nil, fmt.Errorf("select an existing record first")
		}
		if action.Kind == "delete" {
			rows = append(rows[:index], rows[index+1:]...)
		} else {
			for _, v := range action.Values {
				rows[index].Values[v.Field] = v.Value
			}
		}
	}
	next.data.Collections[action.Collection] = rows
	if err := next.validateData(); err != nil {
		return nil, err
	}
	if next.Path != "" {
		if err := writeJSON(filepath.Join(next.Path, "data.json"), next.data); err != nil {
			return nil, fmt.Errorf("save record change: %w", err)
		}
	}
	return next, nil
}

// Save publishes a complete app directory, never a half-written library entry.
func (s *Store) Save(directory string) (*Store, error) {
	if s.Path != "" {
		return s, nil
	}
	if err := s.validateData(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, fmt.Errorf("create app library: %w", err)
	}
	stage, err := os.MkdirTemp(directory, ".pending-")
	if err != nil {
		return nil, fmt.Errorf("stage local app: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := writeJSON(filepath.Join(stage, "app.json"), s.Definition); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(stage, "data.json"), s.data); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(filepath.Join(directory, "app-"+rand.Text()))
	if err != nil {
		return nil, err
	}
	if err := os.Rename(stage, path); err != nil {
		return nil, fmt.Errorf("publish local app: %w", err)
	}
	next := *s
	next.Path = path
	return &next, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".write-")
	if err != nil {
		return fmt.Errorf("create temporary data file: %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync data: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replace data: %w", err)
	}
	return nil
}
