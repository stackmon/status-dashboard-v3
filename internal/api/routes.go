package api

import (
	"fmt"

	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
	"github.com/stackmon/otc-status-dashboard/internal/api/rss"
	v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
	newRSS "github.com/stackmon/otc-status-dashboard/internal/rss"
)

const (
	authGroupPath = "auth"
	v1Group       = "v1"
	v2Group       = "v2"
)

// InitRoutes registers all HTTP routes on the underlying gin engine.
// openAPISpecPath is consumed once here to bind the /openapi.json handler;
// it is not retained on the API struct because it has no per-request use.
// Returns an error if the OpenAPI spec cannot be read or parsed — callers
// should propagate this so a misconfigured deployment fails at boot rather
// than serving 500s on first request.
//
//nolint:funlen
func (a *API) InitRoutes(openAPISpecPath string) error {
	if a.oa2Prov != nil {
		authAPI := a.r.Group(authGroupPath)
		{
			authAPI.GET("login", auth.GetLoginPageHandler(a.oa2Prov, a.log))
			authAPI.GET("callback", auth.GetCallbackHandler(a.oa2Prov, a.log))
			authAPI.POST("token", auth.PostTokenHandler(a.oa2Prov, a.log))
			authAPI.PUT("logout", auth.PutLogoutHandler(a.oa2Prov, a.log))
			authAPI.POST("refresh", auth.PostRefreshHandler(a.oa2Prov, a.log))
		}
	}

	v1API := a.r.Group(v1Group)
	{
		v1API.GET("component_status", v1.GetComponentsStatusHandler(a.db, a.log))
		v1API.POST("component_status",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			v1.PostComponentStatusHandler(a.db, a.log),
		)

		v1API.GET("incidents", v1.GetIncidentsHandler(a.db, a.log))
	}

	v2API := a.r.Group(v2Group)
	{
		v2API.GET("components", v2.GetComponentsHandler(a.db, a.log))
		v2API.POST("components",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			v2.PostComponentHandler(a.db, a.log))
		v2API.GET("components/:id", v2.GetComponentHandler(a.db, a.log))

		// Incidents section. Deprecated.
		// will be removed in a later version.
		v2API.GET("incidents",
			SetJWTClaims(a.oa2Prov, a.log, a.secretKeyV1),
			v2.GetIncidentsHandler(a.db, a.log, a.rbac))
		v2API.POST("incidents",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			RBACAuthorizationMW(a.rbac, a.log),
			ValidateComponentsMW(a.db, a.log),
			v2.PostIncidentHandler(a.db, a.log),
		)
		v2API.GET("incidents/:eventID",
			SetJWTClaims(a.oa2Prov, a.log, a.secretKeyV1),
			v2.GetIncidentHandler(a.db, a.log, a.rbac))
		v2API.PATCH("incidents/:eventID",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			RBACAuthorizationMW(a.rbac, a.log),
			CheckEventExistenceMW(a.db, a.log),
			v2.PatchIncidentHandler(a.db, a.log))
		v2API.POST("incidents/:eventID/extract",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			RBACAuthorizationMW(a.rbac, a.log),
			CheckEventExistenceMW(a.db, a.log),
			ValidateComponentsMW(a.db, a.log),
			v2.PostIncidentExtractHandler(a.db, a.log))
		v2API.PATCH("incidents/:eventID/updates/:updateID",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			RBACAuthorizationMW(a.rbac, a.log),
			CheckEventExistenceMW(a.db, a.log),
			v2.PatchEventUpdateTextHandler(a.db, a.log))

		// Events section.
		// Get /v2/events returns events page with pagination.
		v2API.GET("events",
			SetJWTClaims(a.oa2Prov, a.log, a.secretKeyV1),
			v2.GetEventsHandler(a.db, a.log, a.rbac))
		v2API.POST("events",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			RBACAuthorizationMW(a.rbac, a.log),
			ValidateComponentsMW(a.db, a.log),
			v2.PostIncidentHandler(a.db, a.log))
		v2API.GET("events/:eventID",
			SetJWTClaims(a.oa2Prov, a.log, a.secretKeyV1),
			v2.GetIncidentHandler(a.db, a.log, a.rbac))
		v2API.PATCH("events/:eventID",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			RBACAuthorizationMW(a.rbac, a.log),
			CheckEventExistenceMW(a.db, a.log),
			v2.PatchIncidentHandler(a.db, a.log))
		v2API.POST("events/:eventID/extract",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			RBACAuthorizationMW(a.rbac, a.log),
			CheckEventExistenceMW(a.db, a.log),
			ValidateComponentsMW(a.db, a.log),
			v2.PostIncidentExtractHandler(a.db, a.log))
		v2API.PATCH("events/:eventID/updates/:updateID",
			AuthenticationMW(a.oa2Prov, a.log, a.secretKeyV1),
			RBACAuthorizationMW(a.rbac, a.log),
			CheckEventExistenceMW(a.db, a.log),
			v2.PatchEventUpdateTextHandler(a.db, a.log))
		// Availability section.
		v2API.GET("availability", v2.GetComponentsAvailabilityHandler(a.db, a.log))

		// For testing purposes only.
		v2API.GET("rss/", newRSS.HandleRSS(a.db, a.log))
	}

	rssFEED := a.r.Group("rss")
	{
		rssFEED.GET("/", rss.HandleRSS(a.db, a.log))
	}

	openAPIHandler, err := filteredOpenAPIHandler(openAPISpecPath)
	if err != nil {
		return fmt.Errorf("init /openapi.json handler: %w", err)
	}
	a.r.GET("/openapi.json", openAPIHandler)
	a.r.GET("/swagger/*any", swaggerUIHandler("/openapi.json"))
	return nil
}
