package api

import (
	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
	v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
)

const (
	v1Group = "v1"
	v2Group = "v2"
)

func (a *API) InitRoutes() {
	authAPI := a.r.Group("auth")
	{
		authAPI.GET("login", auth.GetLoginPageHandler(a.oa2Prov, a.log))
		authAPI.GET("callback", auth.GetCallbackHandler(a.oa2Prov, a.log))
		authAPI.POST("token", auth.PostTokenHandler(a.oa2Prov, a.log))
		authAPI.PUT("logout", auth.PutLogoutHandler(a.oa2Prov, a.log))
	}

	v1Api := a.r.Group(v1Group)
	{
		v1Api.GET("component_status", v1.GetComponentsStatusHandler(a.db, a.log))
		v1Api.POST("component_status", AuthenticationMW(a.oa2Prov, a.log), v1.PostComponentStatusHandler(a.db, a.log))

		v1Api.GET("incidents", v1.GetIncidentsHandler(a.db, a.log))
	}

	v2Api := a.r.Group(v2Group)
	{
		v2Api.GET("components", v2.GetComponentsHandler(a.db, a.log))
		v2Api.POST("components", AuthenticationMW(a.oa2Prov, a.log), v2.PostComponentHandler(a.db, a.log))
		v2Api.GET("components/:id", v2.GetComponentHandler(a.db, a.log))

		v2Api.GET("incidents", v2.GetIncidentsHandler(a.db, a.log))
		v2Api.POST("incidents",
			AuthenticationMW(a.oa2Prov, a.log),
			ValidateComponentsMW(a.db, a.log),
			v2.PostIncidentHandler(a.db, a.log),
		)
		v2Api.GET("incidents/:id", v2.GetIncidentHandler(a.db, a.log))
		v2Api.PATCH("incidents/:id", AuthenticationMW(a.oa2Prov, a.log), v2.PatchIncidentHandler(a.db, a.log))

		v2Api.GET("availability", v2.GetComponentsAvailabilityHandler(a.db, a.log))

		//nolint:gocritic
		//v2Api.GET("rss")
		//v2Api.GET("history")
		//v2Api.GET("/separate/<incident_id>/<component_id>") - > investigate it!!!
	}
}
