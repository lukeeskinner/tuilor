package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

func githubConfig(clientID string) oauth2.Config {
	return oauth2.Config{ClientID: clientID, Scopes: []string{"public_repo", "offline_access"}, Endpoint: oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token", DeviceAuthURL: "https://github.com/login/device/code", AuthStyle: oauth2.AuthStyleInParams}}
}
func authContext(ctx context.Context) context.Context {
	client := *http.DefaultClient
	if injected, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok {
		client = *injected
	}
	client.Timeout = 20 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return context.WithValue(ctx, oauth2.HTTPClient, &client)
}
func SignIn(ctx context.Context, s Connection, clientID string, credentials Credentials, notify func(string, string)) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Kind != "github" || s.Auth != "oauth" {
		return fmt.Errorf("browser sign-in applies only to GitHub")
	}
	if strings.TrimSpace(clientID) == "" {
		return fmt.Errorf("browser sign-in needs TUILOR_GITHUB_CLIENT_ID; use token entry instead")
	}
	ctx = authContext(ctx)
	config := githubConfig(clientID)
	device, err := config.DeviceAuth(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("request GitHub sign-in failed; check connectivity and client registration")
	}
	u, parseErr := url.Parse(device.VerificationURI)
	if parseErr != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.Path != "/login/device" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("GitHub returned an unexpected sign-in URL")
	}
	if notify != nil {
		notify(device.VerificationURI, device.UserCode)
	}
	token, err := config.DeviceAccessToken(ctx, device)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("GitHub sign-in denied, expired, or unavailable")
	}
	login := Login{*token, clientID}
	if _, err = Identity(ctx, login.Token.AccessToken); err != nil {
		return err
	}
	return credentials.Save(ctx, s, login)
}
func Identity(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Tuilor")
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if injected, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok {
		copy := *injected
		copy.CheckRedirect = client.CheckRedirect
		copy.Timeout = 10 * time.Second
		client = &copy
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("check GitHub identity: %w", ctx.Err())
		}
		return "", fmt.Errorf("check GitHub identity failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub rejected credentials (HTTP %d); check token permissions and organization authorization", resp.StatusCode)
	}
	var user struct {
		Login string `json:"login"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&user) != nil || user.Login == "" {
		return "", fmt.Errorf("GitHub returned an invalid identity")
	}
	return user.Login, nil
}
func ReadyLogin(ctx context.Context, s Connection, c Credentials) (Login, error) {
	if s.Auth == "none" {
		return Login{}, nil
	}
	l, err := c.Load(ctx, s)
	if err != nil {
		return l, err
	}
	if s.Auth != "oauth" {
		return l, nil
	}
	if !l.Token.Valid() {
		if l.Token.RefreshToken == "" || l.ClientID == "" {
			return l, ErrSignIn
		}
		ctx = authContext(ctx)
		config := githubConfig(l.ClientID)
		t, e := config.TokenSource(ctx, &l.Token).Token()
		if ctx.Err() != nil {
			return l, ctx.Err()
		}
		if e != nil {
			return l, fmt.Errorf("GitHub login expired; reconnect")
		}
		l.Token = *t
		if _, e = Identity(ctx, t.AccessToken); e != nil {
			return l, e
		}
		if e = c.Save(ctx, s, l); e != nil {
			return l, e
		}
	}
	return l, nil
}

type RuntimeConnection struct {
	ID      string `json:"id"`
	BaseURL string `json:"base_url"`
	Auth    string `json:"auth"`
	Key     string `json:"key"`
	Secret  string `json:"secret"`
	Kind    string `json:"kind"`
}

func Runtime(ctx context.Context, requirements []Requirement, bindings map[string]string) ([]RuntimeConnection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(requirements) == 0 {
		return nil, nil
	}
	store, err := DefaultStore()
	if err != nil {
		return nil, err
	}
	doc, err := store.Load(ctx)
	if err != nil {
		return nil, err
	}
	var out []RuntimeConnection
	for _, r := range requirements {
		var selected *Connection
		for _, s := range doc.Connections {
			if s.ID == bindings[r.ID] && s.Matches(r) {
				copy := s
				selected = &copy
				break
			}
		}
		if selected == nil {
			return nil, fmt.Errorf("connect %s before launching", r.Name)
		}
		login, err := ReadyLogin(ctx, *selected, Credentials{})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", selected.Name, err)
		}
		out = append(out, RuntimeConnection{r.ID, selected.BaseURL, selected.Auth, selected.Key, login.Token.AccessToken, selected.Kind})
	}
	return out, nil
}
