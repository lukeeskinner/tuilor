package connections

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

var ErrNoLogin = errors.New("sign-in required")
var errDamagedLogin = errors.New("saved OAuth credential index is damaged")

// SecretStore is the small substitution boundary used by temporary fixtures.
// Production uses the OS credential store; there is no file fallback.
type SecretStore interface {
	Get(service, user string) (string, error)
	Set(service, user, value string) error
	Delete(service, user string) error
}
type osSecrets struct{}

func (osSecrets) Get(s, u string) (string, error) { return keyring.Get(s, u) }
func (osSecrets) Set(s, u, v string) error        { return keyring.Set(s, u, v) }
func (osSecrets) Delete(s, u string) error        { return keyring.Delete(s, u) }

type Credentials struct{ Backend SecretStore }
type Login struct {
	Token    *oauth2.Token `json:"token"`
	Config   oauth2.Config `json:"config"`
	Resource string        `json:"resource"`
}
type secretIndex struct {
	Generation string `json:"generation"`
	Parts      int    `json:"parts"`
}

const credentialService = "tuilor.mcp.oauth"

func (c Credentials) backend() SecretStore {
	if c.Backend != nil {
		return c.Backend
	}
	return osSecrets{}
}
func secretPart(binding string, idx secretIndex, n int) string {
	return fmt.Sprintf("%s.%s.%d", binding, idx.Generation, n)
}

func (c Credentials) index(ctx context.Context, s Server) (secretIndex, error) {
	if err := ctx.Err(); err != nil {
		return secretIndex{}, err
	}
	raw, err := c.backend().Get(credentialService, s.Binding())
	if errors.Is(err, keyring.ErrNotFound) {
		return secretIndex{}, ErrNoLogin
	}
	if err != nil {
		return secretIndex{}, fmt.Errorf("OS credential store unavailable or locked")
	}
	var idx secretIndex
	if len(raw) > 256 || json.Unmarshal([]byte(raw), &idx) != nil || !identifier.MatchString(idx.Generation) || idx.Parts < 1 || idx.Parts > 128 {
		return secretIndex{}, fmt.Errorf("%w; sign out and sign in again", errDamagedLogin)
	}
	return idx, ctx.Err()
}

func (c Credentials) Load(ctx context.Context, s Server) (Login, error) {
	idx, err := c.index(ctx, s)
	if err != nil {
		return Login{}, err
	}
	var raw string
	for n := 0; n < idx.Parts; n++ {
		if err := ctx.Err(); err != nil {
			return Login{}, err
		}
		part, err := c.backend().Get(credentialService, secretPart(s.Binding(), idx, n))
		if err != nil || len(part) > 1536 {
			return Login{}, fmt.Errorf("cannot read complete OAuth credentials; sign out and sign in again")
		}
		raw += part
	}
	var login Login
	if json.Unmarshal([]byte(raw), &login) != nil || login.Token == nil || login.Token.AccessToken == "" || login.Resource != s.URL || login.Config.ClientID == "" || login.Config.ClientSecret != "" {
		return Login{}, fmt.Errorf("saved OAuth credentials are invalid; sign out and sign in again")
	}
	if ValidateURL(login.Config.Endpoint.TokenURL) != nil {
		return Login{}, fmt.Errorf("invalid saved OAuth token endpoint")
	}
	return login, ctx.Err()
}

// Save publishes a pointer only after every immutable chunk has been written.
// Chunking respects the small per-item limit of the macOS keyring backend.
func (c Credentials) Save(ctx context.Context, s Server, login Login) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := json.Marshal(login)
	if err != nil {
		return fmt.Errorf("encode OAuth credentials: %w", err)
	}
	if len(raw) > 128*1536 {
		return fmt.Errorf("OAuth credentials exceed the credential-store size limit")
	}
	old, oldErr := c.index(ctx, s)
	if oldErr != nil && !errors.Is(oldErr, ErrNoLogin) {
		return oldErr
	}
	idx := secretIndex{ID(), (len(raw) + 1535) / 1536}
	written := 0
	cleanup := func() {
		for n := 0; n < written; n++ {
			_ = c.backend().Delete(credentialService, secretPart(s.Binding(), idx, n))
		}
	}
	for n := 0; n < idx.Parts; n++ {
		if err := ctx.Err(); err != nil {
			cleanup()
			return err
		}
		if err := c.backend().Set(credentialService, secretPart(s.Binding(), idx, n), string(raw[n*1536:min(len(raw), (n+1)*1536)])); err != nil {
			cleanup()
			return fmt.Errorf("save OAuth credentials: OS credential store unavailable, locked, or full")
		}
		written++
	}
	if err := ctx.Err(); err != nil {
		cleanup()
		return err
	}
	pointer, _ := json.Marshal(idx)
	if err := c.backend().Set(credentialService, s.Binding(), string(pointer)); err != nil {
		cleanup()
		return fmt.Errorf("publish OAuth credentials in OS credential store failed")
	}
	if oldErr == nil {
		for n := 0; n < old.Parts; n++ {
			_ = c.backend().Delete(credentialService, secretPart(s.Binding(), old, n))
		}
	}
	return nil
}

func (c Credentials) Delete(ctx context.Context, s Server) error {
	idx, err := c.index(ctx, s)
	if errors.Is(err, ErrNoLogin) {
		return nil
	}
	if errors.Is(err, errDamagedLogin) {
		if deleteErr := c.backend().Delete(credentialService, s.Binding()); deleteErr != nil {
			return fmt.Errorf("remove damaged OAuth login from OS credential store failed")
		}
		return fmt.Errorf("damaged login disabled; orphaned tuilor.mcp.oauth items may need manual removal from your OS credential store")
	}
	if err != nil {
		return err
	}
	if err := c.backend().Delete(credentialService, s.Binding()); err != nil {
		return fmt.Errorf("remove OAuth login from OS credential store failed")
	}
	var failed bool
	for n := 0; n < idx.Parts; n++ {
		if err := c.backend().Delete(credentialService, secretPart(s.Binding(), idx, n)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			failed = true
		}
	}
	if failed {
		return fmt.Errorf("login disabled, but some credential-store items could not be removed")
	}
	return nil
}
