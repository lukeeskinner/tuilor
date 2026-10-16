package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/localapp"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
)

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
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(m.appsDir, name)
		if entry.IsDir() {
			path = filepath.Join(path, "app.json")
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
		} else if !entry.Type().IsRegular() || filepath.Ext(name) != ".json" {
			continue
		}
		label := name
		data, err := os.ReadFile(path)
		var header struct {
			Name string `json:"name"`
		}
		if err == nil && json.Unmarshal(data, &header) == nil && strings.TrimSpace(header.Name) != "" {
			label = header.Name + " (" + name + ")"
		}
		m.files = append(m.files, name)
		m.fileLabels = append(m.fileLabels, label)
	}
}

func loadApp(ctx context.Context, path, server string) loadedAppMsg {
	info, err := os.Lstat(path)
	if err != nil {
		return loadedAppMsg{err: fmt.Errorf("read app: %w", err)}
	}
	if info.IsDir() {
		store, err := localapp.Open(path)
		if err != nil {
			return loadedAppMsg{err: err}
		}
		return loadedAppMsg{definition: store.Definition, local: store, err: ctx.Err()}
	}
	if !info.Mode().IsRegular() {
		return loadedAppMsg{err: fmt.Errorf("app must be a regular JSON file or local app directory")}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return loadedAppMsg{err: fmt.Errorf("read app: %w", err)}
	}
	// Reject malformed files before connecting. Version 2 owns its own data bundle.
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return loadedAppMsg{err: fmt.Errorf("decode app JSON: %w", err)}
	}
	if header.Version == 2 {
		return loadedAppMsg{err: fmt.Errorf("local apps must be directories containing app.json and data.json")}
	}
	if strings.TrimSpace(server) == "" {
		return loadedAppMsg{err: fmt.Errorf("this MCP app requires --server /path/to/trusted-mcp-server")}
	}
	tools, err := mcpclient.ListTools(ctx, server)
	if err != nil {
		return loadedAppMsg{err: err}
	}
	definition, err := app.Parse(data, capabilitiesFor(tools))
	if err == nil {
		err = ctx.Err()
	}
	return loadedAppMsg{definition: definition, tools: tools, err: err}
}
