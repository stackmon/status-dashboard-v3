package api

import (
	v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
)

const (
	v1Group = "v1"
	v2Group = "v2"
)

func (a *API) InitRoutes() {
	v1Api := a.r.Group(v1Group)
	{
		v1Api.GET("component_status", v1.GetComponentsStatusHandler(a.db, a.log))
		v1Api.POST("component_status", v1.PostComponentStatusHandler(a.db, a.log))

		v1Api.GET("incidents", v1.GetIncidentsHandler(a.db, a.log))
	}

	// setup v2 group routing
	v2Api := a.r.Group(v2Group)
	{
		v2Api.GET("components", v2.GetComponentsHandler(a.db, a.log))
		v2Api.POST("components", v2.PostComponentHandler(a.db, a.log))
		v2Api.GET("components/:id", v2.GetComponentHandler(a.db, a.log))

		v2Api.GET("incidents", v2.GetIncidentsHandler(a.db, a.log))
		v2Api.POST("incidents", a.ValidateComponentsMW(), v2.PostIncidentHandler(a.db, a.log))
		v2Api.GET("incidents/:id", v2.GetIncidentHandler(a.db, a.log))
		v2Api.PATCH("incidents/:id", a.ValidateComponentsMW(), v2.PatchIncidentHandler(a.db, a.log))
		v2Api.GET("availability", v2.GetComponentsAvailabilityHandler(a.db, a.log))
		//nolint:gocritic
		//v2Api.GET("rss")
		//v2Api.GET("history")
		//v2Api.GET("/separate/<incident_id>/<component_id>") - > investigate it!!!
		//
		//v2Api.GET("/login/:name")
		//v2Api.GET("/auth/:name")
		//v2Api.GET("/logout")
	}
}
