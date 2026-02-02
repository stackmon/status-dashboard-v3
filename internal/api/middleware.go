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
	claimsContextKey = "claims"
	roleContextKey   = "role"
	userIDContextKey = "user_id"
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

func AuthenticationMW(prov *auth.Provider, logger *zap.Logger, secretKey string, userAuthGroup string) gin.HandlerFunc {
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
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set(claimsContextKey, claims)
		}

		if _, ok := token.Method.(*jwt.SigningMethodRSA); ok && !isAuthGroupInClaims(token, logger, userAuthGroup) {
			apiErrors.RaiseNotAuthorizedErr(c, apiErrors.ErrAuthNotAuthenticated)
			return
		}
		c.Next()
	}
}

func RBACMiddleware(rbacService *rbac.Service, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("attempting to resolve user role")

		claimsVal, exists := c.Get(claimsContextKey)
		if !exists {
			logger.Debug("no claims found in context, assigning NoRole")
			c.Set(roleContextKey, rbac.NoRole)
			c.Next()
			return
		}

		claims, ok := claimsVal.(jwt.MapClaims)
		if !ok {
			logger.Error("claims in context are not of type jwt.MapClaims")
			c.Set(roleContextKey, rbac.NoRole)
			c.Next()
			return
		}

		var groups []string
		if groupsClaim, ok := claims["groups"]; ok {
			if groupInterface, ok := groupsClaim.([]interface{}); ok {
				for _, g := range groupInterface {
					if gStr, ok := g.(string); ok {
						groups = append(groups, gStr)
					}
				}
			}
		}

		role := rbacService.Resolve(groups)
		c.Set(roleContextKey, role)

		if sub, ok := claims["sub"].(string); ok {
			c.Set(claimsContextKey, sub)
		}

		c.Next()
	}
}

func isAuthGroupInClaims(token *jwt.Token, logger *zap.Logger, userAuthGroup string) bool {
	// Check group authorization if authGroup is configured
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		logger.Error("failed to parse token claims")
		return false
	}

	// Check if the "groups" claim exists
	groupsClaim, exists := claims["groups"]
	if !exists {
		logger.Error("groups claim not found in token")
		return false
	}

	// Convert groups claim to string slice
	groups, ok := groupsClaim.([]interface{})
	if !ok {
		logger.Error("groups claim is not an array")
		return false
	}

	// Check if the required group is present
	hasGroup := false
	for _, group := range groups {
		if groupStr, okType := group.(string); okType && strings.TrimPrefix(groupStr, "/") == userAuthGroup {
			hasGroup = true
			break
		}
	}

	if !hasGroup {
		logger.Warn("user does not belong to required group",
			zap.String("required_group", userAuthGroup))
		return false
	}

	logger.Info("user authorized with group membership",
		zap.String("group", userAuthGroup))

	return true
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
