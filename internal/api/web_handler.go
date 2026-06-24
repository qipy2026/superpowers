package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"

	"crawler/internal/model"

	"github.com/gin-gonic/gin"
)

type WebHandler struct {
	api *Handler
}

func NewWebHandler(api *Handler) *WebHandler {
	return &WebHandler{api: api}
}

func (wh *WebHandler) Overview(c *gin.Context) {
	counts, _ := wh.api.Repo.CountByAdapter(c.Request.Context())
	var total int64
	online := 0
	for _, cnt := range counts {
		total += cnt
		if cnt > 0 {
			online++
		}
	}
	recentTasks, _ := wh.api.Repo.ListRecentTasks(c.Request.Context(), 18)
	taskMap := make(map[string]string)
	for _, t := range recentTasks {
		taskMap[t.Adapter] = t.Status
	}

	type Card struct {
		AdapterMeta
		Status string
		Count  int64
		CSS    string
	}
	var cards []Card
	var errors int64
	for _, name := range wh.api.Reg.List() {
		meta, ok := wh.api.AdapterMeta[name]
		if !ok {
			meta = AdapterMeta{Label: name}
		}
		meta.Name = name
		css := "ok"
		if s, ok := taskMap[name]; ok {
			switch s {
			case "failed":
				css = "err"
				errors++
			case "running":
				css = "warn"
			}
		}
		cards = append(cards, Card{AdapterMeta: meta, Status: taskMap[name], Count: counts[name], CSS: css})
	}

	allAdapters := make([]AdapterMeta, 0)
	for _, card := range cards {
		allAdapters = append(allAdapters, card.AdapterMeta)
	}

	c.HTML(http.StatusOK, "overview.html", gin.H{
		"Adapters": allAdapters,
		"Total":    total,
		"Online":   online,
		"Cards":    cards,
		"Errors":   errors,
	})
}

func (wh *WebHandler) Detail(c *gin.Context) {
	name := c.Param("adapter")
	label := name
	for _, m := range wh.api.AdapterMeta {
		if m.Name == name {
			label = m.Label
			break
		}
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

	result, _ := wh.api.Repo.QueryData(c.Request.Context(), model.QueryParams{
		Adapter:  name,
		Page:     page,
		PageSize: 20,
	})

	dataJSON := "["
	for i, r := range result.Rows {
		if i > 0 {
			dataJSON += ","
		}
		dataJSON += fmt.Sprintf(`{"label":"%s","value":%d}`, r.CollectedAt.Format("01-02"), i+1)
	}
	dataJSON += "]"

	allAdapters := make([]AdapterMeta, 0)
	for _, n := range wh.api.Reg.List() {
		m, ok := wh.api.AdapterMeta[n]
		if !ok {
			m = AdapterMeta{Label: n}
		}
		m.Name = n
		allAdapters = append(allAdapters, m)
	}

	c.HTML(http.StatusOK, "detail.html", gin.H{
		"Adapters":  allAdapters,
		"Name":      name,
		"Label":     label,
		"Total":     result.Total,
		"Page":      page,
		"Rows":      result.Rows,
		"HasMore":   result.Total > int64(page*20),
		"ChartData": dataJSON,
	})
}

func (wh *WebHandler) ExportCSV(c *gin.Context) {
	name := c.Param("adapter")
	result, _ := wh.api.Repo.QueryData(c.Request.Context(), model.QueryParams{
		Adapter: name, Page: 1, PageSize: 10000,
	})
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", name))
	w := csv.NewWriter(c.Writer)
	w.Write([]string{"data", "source_url", "collected_at"})
	for _, r := range result.Rows {
		w.Write([]string{r.DataJSON, r.SourceURL, r.CollectedAt.Format("2006-01-02 15:04:05")})
	}
	w.Flush()
}
