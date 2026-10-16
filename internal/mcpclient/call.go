package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ValidateArguments checks actual values, not merely the form's field bindings.
// Resolution has no external loader and never fetches remote schemas.
func (tool Tool) ValidateArguments(arguments map[string]any) error {
	var schema jsonschema.Schema
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		return fmt.Errorf("decode input schema for %q: %w", tool.Name, err)
	}
	if schema.Type != "object" {
		return fmt.Errorf("tool %q requires an object input schema", tool.Name)
	}
	for name := range arguments {
		property := schema.Properties[name]
		if property == nil || property.Type != "string" {
			return fmt.Errorf("tool %q has no supported string parameter %q", tool.Name, name)
		}
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return fmt.Errorf("resolve input schema for %q: %w", tool.Name, err)
	}
	if err := resolved.Validate(arguments); err != nil {
		return fmt.Errorf("invalid arguments for %q: %w", tool.Name, err)
	}
	return nil
}

type CallResult struct {
	Text    string
	IsError bool
}

// CallTool is called only after the UI authorizes this exact tool and argument
// snapshot. It discovers and validates on the same session used for the one call.
// Neither annotations nor discovered descriptions grant permission.
func CallTool(ctx context.Context, executable, name string, arguments map[string]any) (result CallResult, err error) {
	session, err := connect(ctx, executable)
	if err != nil {
		return result, err
	}
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close MCP session: %w", closeErr))
		}
	}()
	tools, err := listTools(ctx, session)
	if err != nil {
		return result, err
	}
	var selected *Tool
	for i := range tools {
		if tools[i].Name == name {
			if selected != nil {
				return result, fmt.Errorf("duplicate MCP tool %q", name)
			}
			selected = &tools[i]
		}
	}
	if selected == nil {
		return result, fmt.Errorf("MCP tool %q is no longer available", name)
	}
	if err := selected.ValidateArguments(arguments); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("before tool call: %w", err)
	}
	response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		if ctx.Err() != nil {
			err = errors.Join(err, ctx.Err())
		}
		return result, fmt.Errorf("call tool %q: %w; the tool may have run; no automatic retry was made", name, err)
	}
	if response == nil {
		return result, fmt.Errorf("tool %q returned no result; the tool may have run; no automatic retry was made", name)
	}
	result.IsError = response.IsError
	var parts []string
	hasText := false
	for _, content := range response.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
			hasText = hasText || text.Text != ""
		} else {
			parts = append(parts, "[Non-text content omitted]")
		}
	}
	if !hasText && response.StructuredContent != nil {
		data, encodeErr := json.MarshalIndent(response.StructuredContent, "", "  ")
		if encodeErr != nil {
			return result, fmt.Errorf("encode tool result: %w", encodeErr)
		}
		parts = append(parts, string(data))
	}
	if len(response.InputRequests) > 0 {
		result.IsError = true
		parts = append(parts, "Additional tool interaction is unsupported; nothing was retried.")
	}
	result.Text = strings.Join(parts, "\n")
	if result.Text == "" {
		result.Text = "Tool completed without text output."
	}
	if len(result.Text) > 64*1024 {
		result.Text = result.Text[:64*1024] + "\n[Output truncated]"
	}
	return result, nil
}
