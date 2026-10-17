package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lukeeskinner/tuilor/internal/codeapp"
)

// The library contains host-published Go projects, never declarative UI files.
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
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		label := name
		data, err := os.ReadFile(path)
		var header struct {
			App struct {
				Name string `json:"name"`
			} `json:"app"`
		}
		if err == nil {
			_ = json.Unmarshal(data, &header)
		}
		if strings.TrimSpace(header.App.Name) != "" {
			label = header.App.Name + " (" + name + ")"
		}
		m.files = append(m.files, name)
		m.fileLabels = append(m.fileLabels, label)
	}
}

func loadApp(ctx context.Context, path string) loadedAppMsg {
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
