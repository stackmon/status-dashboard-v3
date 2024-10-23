package api

import v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"

const (
	v1Group = "v1"
	v2Group = "v2"
)

func (a *API) initRoutes() {
	v1Api := a.r.Group(v1Group)
	{
		v1Api.GET("component_status", v1.GetComponentsStatusHandler(a.db, a.log))
		v1Api.POST("component_status", v1.PostComponentStatusHandler(a.db, a.log))

		v1Api.GET("incidents", v1.GetIncidentsHandler(a.db, a.log))
	}
	//nolint:gocritic
	// setup v2 group routing
	//v2 := a.router.Group(v2Group)
	//{
	//	v2.GET("components", a.GetComponentsStatusHandler)
	//	v2.GET("components/:id", a.GetComponentHandler)
	//	v2.GET("component_status", a.GetComponentsStatusHandler)
	//	v2.POST("component_status", a.PostComponentStatusHandler)
	//
	//	v2.GET("incidents", a.GetIncidentsHandler)
	//	v2.POST("incidents", a.ValidateComponentsMW(), a.PostIncidentHandler)
	//	v2.GET("incidents/:id", a.GetIncidentHandler)
	//	v2.PATCH("incidents/:id", a.ValidateComponentsMW(), a.PatchIncidentHandler)
	//
	//	v2.GET("rss")
	//	v2.GET("history")
	//	v2.GET("availability")
	//	v2.GET("/separate/<incident_id>/<component_id>") - > investigate it!!!
	//
	//	v2.GET("/login/:name")
	//	v2.GET("/auth/:name")
	//	v2.GET("/logout")
	//}
}
