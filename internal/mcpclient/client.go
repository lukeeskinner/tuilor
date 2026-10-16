package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

func connect(ctx context.Context, executable string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "tuilor",
			Version: "0.1.0",
		},
		nil,
	)

	transport := &mcp.CommandTransport{
		Command:           exec.CommandContext(ctx, executable),
		TerminateDuration: time.Second,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("connect to MCP server: %w", ctx.Err())
		}
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	return session, nil
}

func ListTools(ctx context.Context, executable string) (tools []Tool, err error) {
	session, err := connect(ctx, executable)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close MCP session: %w", closeErr))
			tools = nil
		}
	}()
	return listTools(ctx, session)
}

func listTools(ctx context.Context, session *mcp.ClientSession) ([]Tool, error) {
	var tools []Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("list tools: %w", ctx.Err())
			}
			return nil, fmt.Errorf("list tools: %w", err)
		}

		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("marshal schema for %q: %w", tool.Name, err)
		}

		tools = append(tools, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		})
	}

	return tools, nil
}
