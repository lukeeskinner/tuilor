// Package integrations owns native API settings and authentication, not app code.
package integrations

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lukeeskinner/tuilor/internal/connections"
)

type Connection struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	BaseURL string `json:"base_url"`
	Auth    string `json:"auth"`
	Key     string `json:"key,omitempty"`
}
type Requirement struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	BaseURL string `json:"base_url"`
	Auth    string `json:"auth"`
	Key     string `json:"key"`
	Setup   *Setup `json:"setup,omitempty"`
}
type Document struct {
	Version     int          `json:"version"`
	NerdIcons   bool         `json:"nerd_icons"`
	Connections []Connection `json:"connections"`
}
type Store struct{ Path string }

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$`)
var headerName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func ID() string { return "api-" + strings.ToLower(rand.Text()) }
func DefaultStore() (Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	return Store{filepath.Join(dir, "tuilor", "integrations.json")}, err
}
func (c Connection) Validate() error {
	if !identifier.MatchString(c.ID) || strings.TrimSpace(c.Name) == "" || len(c.Name) > 200 || strings.ContainsAny(c.Name, "\r\n\x1b") {
		return fmt.Errorf("provide a valid connection ID and name")
	}
	if len(c.BaseURL) > 8192 || len(c.Key) > 128 {
		return fmt.Errorf("connection URL or credential key exceeds its limit")
	}
	if err := connections.ValidateURL(c.BaseURL); err != nil {
		return err
	}
	u, _ := url.Parse(c.BaseURL)
	if u.RawQuery != "" || u.Opaque != "" {
		return fmt.Errorf("base URL cannot contain a query")
	}
	if c.Kind != "github" && c.Kind != "custom" {
		return fmt.Errorf("unsupported integration")
	}
	if c.Kind == "github" && (c.BaseURL != "https://api.github.com" || (c.Auth != "oauth" && c.Auth != "bearer")) {
		return fmt.Errorf("GitHub requires its official API and browser or token authentication")
	}
	switch c.Auth {
	case "none", "bearer":
		if c.Key != "" {
			return fmt.Errorf("credential key applies only to header or query authentication")
		}
	case "oauth":
		if c.Kind != "github" || c.Key != "" {
			return fmt.Errorf("browser sign-in is supported for GitHub only")
		}
	case "header", "query":
		if !headerName.MatchString(c.Key) || reservedHeader(c.Key) {
			return fmt.Errorf("provide a nonreserved credential key")
		}
	default:
		return fmt.Errorf("unsupported authentication method")
	}
	return nil
}
func reservedHeader(key string) bool {
	switch strings.ToLower(key) {
	case "host", "content-length", "connection", "transfer-encoding", "proxy-authorization", "cookie":
		return true
	}
	return false
}
func (r Requirement) Validate() error {
	if err := (Connection{r.ID, r.Name, r.Kind, r.BaseURL, r.Auth, r.Key}).Validate(); err != nil {
		return err
	}
	return r.Setup.Validate()
}
func (c Connection) Matches(r Requirement) bool {
	return c.Kind == r.Kind && strings.TrimRight(c.BaseURL, "/") == strings.TrimRight(r.BaseURL, "/") && (c.Kind == "github" || (c.Auth == r.Auth && c.Key == r.Key))
}
func (c Connection) Binding() string {
	b, _ := json.Marshal([]string{c.ID, c.Kind, c.BaseURL, c.Auth, c.Key})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s Store) Load(ctx context.Context) (Document, error) {
	d := Document{Version: 1}
	if err := ctx.Err(); err != nil {
		return d, err
	}
	info, err := os.Lstat(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, fmt.Errorf("read integrations: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return d, fmt.Errorf("integration configuration is not a regular file up to 1 MiB")
	}
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return d, err
	}
	d = Document{}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if !json.Valid(b) || decoder.Decode(&d) != nil || d.Version != 1 {
		return d, fmt.Errorf("integration configuration is corrupt; original file preserved")
	}
	seen := map[string]bool{}
	for _, c := range d.Connections {
		if c.Validate() != nil || seen[c.ID] {
			return d, fmt.Errorf("integration configuration contains invalid or duplicate connections; preserved")
		}
		seen[c.ID] = true
	}
	return d, ctx.Err()
}
func (s Store) Save(ctx context.Context, d Document) error {
	// Refuse to replace a damaged existing document.
	if _, err := s.Load(ctx); err != nil {
		return err
	}
	if d.Version != 1 || len(d.Connections) > 128 {
		return fmt.Errorf("invalid integration document")
	}
	seen := map[string]bool{}
	for _, c := range d.Connections {
		if err := c.Validate(); err != nil {
			return err
		}
		if seen[c.ID] {
			return fmt.Errorf("duplicate connection")
		}
		seen[c.ID] = true
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > 1024*1024 {
		return fmt.Errorf("integration configuration exceeds 1 MiB; previous settings preserved")
	}
	if err = os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.Path), ".integrations-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = os.Rename(f.Name(), s.Path)
	}
	if err != nil {
		return fmt.Errorf("save integrations: %w", err)
	}
	return nil
}
func Catalog(ctx context.Context) ([]Requirement, error) {
	s, err := DefaultStore()
	if err != nil {
		return nil, err
	}
	d, err := s.Load(ctx)
	if err != nil {
		return nil, err
	}
	out := []Requirement{}
	for _, c := range d.Connections {
		out = append(out, Requirement{ID: c.ID, Name: c.Name, Kind: c.Kind, BaseURL: c.BaseURL, Auth: c.Auth, Key: c.Key})
	}
	return out, nil
}
