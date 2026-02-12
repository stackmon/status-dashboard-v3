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
	eventContextKey  = "event"
	roleContextKey   = "role"
	userIDContextKey = "user_id"
	authMethodKey    = "auth_method"
	authMethodHMAC   = "hmac"
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
			logger.Info("HMAC token detected, using secret key for validation")
			if secretKey == "" {
				return nil, fmt.Errorf("secret key is not configured for HMAC token validation")
			}
			return []byte(secretKey), nil

		case *jwt.SigningMethodRSA:
			logger.Info("RSA token detected, using Keycloak public key for validation")
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

// AuthenticationMW validates JWT tokens.
// Missing or invalid tokens result in 401.
func AuthenticationMW(prov *auth.Provider, logger *zap.Logger, secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if prov.Disabled {
			logger.Info("authentication is disabled")
			c.Next()
			return
		}

		logger.Info("start to process authentication request")

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := parseToken(rawToken, secretKey, prov, logger)

		if err != nil {
			logger.Error("token parsing error", zap.Error(err))
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		if !token.Valid {
			logger.Error("token validation error", zap.Error(err))
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthTokenInvalid)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		if errUserID := setUserIDFromClaims(claims, c, logger); errUserID != nil {
			logger.Error("failed to set userID from claims", zap.Error(err))
			return
		}

		if groupsErr := setGroupsFromClaims(claims, c, logger); groupsErr != nil {
			logger.Error("failed to set groups from claims", zap.Error(groupsErr))
			return
		}

		c.Next()
	}
}

func SetJWTClaims(
	prov *auth.Provider, logger *zap.Logger, secretKey string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Info("start to retrieve JWT claims from the token")

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Info("jwt header is empty, skipping JWT claims retrieval")
			c.Set(v2.UsernameContextKey, nil)
			c.Next()
			return
		}

		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := parseToken(rawToken, secretKey, prov, logger)

		if err != nil {
			logger.Error("token parsing error", zap.Error(err))
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		if !token.Valid {
			logger.Error("token validation error", zap.Error(err))
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthTokenInvalid)
			return
		}

		if errUserID := setUserIDFromClaims(claims, c, logger); errUserID != nil {
			logger.Error("failed to set userID from claims", zap.Error(err))
			return
		}

		if groupsErr := setGroupsFromClaims(claims, c, logger); groupsErr != nil {
			logger.Error("failed to set groups from claims", zap.Error(groupsErr))
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
		return fmt.Errorf("preferred_username claim not found")
	}

	groups, ok := groupsCl.([]string)
	if !ok {
		return fmt.Errorf("group claim is not an array of strings")
	}

	c.Set(v2.UserIDGroupsContextKey, groups)
	logger.Info("extracted groups from JWT", zap.Strings("groups", groups))

	return nil
}

// RBACAuthorizationMW resolves user roles from JWT claims for write operations (POST/PATCH).
// Users without configured groups are rejected with 401.
func RBACAuthorizationMW(rbacService *rbac.Service, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("attempting to resolve user role")

		groupsVal, exists := c.Get(v2.UserIDGroupsContextKey)
		if !exists {
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		groups, ok := groupsVal.([]string)
		if !ok {
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		if !rbacService.HasAuthorizedGroup(groups) {
			logger.Warn("user does not belong to any configured RBAC group")
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		role := rbacService.Resolve(groups)
		c.Set(roleContextKey, role)
		logger.Debug("user role resolved", zap.Int("role", int(role)))

		c.Next()
	}
}

func CheckEventExistenceMW(dbInst *db.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("checking event existence")

		var incID v2.IncidentID
		if err := c.ShouldBindUri(&incID); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		event, err := dbInst.GetIncident(incID.ID)
		if err != nil {
			if errors.Is(err, db.ErrDBIncidentDSNotExist) {
				apiErrors.RaiseStatusNotFoundErr(c, apiErrors.ErrIncidentDSNotExist)
				return
			}
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
