// Package generator turns requests into generated Go application source.
package generator

import "context"

// Provider is defined at its consumer so verification can substitute an AI
// without API charges. Its response is untrusted; compilation happens separately.
type Provider interface {
	Generate(context.Context, Request) ([]byte, error)
}

type Request struct {
	Instructions    string
	Input           string
	Schema          map[string]any
	MaxOutputTokens int64
}
