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
	"go.uber.org/zap/zapcore"
)

const (
	authCallbackURL = "auth/callback"
)

type Provider struct {
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

func (p *Provider) refreshToken(refreshToken string) (*TokenRepr, error) {
	return p.kc.refreshToken(refreshToken)
}

// authAudit emits a structured audit log for OAuth flow events.
func authAudit(logger *zap.Logger, action, result, reason string) {
	lvl := zapcore.InfoLevel
	if result != "success" {
		lvl = zapcore.WarnLevel
	}
	if ce := logger.Check(lvl, "auth_audit"); ce != nil {
		fields := []zap.Field{
			zap.String("event", "auth_audit"),
			zap.String("idp_type", "keycloak"),
			zap.String("action", action),
			zap.String("result", result),
		}
		if reason != "" {
			fields = append(fields, zap.String("reason", reason))
		}
		ce.Write(fields...)
	}
}

func GetLoginPageHandler(prov *Provider, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := c.Query("state")
		if state == "" {
			authAudit(logger, "login", "failure", "missing_state_param")
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrAuthMissedStateParam)
			return
		}

		oauthURL := prov.AuthCodeURL(state)
		authAudit(logger, "login", "success", "")
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
		code := c.Query("code")
		state := c.Query("state")

		stateDecode, err := base64.RawStdEncoding.DecodeString(state)
		if err != nil {
			authAudit(logger, "callback", "failure", "invalid_base64_state")
			c.SetCookie("error", apiErrors.ErrAuthValidateBase64State.Error(), 1, "/", "", false, false)
			c.Redirect(http.StatusBadRequest, prov.WebURL)
			return
		}

		statePayload := &StatePayload{}
		err = json.Unmarshal(stateDecode, statePayload)
		if err != nil {
			authAudit(logger, "callback", "failure", "invalid_state_json")
			c.SetCookie("error", apiErrors.ErrAuthValidateBase64State.Error(), 1, "/", "", false, false)
			c.Redirect(http.StatusBadRequest, prov.WebURL)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*defaultTimeout)
		defer cancel()
		token, err := prov.Exchange(ctx, code)
		if err != nil {
			authAudit(logger, "callback", "failure", "token_exchange_failed")
			c.SetCookie("error", apiErrors.ErrAuthExchangeToken.Error(), 1, "/", "", false, false)
			c.Redirect(http.StatusBadRequest, statePayload.CallbackURL)
			return
		}

		prov.PutToken(statePayload.CodeChallenge, TokenRepr{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken})
		authAudit(logger, "callback", "success", "")
		c.Redirect(http.StatusSeeOther, statePayload.CallbackURL)
	}
}

type CodeVerifierReq struct {
	CodeVerifier string `json:"code_verifier"`
}

func PostTokenHandler(prov *Provider, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		codeVerifier := CodeVerifierReq{}
		err := c.ShouldBindBodyWithJSON(&codeVerifier)
		if err != nil {
			authAudit(logger, "token_retrieve", "failure", "invalid_code_verifier")
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrAuthWrongCodeVerifier)
			return
		}

		h := sha256.New()
		h.Write([]byte(codeVerifier.CodeVerifier))
		codeChallenge := hex.EncodeToString(h.Sum(nil))

		token, ok := prov.GetToken(codeChallenge)
		if !ok {
			authAudit(logger, "token_retrieve", "failure", "no_data_for_code_verifier")
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrAuthMissingDataForCodeVerifier)
			return
		}
		authAudit(logger, "token_retrieve", "success", "")
		c.JSON(http.StatusOK, token)
	}
}

type PutLogoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

func PutLogoutHandler(prov *Provider, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req PutLogoutReq
		err := c.ShouldBindBodyWithJSON(&req)
		if err != nil {
			authAudit(logger, "logout", "failure", "missing_refresh_token")
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrAuthMissingRefreshToken)
			return
		}

		err = prov.revokeToken(req.RefreshToken)
		if err != nil {
			var keycloakErrorResponse KeycloakExternalError
			switch {
			case errors.As(err, &keycloakErrorResponse):
				authAudit(logger, "logout", "failure", keycloakErrorResponse.Error())
				apiErrors.RaiseBadRequestErr(c, keycloakErrorResponse)
			default:
				authAudit(logger, "logout", "failure", "revoke_token_failed")
				apiErrors.RaiseInternalErr(c, apiErrors.ErrAuthFailedLogout)
			}

			return
		}

		authAudit(logger, "logout", "success", "")
		c.Status(http.StatusNoContent)
	}
}

type RefreshTokenReq struct {
	RefreshToken string `json:"refresh_token"`
}

func PostRefreshHandler(prov *Provider, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RefreshTokenReq
		err := c.ShouldBindJSON(&req)
		if err != nil {
			authAudit(logger, "refresh", "failure", "missing_refresh_token")
			apiErrors.RaiseBadRequestErr(c, apiErrors.ErrAuthMissingRefreshToken)
			return
		}

		token, err := prov.refreshToken(req.RefreshToken)
		if err != nil {
			var keycloakErrorResponse KeycloakExternalError
			switch {
			case errors.As(err, &keycloakErrorResponse):
				authAudit(logger, "refresh", "failure", keycloakErrorResponse.Error())
				apiErrors.RaiseBadRequestErr(c, keycloakErrorResponse)
			default:
				authAudit(logger, "refresh", "failure", "refresh_token_failed")
				apiErrors.RaiseInternalErr(c, apiErrors.ErrAuthFailedRefreshToken)
			}

			return
		}
		authAudit(logger, "refresh", "success", "")
		c.JSON(http.StatusOK, token)
	}
}
