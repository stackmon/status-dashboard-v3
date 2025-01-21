package auth

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
)

const (
	authCallbackURL = "auth/callback"
)

type Provider struct {
	Disabled       bool
	WebURL         string
	kc             *Keycloak
	conf           *oauth2.Config
	storage        *internalStorage
	realmPublicKey *rsa.PublicKey
}

func NewProvider(
	keycloakBaseURL,
	keycloakRealm,
	keycloakClientID,
	keycloakClientSecret,
	hostname,
	webURL string,
) (*Provider, error) {
	kc := NewKeycloak(keycloakBaseURL, keycloakRealm, keycloakClientID, keycloakClientSecret)

	redirectURI := fmt.Sprintf("%s/%s", hostname, authCallbackURL)

	conf := &oauth2.Config{
		ClientID:     keycloakClientID,
		ClientSecret: keycloakClientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     kc.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return &Provider{
		WebURL:  webURL,
		kc:      kc,
		conf:    conf,
		storage: newInternalStorage(),
	}, nil
}

func (p *Provider) AuthCodeURL(state string) string {
	return p.conf.AuthCodeURL(state)
}

func (p *Provider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.conf.Exchange(ctx, code)
}

func (p *Provider) PutToken(key string, token TokenRepr) {
	p.storage.Store(key, token)
}

func (p *Provider) GetToken(key string) (TokenRepr, bool) {
	token, ok := p.storage.Get(key)
	if !ok {
		return TokenRepr{}, false
	}
	p.storage.Delete(key)
	return token, true
}

func (p *Provider) GetPublicKey() (*rsa.PublicKey, error) {
	if p.realmPublicKey != nil {
		return p.realmPublicKey, nil
	}

	pKey, err := p.kc.fetchPublicKey()
	if err != nil {
		return nil, err
	}
	p.realmPublicKey = pKey
	return pKey, nil
}

func (p *Provider) revokeToken(refreshToken string) error {
	return p.kc.revokeToken(refreshToken)
}

func GetLoginPageHandler(prov *Provider, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info("start to process login page request")
		state := c.Query("state")
		if state == "" {
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrAuthMissedStateParam)
			return
		}

		oauthURL := prov.AuthCodeURL(state)
		logger.Info("redirect to keycloak login page")
		c.Redirect(http.StatusSeeOther, oauthURL)
	}
}

type StatePayload struct {
	CallbackURL   string `json:"callback_url"`
	CodeChallenge string `json:"code_challenge"`
}

type TokenRepr struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// GetCallbackHandler is a handler for the callback from the Keycloak, it redirects to the FE url.
func GetCallbackHandler(prov *Provider, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info("start to process authentication callback from keycloak")
		code := c.Query("code")
		state := c.Query("state")

		stateDecode, err := base64.RawStdEncoding.DecodeString(state)
		if err != nil {
			logger.Error("failed to decode base64 for state", zap.Error(err), zap.String("state", state))
			c.SetCookie("error", apiErrors.ErrAuthValidateBase64State.Error(), 1, "/", "", false, false)
			c.Redirect(http.StatusBadRequest, prov.WebURL)
			return
		}

		statePayload := &StatePayload{}
		err = json.Unmarshal(stateDecode, statePayload)
		if err != nil {
			logger.Error(
				"failed to unmarshal state to a struct", zap.Error(err), zap.String("state_decode", string(stateDecode)),
			)
			c.SetCookie("error", apiErrors.ErrAuthValidateBase64State.Error(), 1, "/", "", false, false)
			c.Redirect(http.StatusBadRequest, prov.WebURL)
			return
		}

		logger.Info("try to exchange code for tokens")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*defaultTimeout)
		defer cancel()
		token, err := prov.Exchange(ctx, code)
		if err != nil {
			logger.Error("failed to exchange a code to a tokens", zap.Error(err), zap.String("code", code))
			c.SetCookie("error", apiErrors.ErrAuthExchangeToken.Error(), 1, "/", "", false, false)
			c.Redirect(http.StatusBadRequest, statePayload.CallbackURL)
			return
		}

		prov.PutToken(statePayload.CodeChallenge, TokenRepr{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken})
		logger.Info("redirect to the client callback url")
		c.Redirect(http.StatusSeeOther, statePayload.CallbackURL)
	}
}

type CodeVerifierReq struct {
	CodeVerifier string `json:"code_verifier"`
}

func PostTokenHandler(prov *Provider, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info("start to process token request")
		codeVerifier := CodeVerifierReq{}
		err := c.ShouldBindBodyWithJSON(&codeVerifier)
		if err != nil {
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrAuthWrongCodeVerifier)
			return
		}

		h := sha256.New()
		h.Write([]byte(codeVerifier.CodeVerifier))
		codeChallenge := hex.EncodeToString(h.Sum(nil))

		logger.Debug("try to get token from the storage")
		token, ok := prov.GetToken(codeChallenge)
		if !ok {
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrAuthMissingDataForCodeVerifier)
			return
		}
		logger.Info("return token to the client")
		c.JSON(http.StatusOK, token)
	}
}

type PutLogoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

func PutLogoutHandler(prov *Provider, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("start to process logout request")

		var req PutLogoutReq
		err := c.ShouldBindBodyWithJSON(&req)
		if err != nil {
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrAuthMissingRefreshToken)
			return
		}

		err = prov.revokeToken(req.RefreshToken)
		if err != nil {
			var keycloakErrorResponse KeycloakExternalError
			switch {
			case errors.As(err, &keycloakErrorResponse):
				apiErrors.RaiseBadRequestErr(c, keycloakErrorResponse)
			default:
				logger.Error("failed to revoke token", zap.Error(err))
				apiErrors.RaiseInternalErr(c, apiErrors.ErrAuthFailedLogout)
			}

			return
		}

		c.Status(http.StatusNoContent)
	}
}
