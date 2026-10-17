package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukeeskinner/tuilor/internal/generator"
)

type providerFunc func(context.Context, generator.Request) ([]byte, error)

func (f providerFunc) Generate(ctx context.Context, r generator.Request) ([]byte, error) {
	return f(ctx, r)
}
func TestCreatePreflightAndExistingIDsBlockPaidRequests(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	calls := 0
	factory := func() (generator.Provider, error) {
		return providerFunc(func(context.Context, generator.Request) ([]byte, error) {
			calls++
			t.Fatal("generation happened before preflight")
			return nil, nil
		}), nil
	}
	root := t.TempDir()
	cmd := newCreateWithProvider(factory)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"custom app", "--output-dir", root, "--name", "my-app"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "Go 1.25.5") || calls != 0 {
		t.Fatal("missing Go incurred generation", err, calls)
	}
	if err := os.Mkdir(filepath.Join(root, "my-app"), 0700); err != nil {
		t.Fatal(err)
	}
	retry := newCreateWithProvider(factory)
	retry.SetOut(new(bytes.Buffer))
	retry.SetErr(new(bytes.Buffer))
	retry.SetArgs([]string{"custom app", "--output-dir", root, "--name", "my-app"})
	if err := retry.Execute(); err == nil || !strings.Contains(err.Error(), "already exists") || calls != 0 {
		t.Fatal("existing ID incurred generation", err, calls)
	}
}
func TestCreateMissingCredentialsDoesNotPublish(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	root := t.TempDir()
	cmd := newCreateCommand()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"custom app", "--output-dir", root})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("missing credentials published an app", err)
	}
}
func TestRunRejectsUnsafeIDs(t *testing.T) {
	for _, id := range []string{"../outside", "/tmp/app", "with space"} {
		cmd := newRunCommand()
		cmd.SetArgs([]string{id})
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		if err := cmd.Execute(); err == nil {
			t.Fatal("unsafe ID accepted")
		}
	}
}
