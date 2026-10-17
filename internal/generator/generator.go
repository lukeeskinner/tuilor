// Package generator turns requests into application source.
package generator

import "context"

// Provider is defined at its consumer so verification can substitute an AI
// without API charges. Its response is untrusted; source validation and compilation
// happen separately.
type Provider interface {
	Generate(context.Context, Request) ([]byte, error)
}

type Request struct {
	Instructions    string
	Input           string
	Schema          map[string]any
	MaxOutputTokens int64
}
