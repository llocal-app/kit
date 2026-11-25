// Package identity is the X-LLocal-Identity token the gateway adds to every request it
// forwards to an app: a JWT signed with Ed25519 (alg EdDSA), checked against the public
// keys the runner hands the app in LLOCAL_JWKS.
package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const Header = "X-LLocal-Identity"

type Claims struct {
	Subject  string   `json:"sub"`
	Audience string   `json:"aud"`
	IssuedAt int64    `json:"iat"`
	Expires  int64    `json:"exp"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Groups   []string `json:"groups"`
	Company  string   `json:"company"`
	Guest    bool     `json:"guest"`
}

// Keys are Ed25519 public keys by key id.
type Keys map[string]ed25519.PublicKey

type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	X   string `json:"x"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

type header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

var b64 = base64.RawURLEncoding

func ParseJWKS(data []byte) (Keys, error) {
	var set jwks
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	keys := Keys{}
	for _, k := range set.Keys {
		x, err := b64.DecodeString(k.X)
		if k.Kty != "OKP" || k.Crv != "Ed25519" || k.Kid == "" || err != nil || len(x) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("jwks: key %q is not an Ed25519 public key with a kid", k.Kid)
		}
		keys[k.Kid] = ed25519.PublicKey(x)
	}
	if len(keys) == 0 {
		return nil, errors.New("jwks: no keys")
	}
	return keys, nil
}

func (k Keys) JWKS() []byte {
	var set jwks
	for _, kid := range slices.Sorted(maps.Keys(k)) {
		set.Keys = append(set.Keys, jwk{Kty: "OKP", Crv: "Ed25519", Kid: kid, X: b64.EncodeToString(k[kid]), Alg: "EdDSA", Use: "sig"})
	}
	data, _ := json.Marshal(set)
	return data
}

// Set puts c on a request the gateway forwards to an app: the signed token, and the plain
// headers a kept app reads, with the name and each group percent-encoded.
func Set(h http.Header, kid string, key ed25519.PrivateKey, c Claims) {
	groups := make([]string, len(c.Groups))
	for i, g := range c.Groups {
		groups[i] = url.PathEscape(g)
	}
	h.Set(Header, Sign(kid, key, c))
	h.Set("X-LLocal-User-Id", c.Subject)
	h.Set("X-LLocal-User-Email", c.Email)
	h.Set("X-LLocal-User-Name", url.PathEscape(c.Name))
	h.Set("X-LLocal-Groups", strings.Join(groups, ","))
}

func Sign(kid string, key ed25519.PrivateKey, c Claims) string {
	h, _ := json.Marshal(header{Alg: "EdDSA", Kid: kid, Typ: "JWT"})
	p, _ := json.Marshal(c)
	signed := b64.EncodeToString(h) + "." + b64.EncodeToString(p)
	return signed + "." + b64.EncodeToString(ed25519.Sign(key, []byte(signed)))
}

// Verify checks the token's signature, that it was made for audience (the app's host name)
// and that it has not expired.
func Verify(keys Keys, token, audience string, now time.Time) (Claims, error) {
	var c Claims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return c, errors.New("malformed token")
	}
	var h header
	if err := decode(parts[0], &h); err != nil {
		return c, err
	}
	if h.Alg != "EdDSA" {
		return c, fmt.Errorf("alg %q is not EdDSA", h.Alg)
	}
	key, ok := keys[h.Kid]
	if !ok {
		return c, fmt.Errorf("unknown key %q", h.Kid)
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return c, errors.New("bad signature")
	}
	if err := decode(parts[1], &c); err != nil {
		return c, err
	}
	switch {
	case c.Audience != audience:
		return c, fmt.Errorf("token is for %q, not %q", c.Audience, audience)
	case now.Unix() >= c.Expires:
		return c, errors.New("token expired")
	case c.Subject == "":
		return c, errors.New("token has no subject")
	}
	return c, nil
}

func decode(part string, v any) error {
	data, err := b64.DecodeString(part)
	if err != nil {
		return errors.New("malformed token")
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("malformed token: %w", err)
	}
	return nil
}
