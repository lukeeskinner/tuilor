package generator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

const DefaultModel = "gpt-4.1-mini"

type OpenAI struct {
	client openai.Client
	model  string
}

func NewOpenAI(apiKey, model string, opts ...option.RequestOption) (*OpenAI, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if strings.TrimSpace(model) == "" {
		model = DefaultModel
	}
	options := []option.RequestOption{option.WithAPIKey(apiKey), option.WithMaxRetries(0)}
	options = append(options, opts...)
	return &OpenAI{client: openai.NewClient(options...), model: model}, nil
}

func NewOpenAIFromEnv() (*OpenAI, error) {
	return NewOpenAI(os.Getenv("OPENAI_API_KEY"), os.Getenv("TUILOR_MODEL"))
}

// apiError preserves the SDK cause for errors. As without printing response bodies
// containing untrusted text or echoing generation inputs into terminal diagnostics.
type apiError struct {
	message string
	cause   error
}

func (e *apiError) Error() string { return e.message }
func (e *apiError) Unwrap() error { return e.cause }

func (p *OpenAI) Generate(ctx context.Context, request Request) ([]byte, error) {
	response, err := p.client.Responses.New(ctx, responses.ResponseNewParams{
		Model:           shared.ResponsesModel(p.model),
		Instructions:    openai.String(request.Instructions),
		Input:           responses.ResponseNewParamsInputUnion{OfString: openai.String(request.Input)},
		Store:           openai.Bool(false),
		MaxOutputTokens: openai.Int(8192),
		Text: responses.ResponseTextConfigParam{Format: responses.ResponseFormatTextConfigUnionParam{
			OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
				Name: "tuilor_app", Schema: request.Schema, Strict: openai.Bool(true),
			},
		}},
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("OpenAI request: %w", ctx.Err())
		}
		message := "OpenAI request failed"
		var api *openai.Error
		if errors.As(err, &api) {
			message = fmt.Sprintf("OpenAI request failed (HTTP %d); check API key, model access, and quota", api.StatusCode)
		}
		return nil, &apiError{message: message, cause: err}
	}
	if response.Status != responses.ResponseStatusCompleted {
		return nil, fmt.Errorf("OpenAI response was not completed (status %q); try a smaller app request", response.Status)
	}
	for _, item := range response.Output {
		if item.Type != "message" && item.Type != "reasoning" {
			return nil, fmt.Errorf("OpenAI returned unexpected output type %q", item.Type)
		}
		for _, part := range item.Content {
			if part.Type == "refusal" {
				return nil, fmt.Errorf("OpenAI refused the generation request")
			}
			if part.Type != "output_text" {
				return nil, fmt.Errorf("OpenAI returned unexpected content type %q", part.Type)
			}
		}
	}
	output := response.OutputText()
	if strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("OpenAI returned no app definition")
	}
	return []byte(output), nil
}
