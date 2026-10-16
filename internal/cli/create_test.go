package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEscapeTerminalControls(t *testing.T) {
	got := EscapeTerminalControls("hello\x1b[2J\n\u009bworld")
	want := `hello\u001b[2J\u000a\u009bworld`
	if got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}

// The test binary doubles as a real stdio MCP server, without a shell or build
// dependency. A tool-call marker makes the discovery-only guarantee observable.
func TestMain(m *testing.M) {
	if os.Getenv("TUILOR_TEST_MCP_MODE") == "server" {
		server := mcp.NewServer(&mcp.Implementation{Name: "generator-fixture", Version: "1"}, nil)
		server.AddTool(&mcp.Tool{Name: "greet", Description: "Greets someone by name", InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"],"additionalProperties":false}`)}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			_ = os.WriteFile(os.Getenv("TUILOR_TEST_MCP_CALLED"), []byte("called"), 0600)
			return nil, fmt.Errorf("generation must never call tools")
		})
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(os.Getenv("TUILOR_TEST_MCP_STOPPED"), []byte("stopped"), 0600); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestCreateEndToEndWithoutAPICharges(t *testing.T) {
	data, err := os.ReadFile("../../examples/greeting.json")
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, valid := range []bool{true, false} {
		t.Run(fmt.Sprint("valid=", valid), func(t *testing.T) {
			dir := t.TempDir()
			called, stopped := filepath.Join(dir, "called"), filepath.Join(dir, "stopped")
			t.Setenv("TUILOR_TEST_MCP_MODE", "server")
			t.Setenv("TUILOR_TEST_MCP_CALLED", called)
			t.Setenv("TUILOR_TEST_MCP_STOPPED", stopped)
			t.Setenv("OPENAI_API_KEY", "test-key")
			t.Setenv("TUILOR_MODEL", "test-model")
			var apiCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiCalls.Add(1)
				if _, err := os.Stat(stopped); err != nil {
					t.Error("MCP process must exit before AI request")
				}
				var body struct {
					Input string `json:"input"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				var input struct {
					Capabilities []app.Capability `json:"capabilities"`
				}
				if err := json.Unmarshal([]byte(body.Input), &input); err != nil {
					t.Error(err)
				}
				if len(input.Capabilities) != 1 || input.Capabilities[0].Name != "greet" || !bytes.Contains(input.Capabilities[0].InputSchema, []byte(`"name"`)) {
					t.Errorf("missing discovered schema: %+v", input)
				}
				candidate := string(data)
				if !valid {
					candidate = strings.Replace(candidate, `"tool": "greet"`, `"tool":"unknown"`, 1)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": candidate}}}}})
			}))
			defer server.Close()
			t.Setenv("OPENAI_BASE_URL", server.URL+"/")
			outputDir := filepath.Join(dir, "apps")
			root := NewRootCommand()
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetArgs([]string{"create", "Build a greeting app", "--server", executable, "--output-dir", outputDir})
			err := root.Execute()
			if apiCalls.Load() != 1 {
				t.Fatalf("API calls=%d", apiCalls.Load())
			}
			if _, statErr := os.Stat(called); !os.IsNotExist(statErr) {
				t.Fatalf("MCP tool was called: %v", statErr)
			}
			if !valid {
				if err == nil || !strings.Contains(err.Error(), "unknown MCP tool") {
					t.Fatalf("invalid output: %v", err)
				}
				if _, statErr := os.Stat(outputDir); !os.IsNotExist(statErr) {
					t.Fatal("invalid definition was saved")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			path := strings.TrimSpace(output.String())
			if filepath.Dir(path) != outputDir {
				t.Fatalf("unexpected output %q", path)
			}
			saved, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := app.Parse(saved, []app.Capability{{Name: "greet", InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)}}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreateArgumentAndConfigurationErrors(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"create"}, "accepts 1 arg"},
		{[]string{"create", "App"}, "required flag"},
		{[]string{"create", " ", "--server", "missing"}, "must not be empty"},
		{[]string{"create", "App", "--server", "missing", "--timeout", "0s"}, "must be positive"},
		{[]string{"create", "App", "--server", "missing"}, "OPENAI_API_KEY"},
	} {
		root := NewRootCommand()
		root.SetArgs(tt.args)
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("args=%v got %v; want %q", tt.args, err, tt.want)
		}
	}
	t.Setenv("OPENAI_API_KEY", "test-key")
	root := NewRootCommand()
	root.SetArgs([]string{"create", "App", "--server", filepath.Join(t.TempDir(), "missing")})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "connect to MCP server") {
		t.Fatalf("got %v", err)
	}
}
