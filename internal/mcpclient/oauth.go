package mcpclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/lukeeskinner/tuilor/internal/connections"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

// OpenBrowser starts only the host-selected OS URL opener, without a shell.
// Callers always retain the URL so browser-launch failure is recoverable.
func OpenBrowser(ctx context.Context, raw string) error {
	if err := connections.ValidateURL(raw); err != nil {
		return err
	}
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "/usr/bin/open", []string{raw}
	case "windows":
		name, args = "rundll32.exe", []string{"url.dll,FileProtocolHandler", raw}
	default:
		name, args = "xdg-open", []string{raw}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("browser could not open; use the displayed sign-in URL")
	}
	return nil
}

// SignIn owns the browser callback listener and a short-lived MCP session.
// It initializes the server but never lists or calls tools.
func SignIn(ctx context.Context, s connections.Server, creds connections.Credentials, onURL func(string)) (err error) {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Kind != "http" || s.Auth != "oauth" {
		return fmt.Errorf("browser sign-in applies only to remote OAuth connections")
	}
	// Fail before asking for consent if the credential store is inaccessible.
	if _, err := creds.Load(ctx, s); err != nil && !errors.Is(err, connections.ErrNoLogin) {
		return err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.CallbackPort))
	if err != nil {
		return fmt.Errorf("open OAuth callback port: %w", err)
	}
	defer listener.Close()
	callbackPath := "/callback"
	redirect := "http://" + listener.Addr().String() + callbackPath
	result := make(chan *auth.AuthorizationResult, 1)
	denied := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		if r.Method != "GET" || len(r.URL.RawQuery) > 16384 {
			http.Error(w, "Invalid callback", 400)
			return
		}
		q := r.URL.Query()
		// Invalid state is checked below before handing a result to the SDK.
		expected, ok := r.Context().Value(callbackStateKey{}).(string)
		if !ok || expected == "" || q.Get("state") != expected {
			http.Error(w, "Invalid sign-in state", 400)
			return
		}
		if q.Get("error") != "" {
			select {
			case denied <- struct{}{}:
			default:
			}
			fmt.Fprint(w, "Sign-in was declined. Return to Tuilor.")
			return
		}
		if q.Get("code") == "" {
			http.Error(w, "Missing authorization code", 400)
			return
		}
		select {
		case result <- &auth.AuthorizationResult{Code: q.Get("code"), State: q.Get("state"), Iss: q.Get("iss")}:
		default:
		}
		fmt.Fprint(w, "Response received. Return to Tuilor to check sign-in status.")
	})
	// The expected state is set before exposing the URL, and callback handling reads
	// it under a mutex through an immutable handler swap.
	state := new(callbackState)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callbackStateKey{}, state.get())))
	})}
	served := make(chan struct{})
	go func() { defer close(served); _ = server.Serve(listener) }()
	defer func() { _ = server.Close(); <-served }()
	client := httpClient()
	defer client.CloseIdleConnections()
	client.Transport = authEndpointTransport{base: client.Transport, allowLoopbackHTTP: strings.HasPrefix(s.URL, "http://")}
	cfg := &auth.AuthorizationCodeHandlerConfig{
		RedirectURL: redirect, Client: client, RequestRefreshToken: true, AcceptUnadvertisedIss: true,
		AuthorizationCodeFetcher: func(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
			u, err := url.Parse(args.URL)
			if err != nil || connections.ValidateURL(args.URL) != nil {
				return nil, fmt.Errorf("invalid authorization endpoint")
			}
			state.set(u.Query().Get("state"))
			onURL(args.URL)
			select {
			case r := <-result:
				return r, nil
			case <-denied:
				return nil, fmt.Errorf("sign-in declined")
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		NewTokenSource: func(_ context.Context, config *oauth2.Config, token *oauth2.Token) (oauth2.TokenSource, error) {
			if config.ClientSecret != "" {
				return nil, fmt.Errorf("this server requires unsupported confidential-client authentication")
			}
			if connections.ValidateURL(config.Endpoint.TokenURL) != nil || connections.ValidateURL(config.Endpoint.AuthURL) != nil {
				return nil, fmt.Errorf("invalid OAuth endpoint")
			}
			login := connections.Login{Token: token, Config: *config, Resource: s.URL}
			if err := creds.Save(ctx, s, login); err != nil {
				return nil, err
			}
			// Bind refresh to this operation, not the SDK's background refresh context.
			return config.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, client), token), nil
		},
	}
	if s.ClientID != "" {
		cfg.PreregisteredClient = &oauthex.ClientCredentials{ClientID: s.ClientID}
	} else {
		cfg.DynamicClientRegistrationConfig = &auth.DynamicClientRegistrationConfig{Metadata: &oauthex.ClientRegistrationMetadata{
			ClientName: "Tuilor", RedirectURIs: []string{redirect}, GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: "none",
		}}
	}
	handler, err := auth.NewAuthorizationCodeHandler(cfg)
	if err != nil {
		return fmt.Errorf("OAuth configuration is unsupported; check client ID and registration requirements")
	}
	// Saving errors must remain useful without displaying token exchange responses.
	tracked := &loginHandler{OAuthHandler: handler}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "tuilor", Version: "0.1.0"}, nil).Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: s.URL, HTTPClient: client, OAuthHandler: tracked, MaxRetries: -1, DisableStandaloneSSE: true,
	}, nil)
	if session != nil {
		defer func() {
			if e := session.Close(); e != nil {
				err = errors.Join(err, fmt.Errorf("close sign-in session failed"))
			}
		}()
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if tracked.err != nil {
		// Never expose SDK errors: OAuth errors can embed authorization codes or tokens.
		return safeLoginError(tracked.err)
	}
	if err != nil {
		return fmt.Errorf("OAuth server connection failed; check URL and server availability")
	}
	if !tracked.called {
		return fmt.Errorf("server did not request OAuth; use Check Connection or choose no authentication")
	}
	return nil
}

func safeLoginError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	// Classify using fixed messages; never copy untrusted SDK diagnostics.
	text := strings.ToLower(err.Error())
	for _, item := range []struct{ match, message string }{
		{"declined", "Browser sign-in was declined."},
		{"credential", "OAuth completed, but the OS credential store could not save the login."},
		{"registration", "OAuth client registration failed; provide a registered public client ID or use a server offering dynamic registration."},
		{"confidential", "This server requires unsupported confidential-client authentication."},
		{"metadata", "OAuth discovery metadata is unavailable or unsupported."},
		{"issuer", "OAuth issuer validation failed; no login was accepted."},
		{"state", "OAuth state validation failed; no login was accepted."},
		{"token", "OAuth token exchange failed; check client and callback settings."},
	} {
		if strings.Contains(text, item.match) {
			return errors.New(item.message)
		}
	}
	return fmt.Errorf("browser sign-in failed; check server support and callback settings")
}

type callbackStateKey struct{}
type callbackState struct {
	mu    sync.RWMutex
	value string
}

func (s *callbackState) get() string      { s.mu.RLock(); defer s.mu.RUnlock(); return s.value }
func (s *callbackState) set(value string) { s.mu.Lock(); defer s.mu.Unlock(); s.value = value }

type loginHandler struct {
	auth.OAuthHandler
	called bool
	err    error
}

func (h *loginHandler) Authorize(ctx context.Context, r *http.Request, resp *http.Response) error {
	h.called = true
	h.err = h.OAuthHandler.Authorize(ctx, r, resp)
	return h.err
}

type authEndpointTransport struct {
	base              http.RoundTripper
	allowLoopbackHTTP bool
}

func (t authEndpointTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if connections.ValidateURL(r.URL.String()) != nil || (r.URL.Scheme == "http" && !t.allowLoopbackHTTP) {
		return nil, fmt.Errorf("OAuth endpoint requires HTTPS")
	}
	return t.base.RoundTrip(r)
}
func (t authEndpointTransport) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func refresh(ctx context.Context, s connections.Server, creds connections.Credentials, login *connections.Login, client *http.Client) error {
	if login.Token.Valid() {
		return nil
	}
	if login.Token.RefreshToken == "" {
		return connections.ErrNoLogin
	}
	source := login.Config.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, client), login.Token)
	token, err := source.Token()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		var retrieve *oauth2.RetrieveError
		if errors.As(err, &retrieve) && retrieve.Response != nil && retrieve.Response.StatusCode >= 500 {
			return fmt.Errorf("OAuth refresh server unavailable; try checking again later")
		}
		return connections.ErrNoLogin
	}
	login.Token = token
	return creds.Save(ctx, s, *login)
}
