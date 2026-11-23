package integrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/oauth2"
)

// Setup is untrusted, nonsecret guidance. It never grants access or supplies
// OAuth endpoints; those belong to host-written authentication code.
type Setup struct {
	DocumentationURL string   `json:"documentation_url"`
	CredentialURL    string   `json:"credential_url"`
	CredentialLabel  string   `json:"credential_label"`
	Permissions      []string `json:"permissions"`
	Instructions     string   `json:"instructions"`
}

func SetupURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" || strings.IndexFunc(raw, unicode.IsSpace) >= 0 || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return nil, fmt.Errorf("setup links must be complete HTTPS URLs without embedded credentials")
	}
	return u, nil
}

func (s *Setup) Validate() error {
	if s == nil {
		return nil
	} // Existing projects have no setup metadata.
	if len(s.CredentialLabel) > 80 || len(s.Instructions) > 2000 || len(s.Permissions) > 16 {
		return fmt.Errorf("integration setup guidance exceeds its limits")
	}
	for _, permission := range s.Permissions {
		if len(permission) > 200 {
			return fmt.Errorf("permission suggestion exceeds its limit")
		}
	}
	for _, raw := range []string{s.DocumentationURL, s.CredentialURL} {
		if raw != "" {
			if _, err := SetupURL(raw); err != nil {
				return err
			}
		}
	}
	return nil
}

// Connect authenticates only after explicit approval, then publishes settings.
// New credentials are cleaned up if settings cannot be published. Existing
// settings and entered form values remain available for retry.
func (s Store) Connect(ctx context.Context, c Connection, secret, clientID string, credentials Credentials, notify func(string, string)) error {
	if err := c.Validate(); err != nil {
		return err
	}
	doc, err := s.Load(ctx)
	if err != nil {
		return err
	}
	var old *Connection
	for i := range doc.Connections {
		if doc.Connections[i].ID == c.ID {
			copy := doc.Connections[i]
			old = &copy
			doc.Connections[i] = c
		}
	}
	if old == nil {
		doc.Connections = append(doc.Connections, c)
	}
	created := false
	switch c.Auth {
	case "none":
	case "oauth":
		err = SignIn(ctx, c, clientID, credentials, notify)
		created = err == nil
	default:
		if secret == "" {
			_, err = ReadyLogin(ctx, c, credentials)
			if errors.Is(err, ErrSignIn) {
				return fmt.Errorf("enter a credential before connecting")
			}
		} else {
			if c.Kind == "github" {
				_, err = Identity(ctx, secret)
			}
			if err == nil {
				err = credentials.Save(ctx, c, Login{Token: oauth2.Token{AccessToken: secret, TokenType: "Bearer"}})
				created = err == nil
			}
		}
	}
	if err != nil {
		return err
	}
	err = s.Save(ctx, doc)
	if err != nil {
		if created && (old == nil || old.Binding() != c.Binding()) {
			err = errors.Join(err, credentials.Delete(context.WithoutCancel(ctx), c))
		}
		return err
	}
	if old != nil && old.Auth != "none" && old.Binding() != c.Binding() {
		if err = credentials.Delete(context.WithoutCancel(ctx), *old); err != nil {
			return fmt.Errorf("connection saved; old credentials need cleanup: %w", err)
		}
	}
	return nil
}
