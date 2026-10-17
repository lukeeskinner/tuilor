// Package codeapp owns generated Go projects. Compilation is not a sandbox.
package codeapp

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/version"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Dependency struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

var uiDependencies = []Dependency{
	{"github.com/charmbracelet/bubbletea", "v1.3.10"},
	{"github.com/charmbracelet/bubbles", "v1.0.0"},
	{"github.com/charmbracelet/lipgloss", "v1.1.0"},
}

type Toolchain struct {
	Path, Version string
	standard      map[string]bool
}

// Environment removes generation credentials from compiler and app processes.
func Environment() []string {
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key) // Environment names are case-insensitive on Windows.
		if strings.HasPrefix(key, "OPENAI_") || strings.HasPrefix(key, "TUILOR_") || strings.HasPrefix(key, "AZURE_OPENAI_") {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func buildEnvironment() []string {
	var env []string
	for _, entry := range Environment() {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		switch key {
		case "GOFLAGS", "GOTOOLCHAIN", "CGO_ENABLED", "GOWORK", "GOENV", "GOOS", "GOARCH", "GOTOOLEXEC":
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GOFLAGS=", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOWORK=off", "GOENV=off", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
}

type limitedOutput struct{ data []byte }

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	const limit = 32 * 1024
	if len(p) >= limit {
		b.data = append(b.data[:0], p[len(p)-limit:]...)
	} else {
		if drop := len(b.data) + len(p) - limit; drop > 0 {
			copy(b.data, b.data[drop:])
			b.data = b.data[:len(b.data)-drop]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

func command(ctx context.Context, directory, path string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir, cmd.Env = directory, buildEnvironment()
	cmd.WaitDelay = 2 * time.Second
	var output limitedOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if ctx.Err() != nil {
		return string(output.data), ctx.Err()
	}
	return string(output.data), err
}

func Preflight(ctx context.Context) (Toolchain, error) {
	path, err := exec.LookPath("go")
	if err != nil {
		return Toolchain{}, fmt.Errorf("Go 1.25.5 or later is required to generate or rebuild an app: %w", err)
	}
	output, err := command(ctx, "", path, "version")
	if err != nil {
		return Toolchain{}, fmt.Errorf("check Go toolchain: %w", err)
	}
	fields := strings.Fields(output)
	if len(fields) < 3 || !version.IsValid(fields[2]) || version.Compare(fields[2], "go1.25.5") < 0 {
		return Toolchain{}, fmt.Errorf("Go 1.25.5 or later is required; found %s", strings.TrimSpace(output))
	}
	output, err = command(ctx, "", path, "list", "std")
	if err != nil {
		return Toolchain{}, fmt.Errorf("list standard library: %w\n%s", err, output)
	}
	standard := map[string]bool{}
	for _, name := range strings.Fields(output) {
		standard[name] = true
	}
	return Toolchain{path, fields[2], standard}, nil
}

var modulePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)
var versionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?(?:\+incompatible)?$`)

func validateDependencies(deps []Dependency) error {
	if len(deps) > 16 {
		return fmt.Errorf("at most 16 additional dependencies are supported")
	}
	seen := map[string]bool{}
	for _, d := range uiDependencies {
		seen[d.Module] = true
	}
	for _, d := range deps {
		if !modulePattern.MatchString(d.Module) || !strings.Contains(d.Module, ".") || !versionPattern.MatchString(d.Version) || seen[d.Module] {
			return fmt.Errorf("invalid, duplicate, or pinned dependency %q at %q", d.Module, d.Version)
		}
		for _, part := range strings.Split(d.Module, "/") {
			if part == "" || part == "." || part == ".." {
				return fmt.Errorf("invalid module path %q", d.Module)
			}
		}
		seen[d.Module] = true
	}
	return nil
}

func validateSource(source string, deps []Dependency, tool Toolchain) error {
	file, err := parser.ParseFile(token.NewFileSet(), "app.go", source, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return fmt.Errorf("parse generated app: %w", err)
	}
	if file.Name.Name != "main" {
		return fmt.Errorf("app.go must use package main")
	}
	factory := false
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Recv == nil {
			if f.Name.Name == "main" || f.Name.Name == "init" {
				return fmt.Errorf("Tuilor owns startup; do not declare %s", f.Name.Name)
			}
			if f.Name.Name == "newModel" {
				factory = true
			}
		}
	}
	if !factory {
		return fmt.Errorf("app.go must implement newModel() (tea.Model, error)")
	}
	for _, group := range file.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(strings.TrimSpace(c.Text), "//go:") {
				return fmt.Errorf("generated compiler directives are unsupported")
			}
		}
	}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return err
		}
		allowed := tool.standard[path]
		for _, d := range append(append([]Dependency{}, uiDependencies...), deps...) {
			if path == d.Module || strings.HasPrefix(path, d.Module+"/") {
				allowed = true
			}
		}
		if !allowed {
			return fmt.Errorf("undeclared import %q; declare requested third-party dependencies", path)
		}
	}
	return nil
}

func (p *Project) Build(ctx context.Context, tool Toolchain) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateSource(p.Spec.Source, p.Spec.Dependencies, tool); err != nil {
		return err
	}
	if err := p.writeSource(); err != nil {
		return err
	}
	module := "module tuilor.generated/app\n\ngo 1.25.5\n\nrequire (\n"
	for _, d := range append(append([]Dependency{}, uiDependencies...), p.Spec.Dependencies...) {
		module += fmt.Sprintf("\t%s %s\n", d.Module, d.Version)
	}
	module += ")\n"
	locked, lockErr := p.moduleFingerprint()
	if lockErr != nil || p.ModuleKey == "" || locked != p.ModuleKey {
		if err := atomicFile(filepath.Join(p.Dir, "go.mod"), []byte(module), 0600); err != nil {
			return fmt.Errorf("write module: %w", err)
		}
	}
	if out, err := command(ctx, p.Dir, tool.Path, "mod", "tidy"); err != nil {
		return fmt.Errorf("prepare dependencies: %w\n%s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(p.Dir, ".bin"), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Join(p.Dir, ".bin"), ".build-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	file.Close()
	defer os.Remove(temporary)
	if out, err := command(ctx, p.Dir, tool.Path, "build", "-mod=readonly", "-trimpath", "-o", temporary, "main.go", "app.go"); err != nil {
		return fmt.Errorf("compile app: %w\n%s", err, out)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Chmod(temporary, 0700); err != nil {
		return err
	}
	if err := os.Rename(temporary, p.binary()); err != nil {
		return err
	}
	p.GoVersion = tool.Version
	p.ModuleKey, err = p.moduleFingerprint()
	if err != nil {
		return err
	}
	p.BuildKey, err = p.fingerprint()
	if err != nil {
		return err
	}
	p.BinaryHash, err = fileHash(p.binary())
	if err != nil {
		return err
	}
	return p.writeManifest()
}

func (p *Project) Run(ctx context.Context, in io.Reader, out, stderr io.Writer) error {
	if !p.Ready() {
		return fmt.Errorf("app needs a successful build before it can run")
	}
	cmd := exec.CommandContext(ctx, p.binary())
	cmd.Dir, cmd.Env = p.Dir, Environment()
	cmd.Stdin, cmd.Stdout = in, out
	var diagnostics limitedOutput
	cmd.Stderr = io.MultiWriter(stderr, &diagnostics)
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Start()
	if err == nil {
		stop := forwardResize(cmd.Process)
		defer stop()
		err = cmd.Wait()
	}
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("run generated app: %w", ctx.Err())
		}
		return fmt.Errorf("generated app exited: %w\n%s", err, string(diagnostics.data))
	}
	return nil
}
