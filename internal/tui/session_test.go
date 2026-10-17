package tui

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestLocalSessionRestoresAlternateScreenAndReleasesStorage(t *testing.T) {
	_, store := runtimeFixture(t, "../../examples/task-manager/app.json")
	bundle := store.Bundle
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := RunLocal(ctx, bundle, strings.NewReader("\x03"), &output); err != nil {
		t.Fatal(err)
	}
	for _, sequence := range []string{"\x1b[?1049h", "\x1b[?1049l", "\x1b[?25h"} {
		if !strings.Contains(output.String(), sequence) {
			t.Fatalf("missing terminal lifecycle sequence %q", sequence)
		}
	}
	// Reopening the same bundle proves shutdown released its single-writer lock.
	if err := RunLocal(ctx, bundle, strings.NewReader("\x1b"), &output); err != nil {
		t.Fatal(err)
	}
}
