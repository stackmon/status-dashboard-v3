package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/api/rbac"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

const (
	eventContextKey = "event"
)

const (
	usernameClaim = "preferred_username"
	groupsClaim   = "groups"
)

func ValidateComponentsMW(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info("start to validate given components")
		type Components struct {
			Components []int `json:"components" binding:"required,min=1"`
		}

		var components Components

		if err := c.ShouldBindBodyWithJSON(&components); err != nil {
			apiErrors.RaiseBadRequestErr(c, fmt.Errorf("%w: %w", apiErrors.ErrComponentInvalidFormat, err))
			return
		}

		// TODO: move this list to the memory cache
		// We should check, that all components are presented in our db.
		dbComps, err := dbInst.GetComponentsAsMap()
		if err != nil {
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		for _, comp := range components.Components {
			if _, ok := dbComps[comp]; !ok {
				apiErrors.RaiseBadRequestErr(c, apiErrors.NewErrComponentDSNotExist(comp))
				return
			}
		}

		c.Next()
	}
}

func parseToken(tokenString string, secretKey string, prov *auth.Provider, logger *zap.Logger) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		switch token.Method.(type) {
		case *jwt.SigningMethodHMAC:
			logger.Debug("selecting HMAC key for token validation")
			if secretKey == "" {
				return nil, fmt.Errorf("secret key is not configured for HMAC token validation")
			}
			return []byte(secretKey), nil

		case *jwt.SigningMethodRSA:
			logger.Debug("selecting RSA key for token validation")
			if prov == nil {
				return nil, fmt.Errorf("RSA token received but Keycloak provider is not configured")
			}
			key, err := prov.GetPublicKey()
			if err != nil {
				return nil, fmt.Errorf("error while getting public key: %w", err)
			}
			return key, nil

		default:
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
	})
}

// idpTypeFromMethod returns a string identifying the IdP based on JWT signing method.
func idpTypeFromMethod(method jwt.SigningMethod) string {
	switch method.(type) {
	case *jwt.SigningMethodHMAC:
		return "local_hmac"
	case *jwt.SigningMethodRSA:
		return "keycloak"
	default:
		return "unknown"
	}
}

// authAudit emits a structured audit log event for authentication/authorization decisions.
// All fields follow a consistent schema for SIEM integration.
func authAudit(logger *zap.Logger, action, result, idpType, username, reason string) {
	fields := []zap.Field{
		zap.String("event", "auth_audit"),
		zap.String("action", action),
		zap.String("result", result),
	}
	if idpType != "" {
		fields = append(fields, zap.String("idp_type", idpType))
	}
	if username != "" {
		fields = append(fields, zap.String("username", username))
	}
	if reason != "" {
		fields = append(fields, zap.String("reason", reason))
	}

	if result == "success" {
		logger.Info("auth_audit", fields...)
	} else {
		logger.Warn("auth_audit", fields...)
	}
}

// validateAndSetClaims parses the raw Bearer token, validates it, and sets
// preferred_username and groups into the gin context. Returns an error on any failure.
func validateAndSetClaims(
	rawToken, secretKey string,
	prov *auth.Provider,
	c *gin.Context,
	logger *zap.Logger,
) error {
	token, err := parseToken(rawToken, secretKey, prov, logger)
	if err != nil {
		authAudit(logger, "token_validation", "failure", "", "", err.Error())
		return apiErrors.ErrAuthNotAuthenticated
	}

	if !token.Valid {
		authAudit(logger, "token_validation", "failure", idpTypeFromMethod(token.Method), "", "invalid_token")
		return apiErrors.ErrAuthTokenInvalid
	}

	idpType := idpTypeFromMethod(token.Method)

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		authAudit(logger, "token_validation", "failure", idpType, "", "claims_extraction_failed")
		return apiErrors.ErrAuthTokenInvalid
	}

	if errUserID := setUserIDFromClaims(claims, c, logger); errUserID != nil {
		authAudit(logger, "token_validation", "failure", idpType, "", "missing_username_claim")
		return apiErrors.ErrAuthTokenInvalid
	}

	username, _ := c.Get(v2.UsernameContextKey)
	usernameStr, _ := username.(string)

	if groupsErr := setGroupsFromClaims(claims, c, logger); groupsErr != nil {
		authAudit(logger, "token_validation", "failure", idpType, usernameStr, "missing_groups_claim")
		return apiErrors.ErrAuthTokenInvalid
	}

	authAudit(logger, "token_validation", "success", idpType, usernameStr, "")
	return nil
}

// AuthenticationMW validates JWT tokens.
// Missing or invalid tokens result in 401.
func AuthenticationMW(prov *auth.Provider, logger *zap.Logger, secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			authAudit(logger, "token_validation", "failure", "", "", "missing_authorization_header")
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		if err := validateAndSetClaims(rawToken, secretKey, prov, c, logger); err != nil {
			apiErrors.RaiseNotAuthorizedErr(c, err)
			return
		}

		c.Next()
	}
}

// SetJWTClaims performs soft authentication for public-read endpoints.
// If no Authorization header is present, the request proceeds anonymously.
// If a token is present but invalid/forged, access is denied (401).
func SetJWTClaims(prov *auth.Provider, logger *zap.Logger, secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		if err := validateAndSetClaims(rawToken, secretKey, prov, c, logger); err != nil {
			apiErrors.RaiseNotAuthorizedErr(c, err)
			return
		}

		c.Next()
	}
}

func setUserIDFromClaims(claims jwt.MapClaims, c *gin.Context, logger *zap.Logger) error {
	preferredUsername, exists := claims[usernameClaim]
	if !exists {
		logger.Error("preferred_username claim not found")
		return fmt.Errorf("preferred_username claim not found")
	}

	preferredUsernameStr, ok := preferredUsername.(string)
	if !ok {
		logger.Error("preferred_username is not a string")
		return fmt.Errorf("preferred_username claim is not a string")
	}

	c.Set(v2.UsernameContextKey, preferredUsernameStr)
	logger.Info("extracted preferred_username from JWT", zap.String(usernameClaim, preferredUsernameStr))

	return nil
}

// setGroupsFromClaims extracts the "groups" claim from JWT as a string slice.
func setGroupsFromClaims(claims jwt.MapClaims, c *gin.Context, logger *zap.Logger) error {
	groupsCl, exists := claims[groupsClaim]
	if !exists {
		logger.Error("group claim not found")
		return fmt.Errorf("groups claim not found")
	}

	rawGroups, ok := groupsCl.([]interface{})
	if !ok {
		return fmt.Errorf("group claim is not an array")
	}

	groups := make([]string, 0, len(rawGroups))
	for _, g := range rawGroups {
		s, isStr := g.(string)
		if !isStr {
			return fmt.Errorf("group claim contains non-string value")
		}
		groups = append(groups, s)
	}

	c.Set(v2.UserIDGroupsContextKey, groups)
	logger.Info("extracted groups from JWT", zap.Strings("groups", groups))

	return nil
}

// RBACAuthorizationMW resolves user roles from JWT claims for write operations (POST/PATCH).
// Users without configured groups are rejected with 403 Forbidden.
func RBACAuthorizationMW(rbacService *rbac.Service, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupsVal, exists := c.Get(v2.UserIDGroupsContextKey)
		if !exists {
			authAudit(logger, "authorization", "denied", "", "", "groups_not_in_context")
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		groups, ok := groupsVal.([]string)
		if !ok {
			authAudit(logger, "authorization", "denied", "", "", "groups_invalid_type")
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		username, _ := c.Get(v2.UsernameContextKey)
		usernameStr, _ := username.(string)

		if !rbacService.HasAuthorizedGroup(groups) {
			authAudit(logger, "authorization", "denied", "", usernameStr, "no_matching_rbac_group")
			apiErrors.RaiseForbiddenErr(c, apiErrors.ErrAuthForbidden)
			return
		}

		role := rbacService.ResolveRole(groups)
		c.Set(v2.RoleContextKey, role)
		authAudit(logger, "authorization", "success", "", usernameStr, fmt.Sprintf("role=%d", int(role)))

		c.Next()
	}
}

func CheckEventExistenceMW(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("checking event existence")

		var incID v2.IncidentID
		if err := c.ShouldBindUri(&incID); err != nil {
			logger.Debug("event existence check failed: invalid event ID in URI", zap.Error(err))
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		event, err := dbInst.GetIncident(incID.ID)
		if err != nil {
			if errors.Is(err, db.ErrDBIncidentDSNotExist) {
				apiErrors.RaiseStatusNotFoundErr(c, apiErrors.ErrIncidentDSNotExist)
				return
			}
			logger.Error("event existence check failed: database error", zap.Error(err))
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.Set(eventContextKey, event)
		c.Next()
	}
}

func ErrorHandle() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}

		status := c.Writer.Status()

		var err error
		err = c.Errors.Last()
		if status >= http.StatusInternalServerError {
			err = apiErrors.ErrInternalError
		}

		c.JSON(-1, apiErrors.ReturnError(err))
	}
}

func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now().UTC()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		end := time.Now().UTC()
		latency := end.Sub(start)

		fields := []zapcore.Field{
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
		}

		if query != "" {
			fields = append(fields, zap.String("query", query))
		}

		switch {
		case c.Writer.Status() >= http.StatusInternalServerError:
			msg := fmt.Sprintf("panic was recovered, %s", apiErrors.ErrInternalError)
			if c.Errors.Last() != nil {
				msg = c.Errors.Last().Error()
			}
			log.Error(msg, fields...)
		case c.Writer.Status() >= http.StatusBadRequest:
			for _, e := range c.Errors.Errors() {
				log.Info(e, fields...)
			}
		default:
			log.Info(path, fields...)
		}
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, "+
				"Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
