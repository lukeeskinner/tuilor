package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GreetInput struct {
	Name string `json:"name"`
}

type GreetOutput struct {
	Message string `json:"message"`
}

func greet(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GreetInput,
) (*mcp.CallToolResult, GreetOutput, error) {
	return nil, GreetOutput{
		Message: "Hello, " + input.Name,
	}, nil
}

func main() {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "tuilor-test",
			Version: "0.1.0",
		},
		nil,
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "greet",
		Description: "Greets someone by name",
	}, greet)

	if err := server.Run(
		context.Background(),
		&mcp.StdioTransport{},
	); err != nil {
		log.Fatal(err)
	}
}
