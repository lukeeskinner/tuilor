package generator

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOpenAIContractAndErrors(t *testing.T) {
	raw := codeResponse(codeReference)
	calls := 0
	transport := roundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		format := body["text"].(map[string]any)["format"].(map[string]any)
		if format["type"] != "json_schema" || format["strict"] != true {
			t.Fatal("structured outputs not strict")
		}
		b, _ := json.Marshal(map[string]any{"id": "resp_fixture", "status": "completed", "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": string(raw)}}}}})
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(b))), Request: r}, nil
	})
	p, err := NewOpenAI("fixture-key", "fixture-model", option.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateCode(context.Background(), p, "notes app", nil, ""); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	p, _ = NewOpenAI("fixture-key", "fixture-model", option.WithHTTPClient(&http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"secret response body"}}`)), Request: r}, nil
	})}))
	_, err = p.Generate(context.Background(), Request{})
	var api *openai.Error
	if err == nil || !errors.As(err, &api) || strings.Contains(err.Error(), "secret") {
		t.Fatal("API error not safely preserved", err)
	}
	p, _ = NewOpenAI("fixture-key", "fixture-model", option.WithHTTPClient(&http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}))
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err = p.Generate(ctx, Request{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := NewOpenAIFromEnv(); err == nil {
		t.Fatal("missing key accepted")
	}
}
