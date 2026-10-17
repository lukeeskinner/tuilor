package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lukeeskinner/tuilor/internal/connections"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

var ErrSignIn = errors.New("sign-in required")

const service = "tuilor.api.credentials"

type Credentials struct{ Backend connections.SecretStore }
type Login struct {
	Token    oauth2.Token `json:"token"`
	ClientID string       `json:"client_id,omitempty"`
}
type secretIndex struct {
	Generation string
	Parts      int
}

func (c Credentials) get(key string) (string, error) {
	if c.Backend != nil {
		return c.Backend.Get(service, key)
	}
	return keyring.Get(service, key)
}
func (c Credentials) set(key, value string) error {
	if c.Backend != nil {
		return c.Backend.Set(service, key, value)
	}
	return keyring.Set(service, key, value)
}
func (c Credentials) del(key string) error {
	if c.Backend != nil {
		return c.Backend.Delete(service, key)
	}
	return keyring.Delete(service, key)
}
func part(key string, i secretIndex, n int) string {
	return fmt.Sprintf("%s.%s.%d", key, i.Generation, n)
}
func (c Credentials) index(ctx context.Context, s Connection) (secretIndex, error) {
	if err := ctx.Err(); err != nil {
		return secretIndex{}, err
	}
	raw, err := c.get(s.Binding())
	if errors.Is(err, keyring.ErrNotFound) {
		return secretIndex{}, ErrSignIn
	}
	if err != nil {
		return secretIndex{}, fmt.Errorf("OS credential store unavailable or locked")
	}
	var idx secretIndex
	if len(raw) > 256 || json.Unmarshal([]byte(raw), &idx) != nil || !identifier.MatchString(idx.Generation) || idx.Parts < 1 || idx.Parts > 128 {
		return idx, fmt.Errorf("saved API login is damaged; remove its credentials from the OS store")
	}
	return idx, nil
}
func (c Credentials) Load(ctx context.Context, s Connection) (Login, error) {
	i, err := c.index(ctx, s)
	if err != nil {
		return Login{}, err
	}
	raw := ""
	for n := 0; n < i.Parts; n++ {
		if err = ctx.Err(); err != nil {
			return Login{}, err
		}
		p, e := c.get(part(s.Binding(), i, n))
		if e != nil || len(p) > 1536 {
			return Login{}, fmt.Errorf("saved API login is incomplete")
		}
		raw += p
	}
	var l Login
	if json.Unmarshal([]byte(raw), &l) != nil || l.Token.AccessToken == "" {
		return l, fmt.Errorf("saved API login is invalid")
	}
	return l, nil
}
func (c Credentials) Save(ctx context.Context, s Connection, l Login) error {
	raw, err := json.Marshal(l)
	if err != nil || len(raw) > 128*1536 || l.Token.AccessToken == "" {
		return fmt.Errorf("invalid or oversized API credentials")
	}
	old, oldErr := c.index(ctx, s)
	if oldErr != nil && !errors.Is(oldErr, ErrSignIn) {
		return oldErr
	}
	i := secretIndex{ID(), (len(raw) + 1535) / 1536}
	written := 0
	cleanup := func() {
		for n := 0; n < written; n++ {
			_ = c.del(part(s.Binding(), i, n))
		}
	}
	for n := 0; n < i.Parts; n++ {
		if err = ctx.Err(); err != nil {
			cleanup()
			return err
		}
		if err = c.set(part(s.Binding(), i, n), string(raw[n*1536:min(len(raw), (n+1)*1536)])); err != nil {
			cleanup()
			return fmt.Errorf("save credentials: OS credential store unavailable, locked, or full")
		}
		written++
	}
	if err = ctx.Err(); err != nil {
		cleanup()
		return err
	}
	b, _ := json.Marshal(i)
	if err = c.set(s.Binding(), string(b)); err != nil {
		cleanup()
		return fmt.Errorf("publish API credentials failed")
	}
	if oldErr == nil {
		for n := 0; n < old.Parts; n++ {
			_ = c.del(part(s.Binding(), old, n))
		}
	}
	return nil
}
func (c Credentials) Delete(ctx context.Context, s Connection) error {
	i, err := c.index(ctx, s)
	if errors.Is(err, ErrSignIn) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = c.del(s.Binding()); err != nil {
		return fmt.Errorf("remove API login failed")
	}
	for n := 0; n < i.Parts; n++ {
		if err = c.del(part(s.Binding(), i, n)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("login disabled; some credential-store items need manual removal")
		}
	}
	return nil
}
