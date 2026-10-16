package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func Save(definition *Definition, directory string) (path string, err error) {
	if definition == nil || definition.Version != 1 {
		return "", fmt.Errorf("flat JSON saves require a version 1 MCP app; local apps must be saved as bundles")
	}
	data, err := json.MarshalIndent(definition, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode app: %w", err)
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", fmt.Errorf("create app directory: %w", err)
	}
	file, err := os.CreateTemp(directory, "app-*.json")
	if err != nil {
		return "", fmt.Errorf("create app file: %w", err)
	}
	success := false
	defer func() {
		if !success {
			file.Close()
			os.Remove(file.Name())
		}
	}()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return "", fmt.Errorf("write app: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close app: %w", err)
	}
	path, err = filepath.Abs(file.Name())
	if err != nil {
		return "", fmt.Errorf("resolve saved app path: %w", err)
	}
	success = true
	return path, nil
}
