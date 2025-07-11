package api

import (
	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
	"github.com/stackmon/otc-status-dashboard/internal/api/rss"
	v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
	newRSS "github.com/stackmon/otc-status-dashboard/internal/rss"
)

const (
	authGroup = "auth"
	v1Group   = "v1"
	v2Group   = "v2"
)

func (a *API) InitRoutes() {
	authAPI := a.r.Group(authGroup)
	{
		authAPI.GET("login", auth.GetLoginPageHandler(a.oa2Prov, a.log))
		authAPI.GET("callback", auth.GetCallbackHandler(a.oa2Prov, a.log))
		authAPI.POST("token", auth.PostTokenHandler(a.oa2Prov, a.log))
		authAPI.PUT("logout", auth.PutLogoutHandler(a.oa2Prov, a.log))
		authAPI.POST("refresh", auth.PostRefreshHandler(a.oa2Prov, a.log))
	}

	v1API := a.r.Group(v1Group)
	{
		v1API.GET("component_status", v1.GetComponentsStatusHandler(a.db, a.log))
		v1API.POST("component_status",
			AuthenticationV1DeprecatedMW(a.oa2Prov, a.log, a.secretKeyV1),
			v1.PostComponentStatusHandler(a.db, a.log),
		)

		v1API.GET("incidents", v1.GetIncidentsHandler(a.db, a.log))
	}

	v2API := a.r.Group(v2Group)
	{
		v2API.GET("components", v2.GetComponentsHandler(a.db, a.log))
		v2API.POST("components", AuthenticationMW(a.oa2Prov, a.log), v2.PostComponentHandler(a.db, a.log))
		v2API.GET("components/:id", v2.GetComponentHandler(a.db, a.log))

		v2API.GET("incidents", v2.GetIncidentsHandler(a.db, a.log))
		v2API.POST("incidents",
			AuthenticationMW(a.oa2Prov, a.log),
			ValidateComponentsMW(a.db, a.log),
			v2.PostIncidentHandler(a.db, a.log),
		)
		v2API.GET("incidents/:id", v2.GetIncidentHandler(a.db, a.log))
		v2API.PATCH("incidents/:id", AuthenticationMW(a.oa2Prov, a.log), v2.PatchIncidentHandler(a.db, a.log))
		v2API.POST("incidents/:id/extract",
			AuthenticationMW(a.oa2Prov, a.log),
			ValidateComponentsMW(a.db, a.log),
			v2.PostIncidentExtractHandler(a.db, a.log))

		v2API.GET("availability", v2.GetComponentsAvailabilityHandler(a.db, a.log))

		// For testing purposes only
		v2API.GET("rss/", newRSS.HandleRSS(a.db, a.log))
	}

	rssFEED := a.r.Group("rss")
	{
		rssFEED.GET("/", rss.HandleRSS(a.db, a.log))
	}
}
