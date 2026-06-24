package api

import (
	"html/template"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *Handler, tmpl *template.Template) {
	v1 := r.Group("/api/v1")
	{
		v1.GET("/adapters", h.ListAdapters)
		v1.POST("/crawl", h.TriggerCrawl)
		v1.GET("/crawl/:task_id", h.GetTaskStatus)
		v1.GET("/data", h.QueryData)
		v1.GET("/stats", h.GetStats)
	}

	wh := NewWebHandler(h, tmpl)
	r.GET("/", wh.Overview)
	r.GET("/detail/:adapter", wh.Detail)
	r.GET("/export/:adapter", wh.ExportCSV)
}
