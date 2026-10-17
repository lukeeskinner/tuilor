package codeapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A cache fixture exercises publication without invoking Go or executing code.
func readyFixture(t *testing.T) *Project {
	t.Helper()
	p, err := Create(context.Background(), filepath.Join(t.TempDir(), "draft"), Spec{Name: "Custom app", Source: "package main\nfunc newModel() {}", Dependencies: []Dependency{}})
	if err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{"go.mod": "module fixture\n", "go.sum": "fixture sums\n", "data/state.json": `{"notes":["trial"]}`} {
		if err := os.WriteFile(filepath.Join(p.Dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(p.Dir, ".bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.binary(), []byte("cache fixture; never execute"), 0700); err != nil {
		t.Fatal(err)
	}
	p.GoVersion = "go1.25.5"
	p.BuildKey, err = p.fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	p.BinaryHash, err = fileHash(p.binary())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.writeManifest(); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestSaveNamedProjectPreservesDataAndIsIdempotent(t *testing.T) {
	p := readyFixture(t)
	root := t.TempDir()
	saved, err := p.SaveAs(context.Background(), root, "custom-app")
	if err != nil || saved.Draft || filepath.Base(saved.Dir) != "custom-app" {
		t.Fatal(saved, err)
	}
	again, err := p.Save(context.Background(), root)
	if err != nil || again.Dir != saved.Dir {
		t.Fatal("duplicate save", err)
	}
	reopened, err := Open(context.Background(), saved.Dir)
	if err != nil || !reopened.Ready() {
		t.Fatal("saved cache was lost", err)
	}
	data, err := os.ReadFile(filepath.Join(reopened.Dir, "data/state.json"))
	if err != nil || string(data) != `{"notes":["trial"]}` {
		t.Fatal("trial data lost", err)
	}
	other := readyFixture(t)
	if _, err := other.SaveAs(context.Background(), root, "custom-app"); err == nil {
		t.Fatal("existing app overwritten")
	}
}
func TestFailedOrCanceledSavePreservesDraft(t *testing.T) {
	p := readyFixture(t)
	original := p.Dir
	file := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Save(context.Background(), file); err == nil {
		t.Fatal("failed save accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Save(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !p.Draft || p.Dir != original || !p.Ready() {
		t.Fatal("failure lost candidate")
	}
	if _, err := p.SaveAs(context.Background(), t.TempDir(), "../escape"); err == nil {
		t.Fatal("unsafe ID accepted")
	}
}
func TestInvalidSourceNeverReachesCompiler(t *testing.T) {
	for _, source := range []string{"package main\nfunc newModel() {}\npackage main", "package other\nfunc newModel() {}", "package main\nfunc main() {}", "package main\nimport _ \"undeclared/module\"\nfunc newModel() {}"} {
		p, err := Create(context.Background(), filepath.Join(t.TempDir(), "draft"), Spec{Name: "Invalid", Source: source})
		if err != nil {
			t.Fatal(err)
		}
		err = p.Build(context.Background(), Toolchain{Path: "/no-compiler", standard: map[string]bool{}})
		if err == nil || strings.Contains(err.Error(), "prepare dependencies") || p.Ready() {
			t.Fatal("invalid source reached tooling", err)
		}
	}
}
func TestCompilerAndAppEnvironmentExcludeAISettings(t *testing.T) {
	for _, name := range []string{"OPENAI_API_KEY", "OPENAI_BASE_URL", "TUILOR_MODEL", "AZURE_OPENAI_API_KEY"} {
		t.Setenv(name, "fixture")
	}
	for _, env := range [][]string{Environment(), buildEnvironment()} {
		for _, entry := range env {
			key, _, _ := strings.Cut(entry, "=")
			if strings.HasPrefix(key, "OPENAI_") || strings.HasPrefix(key, "TUILOR_") || strings.HasPrefix(key, "AZURE_OPENAI_") {
				t.Fatalf("AI setting forwarded: %s", key)
			}
		}
	}
}
