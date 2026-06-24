package api

import (
	"context"
	"crawler/internal/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Engine interface {
	Collect(ctx context.Context, adapterName string) (int64, error)
}

type Repository interface {
	GetTask(ctx context.Context, id int64) (*model.Task, error)
	ListRecentTasks(ctx context.Context, limit int) ([]model.Task, error)
	QueryData(ctx context.Context, params model.QueryParams) (*model.QueryResult, error)
	CountByAdapter(ctx context.Context) (map[string]int64, error)
	ListEnabledConfigs(ctx context.Context) ([]model.CrawlConfig, error)
	UpsertConfig(ctx context.Context, cfg *model.CrawlConfig) error
}

type AdapterInfo struct {
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	Category  string     `json:"category"`
	Enabled   bool       `json:"enabled"`
	Cron      string     `json:"cron"`
	RateLimit int        `json:"rate_limit"`
	LastTask  *TaskBrief `json:"last_task,omitempty"`
}

type TaskBrief struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	Collected int    `json:"collected"`
	StartedAt string `json:"started_at"`
}

type AdapterMeta struct {
	Label    string
	Category string
}

type Handler struct {
	engine      Engine
	repo        Repository
	scheduler   Scheduler
	reg         Registry
	logger      *zap.Logger
	adapterMeta map[string]AdapterMeta
}

type Scheduler interface {
	Trigger(adapterName string) (int64, error)
	AddJob(adapterName, cronExpr string) error
	RemoveJob(adapterName string)
}

type Registry interface {
	List() []string
}

func NewHandler(
	engine Engine,
	repo Repository,
	scheduler Scheduler,
	reg Registry,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		engine:    engine,
		repo:      repo,
		scheduler: scheduler,
		reg:       reg,
		logger:    logger,
		adapterMeta: map[string]AdapterMeta{
			"stats_gov":      {Label: "国家统计局", Category: "government"},
			"hangye_paihang": {Label: "行业排行榜", Category: "ranking"},
			"iresearch":              {Label: "艾瑞数据", Category: "market_research"},
			"guduo":                  {Label: "骨朵数据", Category: "entertainment"},
			"maoyan":                 {Label: "猫眼票房", Category: "entertainment"},
			"penguin_intelligence":   {Label: "企鹅智库", Category: "research"},
			"tencent_research":       {Label: "腾讯研究院", Category: "research"},
		},
	}
}

func (h *Handler) ListAdapters(c *gin.Context) {
	configs, _ := h.repo.ListEnabledConfigs(c.Request.Context())
	cfgMap := make(map[string]model.CrawlConfig)
	for _, cfg := range configs {
		cfgMap[cfg.Adapter] = cfg
	}
	var adapters []AdapterInfo
	for _, name := range h.reg.List() {
		meta, ok := h.adapterMeta[name]
		if !ok {
			meta = AdapterMeta{Label: name, Category: "other"}
		}
		info := AdapterInfo{
			Name:     name,
			Label:    meta.Label,
			Category: meta.Category,
		}
		if cfg, ok := cfgMap[name]; ok {
			info.Enabled = cfg.Enabled
			info.Cron = cfg.CronExpr
			info.RateLimit = cfg.RateLimit
		}
		adapters = append(adapters, info)
	}
	c.JSON(http.StatusOK, gin.H{"adapters": adapters})
}

func (h *Handler) TriggerCrawl(c *gin.Context) {
	var req struct {
		Adapters []string `json:"adapters" binding:"required"`
		Mode     string   `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var tasks []gin.H
	for _, name := range req.Adapters {
		id, err := h.scheduler.Trigger(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		tasks = append(tasks, gin.H{"id": id, "adapter": name, "status": "pending"})
	}
	c.JSON(http.StatusAccepted, gin.H{"tasks": tasks})
}

func (h *Handler) GetTaskStatus(c *gin.Context) {
	idStr := c.Param("task_id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id"})
		return
	}
	task, err := h.repo.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handler) QueryData(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	params := model.QueryParams{
		Adapter:  c.Query("adapter"),
		From:     c.Query("from"),
		To:       c.Query("to"),
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	}
	result, err := h.repo.QueryData(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetStats(c *gin.Context) {
	counts, err := h.repo.CountByAdapter(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var total int64
	for _, c := range counts {
		total += c
	}
	recentTasks, _ := h.repo.ListRecentTasks(c.Request.Context(), 5)
	c.JSON(http.StatusOK, gin.H{
		"total_rows":    total,
		"by_adapter":    counts,
		"recent_tasks":  recentTasks,
		"system_health": "ok",
	})
}
