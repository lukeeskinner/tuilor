package mcpclient

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool struct {
	Name        string
	Description string
}

func ListTools(ctx context.Context, executable string) ([]Tool, error) {
	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "tuilor",
			Version: "0.1.0",
		},
		nil,
	)

	transport := &mcp.CommandTransport{
		Command: exec.CommandContext(ctx, executable),
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer session.Close()

	var tools []Tool

	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list tools: %w", err)
		}

		tools = append(tools, Tool{
			Name:        tool.Name,
			Description: tool.Description,
		})
	}

	return tools, nil
}
