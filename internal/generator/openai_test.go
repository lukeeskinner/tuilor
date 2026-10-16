package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func TestOpenAIRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/responses" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing authentication")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if body["model"] != "chosen-model" || body["instructions"] != "system rules" || body["input"] != "request data" || body["store"] != false {
			t.Errorf("unexpected request body: %+v", body)
		}
		if tools, ok := body["tools"]; ok {
			t.Errorf("AI must not receive callable tools: %v", tools)
		}
		text, ok := body["text"].(map[string]any)
		if !ok {
			t.Error("missing text format")
			return
		}
		format, ok := text["format"].(map[string]any)
		if !ok {
			t.Error("missing schema format")
			return
		}
		if format["type"] != "json_schema" || format["strict"] != true || format["name"] != "tuilor_app" {
			t.Errorf("unexpected format: %+v", format)
		}
		if schema, ok := format["schema"].(map[string]any); !ok || schema["additionalProperties"] != false {
			t.Error("missing app schema")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"candidate\":true}"}]}]}`)
	}))
	defer server.Close()
	provider, err := NewOpenAI("test-key", "chosen-model", option.WithBaseURL(server.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	output, err := provider.Generate(context.Background(), Request{Instructions: "system rules", Input: "request data", Schema: app.JSONSchema()})
	if err != nil || string(output) != `{"candidate":true}` {
		t.Fatalf("output %s; error %v", output, err)
	}
}

func TestOpenAIResponseFailures(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body, want string
	}{
		{"API error", 401, `{"error":{"message":"untrusted secret response","type":"invalid_request_error"}}`, "HTTP 401"},
		{"rate limit", 429, `{"error":{"message":"limited","type":"rate_limit_error"}}`, "HTTP 429"},
		{"server error", 500, `{"error":{"message":"failed","type":"server_error"}}`, "HTTP 500"},
		{"malformed envelope", 200, `{`, "OpenAI request failed"},
		{"incomplete", 200, `{"status":"incomplete","output":[]}`, "not completed"},
		{"failed", 200, `{"status":"failed","output":[]}`, "not completed"},
		{"refusal", 200, `{"status":"completed","output":[{"type":"message","content":[{"type":"refusal","refusal":"untrusted text"}]}]}`, "refused"},
		{"empty output", 200, `{"status":"completed","output":[]}`, "no app definition"},
		{"unexpected operation", 200, `{"status":"completed","output":[{"type":"function_call","name":"greet"}]}`, "unexpected output"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			defer server.Close()
			p, err := NewOpenAI("test-key", "", option.WithBaseURL(server.URL+"/"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = p.Generate(context.Background(), Request{})
			if err == nil || !strings.Contains(err.Error(), tt.want) || strings.Contains(err.Error(), "untrusted") {
				t.Fatalf("got %v; want %q", err, tt.want)
			}
			if tt.status >= 400 {
				var api *openai.Error
				if !errors.As(err, &api) || api.StatusCode != tt.status {
					t.Fatalf("lost API error: %v", err)
				}
			}
			if calls != 1 {
				t.Fatalf("unexpected retries: %d", calls)
			}
		})
	}
}

func TestOpenAICancellationAndTimeout(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint("timeout=", timeout), func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Read the body before waiting so request cancellation reaches the server.
				var body any
				_ = json.NewDecoder(r.Body).Decode(&body)
				close(started)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			p, err := NewOpenAI("test-key", "", option.WithBaseURL(server.URL+"/"))
			if err != nil {
				t.Fatal(err)
			}
			var ctx context.Context
			var cancel context.CancelFunc
			want := context.Canceled
			if timeout {
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)
				want = context.DeadlineExceeded
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()
			result := make(chan error, 1)
			go func() { _, err := p.Generate(ctx, Request{}); result <- err }()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("request did not start")
			}
			if !timeout {
				cancel()
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("got %v; want %v", err, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("request did not stop")
			}
		})
	}
}

func TestOpenAIEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := NewOpenAIFromEnv(); err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("got %v", err)
	}
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("TUILOR_MODEL", "")
	p, err := NewOpenAIFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if p.model != DefaultModel {
		t.Fatalf("default model: %q", p.model)
	}
	t.Setenv("TUILOR_MODEL", "custom-model")
	p, err = NewOpenAIFromEnv()
	if err != nil || p.model != "custom-model" {
		t.Fatalf("provider=%+v error=%v", p, err)
	}
}
