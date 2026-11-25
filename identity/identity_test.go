package identity

import (
	"crypto/ed25519"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestVerify(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(nil)
	_, other, _ := ed25519.GenerateKey(nil)
	keys, err := ParseJWKS(Keys{"k1": pub}.JWKS())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	claims := Claims{Subject: "u_1", Audience: "inventory.acme.llocal.app", IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
		Email: "dana@acme.com", Name: "Dana", Groups: []string{"finance"}, Company: "c_1"}

	c, err := Verify(keys, Sign("k1", key, claims), "inventory.acme.llocal.app", now)
	if err != nil || c.Subject != "u_1" || c.Groups[0] != "finance" || c.Company != "c_1" {
		t.Fatalf("valid token: %+v, %v", c, err)
	}

	good := Sign("k1", key, claims)
	parts := strings.Split(good, ".")
	for name, tc := range map[string]struct {
		token, aud string
		at         time.Time
		want       string
	}{
		"other app":    {good, "payroll.acme.llocal.app", now, "is for"},
		"expired":      {good, "inventory.acme.llocal.app", now.Add(time.Minute), "expired"},
		"unknown key":  {Sign("k2", key, claims), "inventory.acme.llocal.app", now, "unknown key"},
		"wrong signer": {Sign("k1", other, claims), "inventory.acme.llocal.app", now, "bad signature"},
		"alg none":     {b64.EncodeToString([]byte(`{"alg":"none","kid":"k1"}`)) + "." + parts[1] + ".", "inventory.acme.llocal.app", now, "not EdDSA"},
		"swapped body": {parts[0] + "." + strings.Split(Sign("k1", key, Claims{Subject: "u_2"}), ".")[1] + "." + parts[2], "inventory.acme.llocal.app", now, "bad signature"},
		"malformed":    {"abc", "inventory.acme.llocal.app", now, "malformed"},
	} {
		if _, err := Verify(keys, tc.token, tc.aud, tc.at); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}

func TestSet(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(nil)
	h := http.Header{}
	Set(h, "k", key, Claims{Subject: "u_1", Email: "dana@acme.com", Name: "Dana Ruiz", Groups: []string{"finance", "a,b"}})
	if h.Get(Header) == "" || h.Get("X-LLocal-User-Id") != "u_1" || h.Get("X-LLocal-User-Name") != "Dana%20Ruiz" ||
		h.Get("X-LLocal-Groups") != "finance,a%2Cb" {
		t.Errorf("headers = %v", h)
	}
}

func TestParseJWKSRejects(t *testing.T) {
	for _, data := range []string{`{"keys":[]}`, `{"keys":[{"kty":"RSA","kid":"a","n":"x"}]}`, `{"keys":[{"kty":"OKP","crv":"Ed25519","x":"AAAA"}]}`, `nope`} {
		if _, err := ParseJWKS([]byte(data)); err == nil {
			t.Errorf("accepted %s", data)
		}
	}
}
