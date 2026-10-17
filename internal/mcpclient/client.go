// Package mcpclient exposes discovery, never tool execution.
package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lukeeskinner/tuilor/internal/connections"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool struct {
	Name         string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
}

// ListTools preserves the original CLI discovery entry point.
func ListTools(ctx context.Context, executable string) ([]Tool, error) {
	return Discover(ctx, connections.Server{ID: connections.ID(), Name: "CLI server", Kind: "stdio", Command: executable}, connections.Credentials{})
}

// Environment does not inherit secrets by default. Explicit AI-variable forwarding
// is rejected in configuration as well as here.
func Environment(names []string) []string {
	keys := []string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "WINDIR", "TMPDIR", "TMP", "TEMP", "LANG", "LC_ALL"}
	keys = append(keys, names...)
	seen := map[string]bool{}
	var env []string
	for _, key := range keys {
		upper := strings.ToUpper(key)
		if seen[upper] || connections.ForbiddenEnv(key) {
			continue
		}
		seen[upper] = true
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func Discover(ctx context.Context, s connections.Server, creds connections.Credentials) (tools []Tool, err error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	var transport mcp.Transport
	if s.Kind == "stdio" {
		path := s.Command
		if s.Directory != "" && !filepath.IsAbs(path) && strings.ContainsAny(path, `/\`) {
			path = filepath.Join(s.Directory, path)
		}
		if s.Directory != "" {
			var e error
			path, e = absoluteCommand(path)
			if e != nil {
				return nil, e
			}
		}
		cmd := exec.CommandContext(ctx, path, s.Args...)
		cmd.Dir, cmd.Env = s.Directory, Environment(s.Env)
		cmd.WaitDelay = time.Second
		// Server stderr is intentionally not attached to the live terminal or logs.
		transport = &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}
	} else {
		client, e := authorizedClient(ctx, s, creds)
		if e != nil {
			return nil, e
		}
		defer client.CloseIdleConnections()
		transport = &mcp.StreamableClientTransport{Endpoint: s.URL, HTTPClient: client, MaxRetries: -1, DisableStandaloneSSE: true}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "tuilor", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, discoveryError(ctx, s, err)
	}
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close MCP session failed"))
			tools = nil
		}
	}()
	tools, err = listTools(ctx, session)
	if err != nil {
		return nil, discoveryError(ctx, s, err)
	}
	return tools, nil
}

func absoluteCommand(path string) (string, error) {
	if !strings.ContainsAny(path, `/\`) {
		return exec.LookPath(path)
	}
	return filepath.Abs(path)
}

func discoveryError(ctx context.Context, s connections.Server, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("MCP discovery: %w", ctx.Err())
	}
	if errors.Is(err, connections.ErrNoLogin) {
		return connections.ErrNoLogin
	}
	// Remote error strings can contain URLs, response bodies, and tokens.
	if s.Kind == "http" {
		return fmt.Errorf("remote MCP discovery failed; check the URL, server availability, and protocol support")
	}
	var execErr *exec.Error
	var pathErr *os.PathError
	if errors.As(err, &execErr) || errors.As(err, &pathErr) {
		return fmt.Errorf("start MCP executable: %w", err)
	}
	return fmt.Errorf("local MCP discovery failed; check executable arguments and MCP protocol support")
}

func listTools(ctx context.Context, session *mcp.ClientSession) ([]Tool, error) {
	var tools []Tool
	total := 0
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		input, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, err
		}
		var output json.RawMessage
		if tool.OutputSchema != nil {
			output, err = json.Marshal(tool.OutputSchema)
			if err != nil {
				return nil, err
			}
		}
		total += len(tool.Name) + len(tool.Description) + len(input) + len(output)
		if len(tools) >= 1000 || total > 2*1024*1024 {
			return nil, fmt.Errorf("tool metadata exceeds discovery limit")
		}
		tools = append(tools, Tool{tool.Name, tool.Description, input, output})
	}
	return tools, nil
}

type bearerTransport struct {
	base  http.RoundTripper
	token string
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	if t.token != "" {
		r.Header.Set("Authorization", "Bearer "+t.token)
	}
	resp, err := t.base.RoundTrip(r)
	if err == nil && (resp.StatusCode == 401 || resp.StatusCode == 403) {
		resp.Body.Close()
		return nil, connections.ErrNoLogin
	}
	return resp, err
}
func (t bearerTransport) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, Transport: http.DefaultTransport.(*http.Transport).Clone(), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func authorizedClient(ctx context.Context, s connections.Server, creds connections.Credentials) (*http.Client, error) {
	client := httpClient()
	switch s.Auth {
	case "none":
		client.Transport = bearerTransport{client.Transport, ""}
	case "bearer":
		token := os.Getenv(s.TokenEnv)
		if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
			return nil, fmt.Errorf("%w: set the configured bearer-token environment variable", connections.ErrNoLogin)
		}
		client.Transport = bearerTransport{client.Transport, token}
	case "oauth":
		login, err := creds.Load(ctx, s)
		if err != nil {
			return nil, err
		}
		if err := refresh(ctx, s, creds, &login, client); err != nil {
			return nil, err
		}
		client.Transport = bearerTransport{client.Transport, login.Token.AccessToken}
	}
	return client, nil
}
