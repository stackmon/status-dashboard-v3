package app

const (
	v1Group = "v1"
)

func (a *App) InitRoutes() {

	// setup v1 group routing
	v1 := a.router.Group(v1Group)
	{
		v1.GET("component_status")
		v1.POST("component_status")

		v1.GET("incidents", a.GetIncidents)
		v1.POST("incidents")
		v1.GET("incidents/:id", a.GetIncident)
		//v1.POST("incidents/<incident_id>")

		v1.GET("rss")
		v1.GET("history")
		v1.GET("availability")
		v1.GET("/separate/<incident_id>/<component_id>")

		v1.GET("/login/<name>")
		v1.GET("/auth/<name>")
		v1.GET("/logout")
	}
}
