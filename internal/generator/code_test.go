package generator

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lukeeskinner/tuilor/internal/codeapp"
)

type fakeProvider func(context.Context, Request) ([]byte, error)

func (f fakeProvider) Generate(ctx context.Context, r Request) ([]byte, error) { return f(ctx, r) }

func codeResponse(source string) []byte {
	b, _ := json.Marshal(map[string]any{"status": "success", "name": "Custom app", "source": source, "dependencies": []any{}, "reason": ""})
	return b
}

func TestCodeGenerationAndExplicitRepairContract(t *testing.T) {
	previous := &codeapp.Spec{Name: "Custom app", Source: "previous source"}
	calls := 0
	p := fakeProvider(func(ctx context.Context, r Request) ([]byte, error) {
		calls++
		if !strings.Contains(r.Instructions, "not restricted to predefined components") || !strings.Contains(r.Instructions, "saveData") || r.MaxOutputTokens != 16384 {
			t.Fatal("missing source generation contract")
		}
		var input struct {
			Request     string        `json:"request"`
			Previous    *codeapp.Spec `json:"previous"`
			Diagnostics string        `json:"diagnostics"`
		}
		if err := json.Unmarshal([]byte(r.Input), &input); err != nil {
			t.Fatal(err)
		}
		if input.Previous == nil || input.Previous.Source != previous.Source || input.Diagnostics != "compile failed" {
			t.Fatal("repair omitted source/diagnostics")
		}
		return codeResponse(codeReference), nil
	})
	spec, err := GenerateCode(context.Background(), p, "Build a custom timer", previous, "compile failed")
	if err != nil || spec.Source != codeReference || calls != 1 {
		t.Fatal(err, calls)
	}
}

func TestCodeGenerationErrorsAndCancellation(t *testing.T) {
	for name, output := range map[string][]byte{
		"invalid JSON":                 []byte("{"),
		"definition instead of source": []byte(`{"status":"success","definition":{}}`),
		"unsupported":                  []byte(`{"status":"unsupported","name":"","source":"","dependencies":[],"reason":"MCP execution is unavailable"}`),
		"bad dependency":               []byte(strings.Replace(string(codeResponse(codeReference)), `"dependencies":[]`, `"dependencies":[{"module":"evil/path","version":"latest"}]`, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			_, err := GenerateCode(context.Background(), fakeProvider(func(context.Context, Request) ([]byte, error) { calls++; return output, nil }), "custom app", nil, "")
			if err == nil || calls != 1 {
				t.Fatal("invalid output accepted or retried", err, calls)
			}
		})
	}
	failure := errors.New("API unavailable")
	_, err := GenerateCode(context.Background(), fakeProvider(func(context.Context, Request) ([]byte, error) { return nil, failure }), "custom app", nil, "")
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = GenerateCode(ctx, fakeProvider(func(context.Context, Request) ([]byte, error) { cancel(); return codeResponse(codeReference), nil }), "custom app", nil, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatal("late canceled source accepted", err)
	}
}
