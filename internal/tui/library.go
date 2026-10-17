package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lukeeskinner/tuilor/internal/codeapp"
	"github.com/lukeeskinner/tuilor/internal/localapp"
)

// Formats are distinguished by the host manifest, never guessed from app data.
func (m *Model) readLibrary() {
	m.files, m.fileLabels, m.err = nil, nil, nil
	entries, err := os.ReadDir(m.appsDir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		m.err = fmt.Errorf("read app directory: %w", err)
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(m.appsDir, name, "manifest.json")
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
			continue
		}
		label := name
		data, err := os.ReadFile(path)
		var header struct {
			Kind string `json:"kind"`
			App  struct {
				Name string `json:"name"`
			} `json:"app"`
		}
		if err == nil {
			_ = json.Unmarshal(data, &header)
		}
		if strings.TrimSpace(header.App.Name) != "" {
			label = header.App.Name + " (" + name + ")"
		}
		if header.Kind == "go" {
			label += " · Go app"
		} else if header.Kind != "declarative" {
			continue
		}
		m.files = append(m.files, name)
		m.fileLabels = append(m.fileLabels, label)
	}
}

func loadApp(ctx context.Context, path string) loadedAppMsg {
	manifest := filepath.Join(path, "manifest.json")
	info, err := os.Lstat(manifest)
	if err != nil {
		return loadedAppMsg{err: fmt.Errorf("read app manifest: %w", err)}
	}
	if !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return loadedAppMsg{err: fmt.Errorf("app manifest must be a regular file up to 1 MiB")}
	}
	raw, err := os.ReadFile(manifest)
	if err != nil {
		return loadedAppMsg{err: err}
	}
	var header struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return loadedAppMsg{err: fmt.Errorf("read app manifest: %w", err)}
	}
	if header.Kind == "declarative" {
		bundle, err := localapp.Read(ctx, path)
		return loadedAppMsg{local: &bundle, err: err}
	}
	if header.Kind != "go" {
		return loadedAppMsg{err: fmt.Errorf("unsupported saved app format")}
	}
	project, err := codeapp.Open(ctx, path)
	return loadedAppMsg{code: project, err: err}
}

func (m *Model) returnToLibrary(savedPath string) {
	m.screen = "apps"
	m.readLibrary()
	m.appCursor = max(0, min(m.appCursor, len(m.files)-1))
	for i, name := range m.files {
		if name == filepath.Base(savedPath) {
			m.appCursor = i
			break
		}
	}
}
