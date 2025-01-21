package tests

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	authURL = "/auth"
)

func TestAuth(t *testing.T) {
	t.Log("start to test for /auth/login")

	r, _, oa2Prov := initTests(t)

	codeVerifier := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	callbackURL := fmt.Sprintf("%s/callback", oa2Prov.WebURL)

	state := prepareState(codeVerifier, callbackURL)

	// the hashed state is:
	// eyJjb2RlX2NoYWxsZW5nZSI6IjY0Y2MwYWIxYTg4ZWZlYWNkNjRmYTc5ZWNlMzRlZGUwNDRjZDZkMWMzMmMyYTFjMjc5MWU1YmEyMDYzYzFiZWEiLCJjYWxsYmFja191cmwiOiJodHRwOi8vbG9jYWxob3N0OjUxNzMvY2FsbGJhY2sifQ
	url := fmt.Sprintf("%s/login?state=%s", authURL, state)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, url, nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 303, w.Code)
	// TODO: check state param
}

func prepareState(codeVerifier, callbackURL string) string {
	codeChallenge := sha256sum(codeVerifier)
	state := fmt.Sprintf("{\"code_challenge\":\"%s\",\"callback_url\":\"%s\"}", codeChallenge, callbackURL)

	return base64.RawURLEncoding.EncodeToString([]byte(state))
}

func sha256sum(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
