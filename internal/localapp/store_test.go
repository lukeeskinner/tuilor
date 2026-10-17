package localapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lukeeskinner/tuilor/internal/capability"
)

func openFixture(t *testing.T) *Store {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "task-manager")
	if err := os.CopyFS(dir, os.DirFS("../../examples/task-manager")); err != nil {
		t.Fatal(err)
	}
	b, e := Read(context.Background(), dir)
	if e != nil {
		t.Fatal(e)
	}
	s, e := Open(context.Background(), b)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func call(op, id string, values map[string]any) capability.Call {
	return capability.Call{ID: "local.records." + op, Collection: "tasks", RecordID: id, Values: values}
}
func TestRecordLifecycleAndReopen(t *testing.T) {
	ctx := context.Background()
	s := openFixture(t)
	if other, e := Open(ctx, s.Bundle); e == nil {
		other.Close()
		t.Fatal("second writer accepted")
	}
	rows, e := s.Execute(ctx, call("insert", "", map[string]any{"title": "same", "priority": "High", "due": "2026-12-01"}))
	if e != nil {
		t.Fatal(e)
	}
	first := rows[0].ID
	rows, e = s.Execute(ctx, call("insert", "", map[string]any{"title": "same"}))
	if e != nil {
		t.Fatal(e)
	}
	if rows[1].ID == first {
		t.Fatal("duplicate record IDs")
	}
	rows, e = s.Execute(ctx, call("update", first, map[string]any{"completed": true, "priority": "Low"}))
	if e != nil || rows[0].Values["completed"] != true {
		t.Fatal(rows, e)
	}
	deletion := call("delete", first, nil)
	if _, e = s.Execute(ctx, deletion); e == nil {
		t.Fatal("unapproved delete accepted")
	}
	deletion.Approved = true
	rows, e = s.Execute(ctx, deletion)
	if e != nil || len(rows) != 1 || rows[0].ID == first {
		t.Fatal(rows, e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	b, e := Read(ctx, s.Bundle.Dir)
	if e != nil {
		t.Fatal(e)
	}
	reopened, e := Open(ctx, b)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	rows, e = reopened.Execute(ctx, call("list", "", nil))
	if e != nil || len(rows) != 1 {
		t.Fatal(rows, e)
	}
}
func TestFailedMutationAndCorruptionPreserved(t *testing.T) {
	ctx := context.Background()
	s := openFixture(t)
	path := filepath.Join(s.Bundle.Dir, "data.json")
	before, _ := os.ReadFile(path)
	s.write = func(context.Context, string, []byte) error { return errors.New("disk full") }
	if _, e := s.Execute(ctx, call("insert", "", map[string]any{"title": "lost"})); e == nil {
		t.Fatal("write failure ignored")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("failed write changed data")
	}
	s.write = atomicWrite
	for _, bad := range []string{`{corrupt`, string(before) + `garbage`} {
		if e := os.WriteFile(path, []byte(bad), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := s.Execute(ctx, call("insert", "", map[string]any{"title": "overwrite"})); e == nil {
			t.Fatal("corrupt data accepted")
		}
		after, _ = os.ReadFile(path)
		if string(after) != bad {
			t.Fatal("corruption overwritten")
		}
	}
}
func TestInvalidOperationsAndCanceledMutation(t *testing.T) {
	s := openFixture(t)
	ctx := context.Background()
	for _, c := range []capability.Call{call("update", "missing", map[string]any{"title": "x"}), call("insert", "", map[string]any{"title": "x", "due": "tomorrow"}), call("insert", "", map[string]any{"title": "x", "shell": "oops"}), {ID: "mcp.execute", Collection: "tasks"}} {
		if _, e := s.Execute(ctx, c); e == nil {
			t.Fatal("bad call accepted")
		}
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	before, err := os.ReadFile(filepath.Join(s.Bundle.Dir, "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, e := s.Execute(canceled, call("insert", "", map[string]any{"title": "canceled"})); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	after, err := os.ReadFile(filepath.Join(s.Bundle.Dir, "data.json"))
	if err != nil || string(after) != string(before) {
		t.Fatal("canceled mutation changed saved data", err)
	}
}
