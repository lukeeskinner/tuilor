// Package connections owns user-configured MCP connections, never app source.
package connections

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

type Server struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"` // stdio or http
	Command      string   `json:"command,omitempty"`
	Args         []string `json:"args,omitempty"`
	Directory    string   `json:"directory,omitempty"`
	Env          []string `json:"env,omitempty"` // Names only; values come from the environment.
	URL          string   `json:"url,omitempty"`
	Auth         string   `json:"auth,omitempty"` // none, bearer, or oauth
	TokenEnv     string   `json:"token_env,omitempty"`
	ClientID     string   `json:"client_id,omitempty"`
	CallbackPort int      `json:"callback_port,omitempty"`
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var identifier = regexp.MustCompile(`^[a-f0-9]{32}$`)

func ID() string {
	var b [16]byte
	// crypto/rand.Read cannot fail on supported Go platforms.
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func ForbiddenEnv(name string) bool {
	name = strings.ToUpper(name)
	return strings.HasPrefix(name, "OPENAI_") || strings.HasPrefix(name, "TUILOR_") || strings.HasPrefix(name, "AZURE_OPENAI_")
}

// ValidateURL permits unencrypted HTTP only on a literal loopback or localhost.
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("provide an HTTP(S) URL without embedded credentials or a fragment")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := strings.EqualFold(u.Hostname(), "localhost") || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return fmt.Errorf("HTTPS is required except for loopback development servers")
	}
	return nil
}

func (s Server) Validate() error {
	if !identifier.MatchString(s.ID) || strings.TrimSpace(s.Name) == "" || len(s.Name) > 200 {
		return fmt.Errorf("connection needs a valid ID and name (up to 200 bytes)")
	}
	for _, value := range []string{s.Name, s.Command, s.Directory, s.URL, s.ClientID} {
		if len(value) > 8192 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return fmt.Errorf("connection fields cannot contain control characters or exceed 8192 bytes")
		}
	}
	switch s.Kind {
	case "stdio":
		if strings.TrimSpace(s.Command) == "" {
			return fmt.Errorf("an executable is required")
		}
		if s.URL != "" || s.Auth != "" || s.TokenEnv != "" || s.ClientID != "" || s.CallbackPort != 0 {
			return fmt.Errorf("local connections cannot contain remote authentication settings")
		}
		if len(s.Args) > 128 || len(s.Env) > 128 {
			return fmt.Errorf("at most 128 arguments and environment names are supported")
		}
		for _, arg := range s.Args {
			if len(arg) > 8192 || strings.ContainsRune(arg, 0) {
				return fmt.Errorf("invalid command argument")
			}
		}
		for _, name := range s.Env {
			if !envName.MatchString(name) || ForbiddenEnv(name) {
				return fmt.Errorf("invalid or reserved environment variable name")
			}
		}
	case "http":
		if err := ValidateURL(s.URL); err != nil {
			return err
		}
		if s.Command != "" || len(s.Args) > 0 || s.Directory != "" || len(s.Env) > 0 {
			return fmt.Errorf("remote connections cannot contain local process settings")
		}
		switch s.Auth {
		case "none":
		case "bearer":
			if !envName.MatchString(s.TokenEnv) || ForbiddenEnv(s.TokenEnv) {
				return fmt.Errorf("a nonreserved bearer-token environment variable name is required")
			}
		case "oauth":
		default:
			return fmt.Errorf("choose none, bearer, or oauth authentication")
		}
		if s.Auth != "bearer" && s.TokenEnv != "" {
			return fmt.Errorf("token variable applies only to bearer authentication")
		}
		if s.Auth != "oauth" && (s.ClientID != "" || s.CallbackPort != 0) {
			return fmt.Errorf("client ID and callback port apply only to OAuth")
		}
		if s.CallbackPort < 0 || s.CallbackPort > 65535 {
			return fmt.Errorf("callback port must be 0 (automatic) or 1–65535")
		}
	default:
		return fmt.Errorf("choose a local command or remote HTTP connection")
	}
	return nil
}

// Binding changes when credentials must no longer be used for this connection.
func (s Server) Binding() string {
	b, _ := json.Marshal([]any{s.ID, s.Kind, s.URL, s.Auth, s.TokenEnv, s.ClientID, s.CallbackPort})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type Store struct{ Path string }
type document struct {
	Version int      `json:"version"`
	Servers []Server `json:"servers"`
}

func DefaultStore() (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("locate user configuration: %w", err)
	}
	return Store{Path: filepath.Join(dir, "tuilor", "mcp.json")}, nil
}

func (s Store) Load(ctx context.Context) ([]Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []Server{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read MCP connections: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return nil, fmt.Errorf("MCP configuration must be a regular file up to 1 MiB; file preserved")
	}
	dec := json.NewDecoder(io.LimitReader(f, 1024*1024+1))
	dec.DisallowUnknownFields()
	var doc document
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("invalid MCP configuration (preserved): %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("invalid trailing MCP configuration data; file preserved")
	}
	if doc.Version != 1 || len(doc.Servers) > 100 {
		return nil, fmt.Errorf("unsupported MCP configuration version or more than 100 connections; file preserved")
	}
	seen := map[string]bool{}
	for _, server := range doc.Servers {
		if err := server.Validate(); err != nil {
			return nil, fmt.Errorf("invalid MCP configuration (preserved): %w", err)
		}
		if seen[server.ID] {
			return nil, fmt.Errorf("duplicate connection ID; file preserved")
		}
		seen[server.ID] = true
	}
	return doc.Servers, ctx.Err()
}

// Put rereads the document so a damaged configuration is never replaced by a new list.
func (s Store) Put(ctx context.Context, server Server) error {
	if err := server.Validate(); err != nil {
		return err
	}
	servers, err := s.Load(ctx)
	if err != nil {
		return err
	}
	found := false
	for i := range servers {
		if servers[i].ID == server.ID {
			servers[i] = server
			found = true
		}
	}
	if !found {
		servers = append(servers, server)
	}
	return s.write(ctx, servers)
}

func (s Store) Remove(ctx context.Context, id string) error {
	servers, err := s.Load(ctx)
	if err != nil {
		return err
	}
	kept := make([]Server, 0, len(servers))
	for _, server := range servers {
		if server.ID != id {
			kept = append(kept, server)
		}
	}
	return s.write(ctx, kept)
}

func (s Store) write(ctx context.Context, servers []Server) error {
	if len(servers) > 100 {
		return fmt.Errorf("at most 100 connections are supported")
	}
	b, err := json.MarshalIndent(document{1, servers}, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > 1024*1024 {
		return fmt.Errorf("MCP configuration exceeds 1 MiB")
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create MCP configuration directory: %w", err)
	}
	f, err := os.CreateTemp(dir, ".mcp-*")
	if err != nil {
		return fmt.Errorf("stage MCP configuration: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(b, '\n')); err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return fmt.Errorf("write MCP configuration: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), s.Path); err != nil {
		return fmt.Errorf("publish MCP configuration: %w", err)
	}
	return nil
}
