# 网络数据爬虫系统 — Phase 1 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 搭建 Go 爬虫系统骨架，实现 Engine 核心 + 2 个低复杂度 Adapter + 5 个 API 端点

**架构：** 单体引擎 + Adapter 模式，Gin API → Engine（限流/重试）→ Adapter（Colly 采集）→ MySQL（GORM）

**技术栈：** Go 1.22+, gin-gonic/gin, gorm.io/gorm + MySQL, gocolly/colly, robfig/cron/v3, spf13/viper, uber-go/zap

---

## 文件蓝图

| 文件 | 职责 |
|------|------|
| `cmd/crawler/main.go` | 入口：初始化配置、DB、引擎、调度器、启动 HTTP |
| `config/config.yaml` | 服务器/数据库/引擎/适配器配置 |
| `internal/model/model.go` | 3 个 GORM 模型：Task, DataRow, CrawlConfig |
| `internal/repository/repository.go` | CRUD 操作：task CRUD, data 批量写入+查询, config CRUD |
| `internal/adapter/adapter.go` | Adapter 接口 + Task/Stats 运行时类型 |
| `internal/adapter/registry.go` | 注册表：按名称查找 adapter |
| `internal/adapter/stats_gov/adapter.go` | 国家统计局全量采集 |
| `internal/adapter/hangye_paihang/adapter.go` | 行业排行榜增量采集 |
| `internal/engine/limiter.go` | 双层 token bucket：per-adapter + global |
| `internal/engine/retry.go` | 指数退避重试，可/不可重试错误分类 |
| `internal/engine/proxy.go` | 代理池空实现（Phase 2 接入） |
| `internal/engine/engine.go` | 核心编排：限流 → 重试 → Collect → 入库 |
| `internal/scheduler/scheduler.go` | Cron 定时 + 手动触发，调用 Engine |
| `internal/api/handler.go` | 5 个 Gin handler |
| `internal/api/router.go` | 路由注册 |
| `go.mod` | 模块定义 |

---

### 任务 1：项目初始化

**文件：**
- 创建：`go.mod`
- 创建：`config/config.yaml`
- 创建：所有目录

- [ ] **步骤 1：初始化 Go module**

```bash
cd e:\work\code\superpowers
go mod init crawler
```

- [ ] **步骤 2：创建目录结构**

```bash
mkdir -p cmd/crawler internal/model internal/repository \
  internal/adapter/stats_gov internal/adapter/hangye_paihang \
  internal/engine internal/scheduler internal/api config
```

- [ ] **步骤 3：创建配置文件**

创建 `config/config.yaml`：

```yaml
server:
  port: 8080
  mode: debug

database:
  host: 127.0.0.1
  port: 3306
  user: crawler
  password: ${CRAWLER_DB_PASSWORD:-crawler123}
  name: crawler_db
  max_open_conns: 25

engine:
  global_rate_limit: 50
  retry:
    max_retries: 3
    base_delay: 1s
    max_delay: 30s

adapters:
  - name: stats_gov
    enabled: true
    cron: "0 8 * * 1"
    rate_limit: 5
    mode: full
  - name: hangye_paihang
    enabled: true
    cron: "0 9 * * *"
    rate_limit: 10
    mode: incremental
```

- [ ] **步骤 4：安装依赖**

```bash
go get github.com/gin-gonic/gin
go get gorm.io/gorm
go get gorm.io/driver/mysql
go get github.com/gocolly/colly/v2
go get github.com/robfig/cron/v3
go get github.com/spf13/viper
go get go.uber.org/zap
go mod tidy
```

- [ ] **步骤 5：验证构建**

```bash
go build ./...
```
预期：成功（可能有 "no Go files" 警告，后续任务会添加）

- [ ] **步骤 6：Commit**

```bash
git add -A && git commit -m "chore: initialize project skeleton with config and dependencies"
```

---

### 任务 2：数据模型

**文件：**
- 创建：`internal/model/model.go`
- 创建：`internal/model/model_test.go`

- [ ] **步骤 1：编写模型测试**

创建 `internal/model/model_test.go`：

```go
package model

import (
    "testing"
    "time"
)

func TestTaskFields(t *testing.T) {
    now := time.Now()
    task := Task{
        Adapter:   "stats_gov",
        Status:    "pending",
        StatsJSON: `{"collected":0,"skipped":0,"errors":0}`,
        StartedAt: now,
    }
    if task.Adapter != "stats_gov" {
        t.Errorf("expected stats_gov, got %s", task.Adapter)
    }
    if task.Status != "pending" {
        t.Errorf("expected pending, got %s", task.Status)
    }
}

func TestDataRowChecksum(t *testing.T) {
    row := DataRow{
        Adapter:   "stats_gov",
        SourceURL: "https://example.com/data",
        DataJSON:  `{"indicator":"GDP","value":"100"}`,
    }
    row.SetChecksum()
    if row.Checksum == "" {
        t.Error("checksum should not be empty")
    }
    expected := row.Checksum
    row.SetChecksum()
    if row.Checksum != expected {
        t.Error("checksum should be deterministic for same data")
    }
}

func TestCrawlConfigDefaults(t *testing.T) {
    cfg := CrawlConfig{
        Adapter:   "test",
        Enabled:   true,
        RateLimit: 10,
    }
    if cfg.CronExpr != "" {
        t.Errorf("default CronExpr should be empty, got %s", cfg.CronExpr)
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/model/... -v
```
预期：FAIL — model types not defined

- [ ] **步骤 3：编写模型代码**

创建 `internal/model/model.go`：

```go
package model

import (
    "crypto/md5"
    "fmt"
    "time"
)

type Task struct {
    ID        int64      `gorm:"primaryKey;autoIncrement" json:"id"`
    Adapter   string     `gorm:"size:50;not null;index" json:"adapter"`
    Status    string     `gorm:"size:20;not null;default:pending" json:"status"`
    Error     string     `gorm:"type:text" json:"error,omitempty"`
    StatsJSON string     `gorm:"type:json" json:"stats_json"`
    StartedAt time.Time  `gorm:"not null" json:"started_at"`
    EndedAt   *time.Time `json:"ended_at,omitempty"`
}

func (Task) TableName() string { return "tasks" }

type DataRow struct {
    ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
    TaskID      int64     `gorm:"not null;index" json:"task_id"`
    Adapter     string    `gorm:"size:50;not null" json:"adapter"`
    SourceURL   string    `gorm:"size:500;not null" json:"source_url"`
    DataJSON    string    `gorm:"type:json;not null" json:"data_json"`
    CollectedAt time.Time `gorm:"not null" json:"collected_at"`
    Checksum    string    `gorm:"size:32;not null;index" json:"checksum"`
}

func (DataRow) TableName() string { return "data_rows" }

func (r *DataRow) SetChecksum() {
    raw := fmt.Sprintf("%s|%s", r.SourceURL, r.DataJSON)
    r.Checksum = fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

type CrawlConfig struct {
    Adapter   string    `gorm:"size:50;primaryKey" json:"adapter"`
    CronExpr  string    `gorm:"size:50;not null;default:''" json:"cron_expr"`
    Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
    RateLimit int       `gorm:"not null;default:10" json:"rate_limit"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (CrawlConfig) TableName() string { return "crawl_configs" }
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/model/... -v
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/model/ && git commit -m "feat: add data models (Task, DataRow, CrawlConfig)"
```

---

### 任务 3：Repository 层

**文件：**
- 创建：`internal/repository/repository.go`
- 创建：`internal/repository/repository_test.go`

- [ ] **步骤 1：编写 repository 测试（需 MySQL）**

创建 `internal/repository/repository_test.go`：

```go
package repository

import (
    "context"
    "crawler/internal/model"
    "testing"
    "time"
)

func setupTestDB(t *testing.T) *Repository {
    t.Helper()
    repo, err := New("crawler:crawler123@tcp(127.0.0.1:3306)/crawler_test?parseTime=true")
    if err != nil {
        t.Skipf("MySQL not available, skipping: %v", err)
    }
    repo.db.AutoMigrate(&model.Task{}, &model.DataRow{}, &model.CrawlConfig{})
    // clean slate
    repo.db.Exec("DELETE FROM data_rows")
    repo.db.Exec("DELETE FROM tasks")
    repo.db.Exec("DELETE FROM crawl_configs")
    return repo
}

func TestCreateAndGetTask(t *testing.T) {
    repo := setupTestDB(t)
    ctx := context.Background()
    task := &model.Task{
        Adapter: "test_adapter",
        Status:  "pending",
        StatsJSON: `{"collected":0,"skipped":0,"errors":0}`,
        StartedAt: time.Now(),
    }
    if err := repo.CreateTask(ctx, task); err != nil {
        t.Fatalf("CreateTask failed: %v", err)
    }
    if task.ID == 0 {
        t.Error("expected ID to be assigned")
    }
    got, err := repo.GetTask(ctx, task.ID)
    if err != nil {
        t.Fatalf("GetTask failed: %v", err)
    }
    if got.Adapter != "test_adapter" {
        t.Errorf("expected test_adapter, got %s", got.Adapter)
    }
}

func TestUpsertConfig(t *testing.T) {
    repo := setupTestDB(t)
    ctx := context.Background()
    cfg := &model.CrawlConfig{
        Adapter:   "test",
        CronExpr:  "0 */6 * * *",
        Enabled:   true,
        RateLimit: 5,
    }
    if err := repo.UpsertConfig(ctx, cfg); err != nil {
        t.Fatalf("UpsertConfig failed: %v", err)
    }
    cfgs, err := repo.ListEnabledConfigs(ctx)
    if err != nil {
        t.Fatalf("ListEnabledConfigs failed: %v", err)
    }
    found := false
    for _, c := range cfgs {
        if c.Adapter == "test" {
            found = true
            if c.RateLimit != 5 {
                t.Errorf("expected RateLimit=5, got %d", c.RateLimit)
            }
        }
    }
    if !found {
        t.Error("expected test adapter in config list")
    }
}

func TestBatchInsertDataRows(t *testing.T) {
    repo := setupTestDB(t)
    ctx := context.Background()
    now := time.Now()
    rows := []model.DataRow{
        {TaskID: 1, Adapter: "test", SourceURL: "https://a.com/1",
         DataJSON: `{"k":"v1"}`, CollectedAt: now},
        {TaskID: 1, Adapter: "test", SourceURL: "https://a.com/2",
         DataJSON: `{"k":"v2"}`, CollectedAt: now},
    }
    rows[0].SetChecksum()
    rows[1].SetChecksum()
    inserted, skipped, err := repo.BatchInsertDataRows(ctx, rows)
    if err != nil {
        t.Fatalf("BatchInsertDataRows failed: %v", err)
    }
    if inserted != 2 {
        t.Errorf("expected 2 inserted, got %d", inserted)
    }
    if skipped != 0 {
        t.Errorf("expected 0 skipped, got %d", skipped)
    }
    // re-insert should skip duplicates
    inserted2, skipped2, _ := repo.BatchInsertDataRows(ctx, rows)
    if inserted2 != 0 {
        t.Errorf("expected 0 inserted on re-run, got %d", inserted2)
    }
    if skipped2 != 2 {
        t.Errorf("expected 2 skipped on re-run, got %d", skipped2)
    }
}

func TestQueryData(t *testing.T) {
    repo := setupTestDB(t)
    ctx := context.Background()
    now := time.Now()
    row := model.DataRow{
        TaskID: 1, Adapter: "stats_gov", SourceURL: "https://a.com/gdp",
        DataJSON: `{"indicator":"GDP 总量","value":"126万亿"}`,
        CollectedAt: now,
    }
    row.SetChecksum()
    repo.BatchInsertDataRows(ctx, []model.DataRow{row})

    result, err := repo.QueryData(ctx, QueryParams{
        Adapter:  "stats_gov",
        Keyword:  "GDP",
        Page:     1,
        PageSize: 10,
    })
    if err != nil {
        t.Fatalf("QueryData failed: %v", err)
    }
    if result.Total == 0 {
        t.Error("expected to find the GDP row")
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/repository/... -v
```
预期：FAIL — Repository type / New function not defined

- [ ] **步骤 3：编写 Repository**

创建 `internal/repository/repository.go`：

```go
package repository

import (
    "context"
    "crawler/internal/model"
    "errors"
    "time"

    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

type Repository struct {
    db *gorm.DB
}

type QueryParams struct {
    Adapter  string
    From     string
    To       string
    Keyword  string
    Page     int
    PageSize int
}

type QueryResult struct {
    Total int64           `json:"total"`
    Page  int             `json:"page"`
    Rows  []model.DataRow `json:"rows"`
}

func New(dsn string) (*Repository, error) {
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        return nil, err
    }
    return &Repository{db: db}, nil
}

func (r *Repository) AutoMigrate() error {
    return r.db.AutoMigrate(&model.Task{}, &model.DataRow{}, &model.CrawlConfig{})
}

func (r *Repository) DB() *gorm.DB { return r.db }

// --- Task CRUD ---

func (r *Repository) CreateTask(ctx context.Context, task *model.Task) error {
    return r.db.WithContext(ctx).Create(task).Error
}

func (r *Repository) UpdateTask(ctx context.Context, task *model.Task) error {
    return r.db.WithContext(ctx).Save(task).Error
}

func (r *Repository) GetTask(ctx context.Context, id int64) (*model.Task, error) {
    var task model.Task
    err := r.db.WithContext(ctx).First(&task, id).Error
    if err != nil {
        return nil, err
    }
    return &task, nil
}

func (r *Repository) ListRecentTasks(ctx context.Context, limit int) ([]model.Task, error) {
    var tasks []model.Task
    err := r.db.WithContext(ctx).Order("started_at DESC").Limit(limit).Find(&tasks).Error
    return tasks, err
}

// --- DataRow CRUD ---

func (r *Repository) BatchInsertDataRows(ctx context.Context, rows []model.DataRow) (inserted, skipped int64, err error) {
    // INSERT IGNORE on checksum unique constraint for dedup
    result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
        Columns:   []clause.Column{{Name: "checksum"}},
        DoNothing: true,
    }).CreateInBatches(rows, 100)
    if result.Error != nil {
        return 0, 0, result.Error
    }
    inserted = result.RowsAffected
    skipped = int64(len(rows)) - inserted
    return inserted, skipped, nil
}

func (r *Repository) GetLatestCollectedAt(ctx context.Context, adapterName string) (*time.Time, error) {
    var row model.DataRow
    err := r.db.WithContext(ctx).
        Where("adapter = ?", adapterName).
        Order("collected_at DESC").
        First(&row).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return &row.CollectedAt, nil
}

func (r *Repository) QueryData(ctx context.Context, params QueryParams) (*QueryResult, error) {
    if params.Page <= 0 {
        params.Page = 1
    }
    if params.PageSize <= 0 || params.PageSize > 100 {
        params.PageSize = 20
    }
    q := r.db.WithContext(ctx).Model(&model.DataRow{})
    if params.Adapter != "" {
        q = q.Where("adapter = ?", params.Adapter)
    }
    if params.From != "" {
        q = q.Where("collected_at >= ?", params.From)
    }
    if params.To != "" {
        q = q.Where("collected_at <= ?", params.To)
    }
    if params.Keyword != "" {
        q = q.Where("data_json LIKE ?", "%"+params.Keyword+"%")
    }
    var total int64
    if err := q.Count(&total).Error; err != nil {
        return nil, err
    }
    var rows []model.DataRow
    offset := (params.Page - 1) * params.PageSize
    if err := q.Order("collected_at DESC").Offset(offset).Limit(params.PageSize).Find(&rows).Error; err != nil {
        return nil, err
    }
    return &QueryResult{Total: total, Page: params.Page, Rows: rows}, nil
}

func (r *Repository) CountByAdapter(ctx context.Context) (map[string]int64, error) {
    type count struct {
        Adapter string
        Count   int64
    }
    var results []count
    err := r.db.WithContext(ctx).Model(&model.DataRow{}).
        Select("adapter, count(*) as count").
        Group("adapter").Find(&results).Error
    if err != nil {
        return nil, err
    }
    m := make(map[string]int64)
    for _, c := range results {
        m[c.Adapter] = c.Count
    }
    return m, nil
}

// --- CrawlConfig CRUD ---

func (r *Repository) UpsertConfig(ctx context.Context, cfg *model.CrawlConfig) error {
    return r.db.WithContext(ctx).Clauses(clause.OnConflict{
        Columns:   []clause.Column{{Name: "adapter"}},
        DoUpdates: clause.AssignmentColumns([]string{"cron_expr", "enabled", "rate_limit", "updated_at"}),
    }).Create(cfg).Error
}

func (r *Repository) ListEnabledConfigs(ctx context.Context) ([]model.CrawlConfig, error) {
    var configs []model.CrawlConfig
    err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error
    return configs, err
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
# 需要 MySQL 运行中且有 crawler_test 数据库
go test ./internal/repository/... -v
```
预期：PASS（若 MySQL 不可用则 SKIP）

- [ ] **步骤 5：Commit**

```bash
git add internal/repository/ && git commit -m "feat: add repository layer with task/data/config CRUD"
```

---

### 任务 4：Adapter 接口 & 注册表

**文件：**
- 创建：`internal/adapter/adapter.go`
- 创建：`internal/adapter/registry.go`
- 创建：`internal/adapter/adapter_test.go`

- [ ] **步骤 1：编写接口测试**

创建 `internal/adapter/adapter_test.go`：

```go
package adapter

import (
    "context"
    "testing"
)

type mockAdapter struct {
    name string
}

func (m *mockAdapter) Name() string { return m.name }
func (m *mockAdapter) Validate() error { return nil }
func (m *mockAdapter) Collect(ctx context.Context, task *Task) ([]DataRow, error) {
    return []DataRow{
        {SourceURL: "https://x.com/1", Data: map[string]string{"key": "value"}},
    }, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
    reg := NewRegistry()
    a := &mockAdapter{name: "test"}
    reg.Register(a)
    got, ok := reg.Get("test")
    if !ok {
        t.Fatal("expected to find test adapter")
    }
    if got.Name() != "test" {
        t.Errorf("expected 'test', got '%s'", got.Name())
    }
    _, ok = reg.Get("nonexistent")
    if ok {
        t.Error("expected false for nonexistent adapter")
    }
}

func TestRegistryList(t *testing.T) {
    reg := NewRegistry()
    reg.Register(&mockAdapter{name: "a"})
    reg.Register(&mockAdapter{name: "b"})
    names := reg.List()
    if len(names) != 2 {
        t.Errorf("expected 2 adapters, got %d", len(names))
    }
}

func TestTaskAndStats(t *testing.T) {
    task := &Task{
        ID:      "t1",
        Adapter: "test",
        Status:  "running",
    }
    task.Stats.Collected = 100
    if task.Stats.Collected != 100 {
        t.Error("stats field not working")
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/adapter/... -v
```
预期：FAIL — types not defined

- [ ] **步骤 3：编写接口和注册表**

创建 `internal/adapter/adapter.go`：

```go
package adapter

import (
    "context"
    "time"
)

type Task struct {
    ID        string    `json:"id"`
    Adapter   string    `json:"adapter"`
    Status    string    `json:"status"`
    StartedAt time.Time `json:"started_at"`
    EndedAt   time.Time `json:"ended_at,omitempty"`
    Error     string    `json:"error,omitempty"`
    Stats     Stats     `json:"stats"`
}

type Stats struct {
    Collected int `json:"collected"`
    Skipped   int `json:"skipped"`
    Errors    int `json:"errors"`
}

type DataRow struct {
    SourceURL string            `json:"source_url"`
    Data      map[string]string `json:"data"`
}

type Adapter interface {
    Name() string
    Validate() error
    Collect(ctx context.Context, task *Task) ([]DataRow, error)
}
```

创建 `internal/adapter/registry.go`：

```go
package adapter

import "sync"

type Registry struct {
    mu       sync.RWMutex
    adapters map[string]Adapter
}

func NewRegistry() *Registry {
    return &Registry{adapters: make(map[string]Adapter)}
}

func (r *Registry) Register(a Adapter) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.adapters[a.Name()] = a
}

func (r *Registry) Get(name string) (Adapter, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    a, ok := r.adapters[name]
    return a, ok
}

func (r *Registry) List() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    names := make([]string, 0, len(r.adapters))
    for name := range r.adapters {
        names = append(names, name)
    }
    return names
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/adapter/... -v
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/adapter/ && git commit -m "feat: add Adapter interface and Registry"
```

---

### 任务 5：RateLimiter

**文件：**
- 创建：`internal/engine/limiter.go`
- 创建：`internal/engine/limiter_test.go`

- [ ] **步骤 1：编写限流器测试**

创建 `internal/engine/limiter_test.go`：

```go
package engine

import (
    "testing"
    "time"
)

func TestTokenBucketAllowsWithinLimit(t *testing.T) {
    lim := NewRateLimiter(10) // 10 tokens/sec
    allowed := 0
    for i := 0; i < 10; i++ {
        if lim.Allow() {
            allowed++
        }
    }
    if allowed != 10 {
        t.Errorf("expected 10 allowed, got %d", allowed)
    }
}

func TestTokenBucketBlocksOverLimit(t *testing.T) {
    lim := NewRateLimiter(10)
    for i := 0; i < 10; i++ {
        lim.Allow()
    }
    if lim.Allow() {
        t.Error("expected false after exhausting tokens")
    }
}

func TestTokenBucketRefills(t *testing.T) {
    lim := NewRateLimiter(100)
    for i := 0; i < 100; i++ {
        lim.Allow()
    }
    if lim.Allow() {
        t.Error("should be empty")
    }
    time.Sleep(50 * time.Millisecond)
    if !lim.Allow() {
        t.Error("should have refilled at least 1 token after 50ms")
    }
}

func TestMultiLimiter(t *testing.T) {
    ml := NewMultiLimiter(10) // global 10/sec
    ml.SetAdapterLimit("a", 5)
    // adapter 'a' can't exceed 5
    allowed := 0
    for i := 0; i < 10; i++ {
        if ml.Allow("a") {
            allowed++
        }
    }
    if allowed > 5 {
        t.Errorf("adapter limit breached: %d > 5", allowed)
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/engine/... -v -run TestToken
```
预期：FAIL

- [ ] **步骤 3：编写限流器实现**

创建 `internal/engine/limiter.go`：

```go
package engine

import (
    "sync"
    "time"
)

type RateLimiter struct {
    rate       float64    // tokens per second
    tokens     float64
    lastRefill time.Time
    mu         sync.Mutex
}

func NewRateLimiter(ratePerSec float64) *RateLimiter {
    return &RateLimiter{
        rate:       ratePerSec,
        tokens:     ratePerSec,
        lastRefill: time.Now(),
    }
}

func (r *RateLimiter) Allow() bool {
    r.mu.Lock()
    defer r.mu.Unlock()
    now := time.Now()
    elapsed := now.Sub(r.lastRefill).Seconds()
    r.tokens += elapsed * r.rate
    if r.tokens > r.rate {
        r.tokens = r.rate
    }
    r.lastRefill = now
    if r.tokens >= 1 {
        r.tokens--
        return true
    }
    return false
}

type MultiLimiter struct {
    global    *RateLimiter
    perAdapter map[string]*RateLimiter
    mu        sync.RWMutex
}

func NewMultiLimiter(globalRate float64) *MultiLimiter {
    return &MultiLimiter{
        global:     NewRateLimiter(globalRate),
        perAdapter: make(map[string]*RateLimiter),
    }
}

func (m *MultiLimiter) SetAdapterLimit(name string, rate float64) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.perAdapter[name] = NewRateLimiter(rate)
}

func (m *MultiLimiter) Allow(adapter string) bool {
    if !m.global.Allow() {
        return false
    }
    m.mu.RLock()
    lim, ok := m.perAdapter[adapter]
    m.mu.RUnlock()
    if !ok {
        return true
    }
    return lim.Allow()
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/engine/... -v -run TestToken
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/engine/limiter.go internal/engine/limiter_test.go && \
  git commit -m "feat: add token bucket rate limiter with per-adapter support"
```

---

### 任务 6：RetryManager

**文件：**
- 创建：`internal/engine/retry.go`
- 创建：`internal/engine/retry_test.go`

- [ ] **步骤 1：编写重试测试**

创建 `internal/engine/retry_test.go`：

```go
package engine

import (
    "context"
    "errors"
    "testing"
    "time"
)

func TestRetrySucceedsOnRetryableError(t *testing.T) {
    cfg := RetryConfig{MaxRetries: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
    rm := NewRetryManager(cfg)
    attempts := 0
    err := rm.Do(context.Background(), func() error {
        attempts++
        if attempts < 3 {
            return &RetryableError{Err: errors.New("timeout")}
        }
        return nil
    })
    if err != nil {
        t.Fatalf("expected success, got: %v", err)
    }
    if attempts != 3 {
        t.Errorf("expected 3 attempts, got %d", attempts)
    }
}

func TestRetryGivesUp(t *testing.T) {
    cfg := RetryConfig{MaxRetries: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
    rm := NewRetryManager(cfg)
    attempts := 0
    err := rm.Do(context.Background(), func() error {
        attempts++
        return &RetryableError{Err: errors.New("timeout")}
    })
    if err == nil {
        t.Fatal("expected error")
    }
    if attempts != 3 {
        t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", attempts)
    }
}

func TestNoRetryOnNonRetryableError(t *testing.T) {
    cfg := RetryConfig{MaxRetries: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
    rm := NewRetryManager(cfg)
    attempts := 0
    err := rm.Do(context.Background(), func() error {
        attempts++
        return errors.New("parse error")
    })
    if err == nil {
        t.Fatal("expected error")
    }
    if attempts != 1 {
        t.Errorf("expected 1 attempt for non-retryable, got %d", attempts)
    }
}

func TestRetryRespectsContext(t *testing.T) {
    cfg := RetryConfig{MaxRetries: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: 1 * time.Second}
    rm := NewRetryManager(cfg)
    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()
    err := rm.Do(ctx, func() error {
        return &RetryableError{Err: errors.New("timeout")}
    })
    if err == nil {
        t.Fatal("expected context deadline error")
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/engine/... -v -run TestRetry
```
预期：FAIL

- [ ] **步骤 3：编写重试实现**

创建 `internal/engine/retry.go`：

```go
package engine

import (
    "context"
    "fmt"
    "math"
    "time"
)

type RetryableError struct{ Err error }

func (e *RetryableError) Error() string { return fmt.Sprintf("retryable: %s", e.Err) }
func (e *RetryableError) Unwrap() error { return e.Err }

type RetryConfig struct {
    MaxRetries int
    BaseDelay  time.Duration
    MaxDelay   time.Duration
}

type RetryManager struct{ cfg RetryConfig }

func NewRetryManager(cfg RetryConfig) *RetryManager {
    if cfg.MaxRetries <= 0 {
        cfg.MaxRetries = 3
    }
    if cfg.BaseDelay <= 0 {
        cfg.BaseDelay = 1 * time.Second
    }
    if cfg.MaxDelay <= 0 {
        cfg.MaxDelay = 30 * time.Second
    }
    return &RetryManager{cfg: cfg}
}

func (r *RetryManager) Do(ctx context.Context, fn func() error) error {
    var lastErr error
    for attempt := 0; attempt <= r.cfg.MaxRetries; attempt++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        err := fn()
        if err == nil {
            return nil
        }
        lastErr = err
        if _, ok := err.(*RetryableError); !ok {
            return err
        }
        if attempt == r.cfg.MaxRetries {
            break
        }
        delay := time.Duration(float64(r.cfg.BaseDelay) * math.Pow(2, float64(attempt)))
        if delay > r.cfg.MaxDelay {
            delay = r.cfg.MaxDelay
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(delay):
        }
    }
    return fmt.Errorf("exhausted %d retries: %w", r.cfg.MaxRetries, lastErr)
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/engine/... -v -run TestRetry
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/engine/retry.go internal/engine/retry_test.go && \
  git commit -m "feat: add exponential backoff retry manager"
```

---

### 任务 7：Engine 核心 + ProxyPool 空实现

**文件：**
- 创建：`internal/engine/proxy.go`
- 创建：`internal/engine/engine.go`
- 创建：`internal/engine/engine_test.go`

- [ ] **步骤 1：编写 Engine 测试**

创建 `internal/engine/engine_test.go`：

```go
package engine

import (
    "context"
    "crawler/internal/adapter"
    "crawler/internal/model"
    "errors"
    "testing"
    "time"
)

type stubRepo struct {
    tasks       map[int64]*model.Task
    dataRows    []model.DataRow
    configs     map[string]model.CrawlConfig
}

func (s *stubRepo) CreateTask(ctx context.Context, t *model.Task) error {
    t.ID = int64(len(s.tasks) + 1)
    s.tasks[t.ID] = t
    return nil
}
func (s *stubRepo) UpdateTask(ctx context.Context, t *model.Task) error {
    s.tasks[t.ID] = t
    return nil
}
func (s *stubRepo) GetTask(ctx context.Context, id int64) (*model.Task, error) {
    t, ok := s.tasks[id]
    if !ok { return nil, errors.New("not found") }
    return t, nil
}
func (s *stubRepo) BatchInsertDataRows(ctx context.Context, rows []model.DataRow) (int64, int64, error) {
    s.dataRows = append(s.dataRows, rows...)
    return int64(len(rows)), 0, nil
}
func (s *stubRepo) GetLatestCollectedAt(ctx context.Context, name string) (*time.Time, error) {
    return nil, nil
}
func (s *stubRepo) ListEnabledConfigs(ctx context.Context) ([]model.CrawlConfig, error) {
    var cfgs []model.CrawlConfig
    for _, c := range s.configs {
        if c.Enabled {
            cfgs = append(cfgs, c)
        }
    }
    return cfgs, nil
}
func (s *stubRepo) CountByAdapter(ctx context.Context) (map[string]int64, error) {
    return nil, nil
}
func (s *stubRepo) QueryData(ctx context.Context, p QueryParams) (*QueryResult, error) {
    return nil, nil
}
func (s *stubRepo) UpsertConfig(ctx context.Context, c *model.CrawlConfig) error {
    return nil
}
func (s *stubRepo) ListRecentTasks(ctx context.Context, limit int) ([]model.Task, error) {
    return nil, nil
}

type stubAdapter struct {
    name       string
    collectFn  func(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error)
}

func (s *stubAdapter) Name() string { return s.name }
func (s *stubAdapter) Validate() error { return nil }
func (s *stubAdapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    return s.collectFn(ctx, task)
}

func TestEngineCollectSuccess(t *testing.T) {
    repo := &stubRepo{
        tasks:   make(map[int64]*model.Task),
        configs: map[string]model.CrawlConfig{"test": {Adapter: "test", Enabled: true, RateLimit: 10}},
    }
    reg := adapter.NewRegistry()
    reg.Register(&stubAdapter{
        name: "test",
        collectFn: func(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
            return []adapter.DataRow{{SourceURL: "https://x.com/1", Data: map[string]string{"a": "1"}}}, nil
        },
    })
    ml := NewMultiLimiter(100)
    rm := NewRetryManager(RetryConfig{MaxRetries: 1, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond})
    eng := New(repo, reg, ml, rm)

    ctx := context.Background()
    taskID, err := eng.Collect(ctx, "test")
    if err != nil {
        t.Fatalf("Collect failed: %v", err)
    }
    if taskID <= 0 {
        t.Errorf("expected task ID > 0, got %d", taskID)
    }
    task, _ := repo.GetTask(ctx, taskID)
    if task.Status != "done" {
        t.Errorf("expected done, got %s", task.Status)
    }
    if len(repo.dataRows) != 1 {
        t.Errorf("expected 1 data row, got %d", len(repo.dataRows))
    }
}

func TestEngineCollectAdapterNotFound(t *testing.T) {
    repo := &stubRepo{tasks: make(map[int64]*model.Task), configs: make(map[string]model.CrawlConfig)}
    reg := adapter.NewRegistry()
    eng := New(repo, reg, NewMultiLimiter(100), NewRetryManager(RetryConfig{}))
    _, err := eng.Collect(context.Background(), "nonexistent")
    if err == nil {
        t.Fatal("expected error for nonexistent adapter")
    }
}
```

**注意：** stubRepo 需要实现 Engine 依赖的接口。我们在 engine_test.go 中定义 `Repository` 接口。

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/engine/... -v -run TestEngine
```
预期：FAIL — Engine not defined

- [ ] **步骤 3：编写 ProxyPool 空实现**

创建 `internal/engine/proxy.go`：

```go
package engine

import "net/url"

type ProxyPool struct{}

func NewProxyPool() *ProxyPool { return &ProxyPool{} }

func (p *ProxyPool) Next() (*url.URL, error) {
    return nil, nil // Phase 2: return actual proxy
}

func (p *ProxyPool) MarkBad(u *url.URL) {
    // Phase 2: remove from pool
}
```

- [ ] **步骤 4：编写 Engine 核心**

创建 `internal/engine/engine.go`：

```go
package engine

import (
    "context"
    "crawler/internal/adapter"
    "crawler/internal/model"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"

    "go.uber.org/zap"
)

type Repository interface {
    CreateTask(ctx context.Context, task *model.Task) error
    UpdateTask(ctx context.Context, task *model.Task) error
    GetTask(ctx context.Context, id int64) (*model.Task, error)
    BatchInsertDataRows(ctx context.Context, rows []model.DataRow) (int64, int64, error)
    GetLatestCollectedAt(ctx context.Context, adapterName string) (*time.Time, error)
    ListEnabledConfigs(ctx context.Context) ([]model.CrawlConfig, error)
    CountByAdapter(ctx context.Context) (map[string]int64, error)
    QueryData(ctx context.Context, params QueryParams) (*QueryResult, error)
    UpsertConfig(ctx context.Context, cfg *model.CrawlConfig) error
    ListRecentTasks(ctx context.Context, limit int) ([]model.Task, error)
}

type QueryParams = struct {
    Adapter  string
    From     string
    To       string
    Keyword  string
    Page     int
    PageSize int
}

type QueryResult struct {
    Total int64         `json:"total"`
    Page  int           `json:"page"`
    Rows  []model.DataRow `json:"rows"`
}

type Engine struct {
    repo    Repository
    reg     *adapter.Registry
    limiter *MultiLimiter
    retry   *RetryManager
    logger  *zap.Logger
}

func New(repo Repository, reg *adapter.Registry, limiter *MultiLimiter, retry *RetryManager) *Engine {
    logger, _ := zap.NewProduction()
    return &Engine{repo: repo, reg: reg, limiter: limiter, retry: retry, logger: logger}
}

func (e *Engine) SetLogger(logger *zap.Logger) { e.logger = logger }

func generateID() string {
    b := make([]byte, 8)
    rand.Read(b)
    return hex.EncodeToString(b)
}

func (e *Engine) Collect(ctx context.Context, adapterName string) (int64, error) {
    a, ok := e.reg.Get(adapterName)
    if !ok {
        return 0, fmt.Errorf("adapter %q not found", adapterName)
    }
    task := &model.Task{
        Adapter:   adapterName,
        Status:    "running",
        StatsJSON: `{"collected":0,"skipped":0,"errors":0}`,
        StartedAt: time.Now(),
    }
    if err := e.repo.CreateTask(ctx, task); err != nil {
        return 0, fmt.Errorf("create task: %w", err)
    }
    go e.runCollection(context.Background(), task, a)
    return task.ID, nil
}

func (e *Engine) runCollection(ctx context.Context, dbTask *model.Task, a adapter.Adapter) {
    adTask := &adapter.Task{
        ID:      generateID(),
        Adapter: a.Name(),
        Status:  "running",
        StartedAt: time.Now(),
    }
    var rows []adapter.DataRow
    var collectErr error

    // apply rate limit, then retry collection
    e.limiter.Allow(a.Name())
    collectErr = e.retry.Do(ctx, func() error {
        var err error
        rows, err = a.Collect(ctx, adTask)
        if err != nil {
            return &RetryableError{Err: err}
        }
        return nil
    })

    now := time.Now()
    if collectErr != nil {
        dbTask.Status = "failed"
        dbTask.Error = collectErr.Error()
        dbTask.EndedAt = &now
        dbTask.StatsJSON = fmt.Sprintf(`{"collected":0,"skipped":0,"errors":1}`)
        e.repo.UpdateTask(ctx, dbTask)
        return
    }

    collected := 0
    skipped := 0
    errors := 0

    // convert to model.DataRow and batch insert
    modelRows := make([]model.DataRow, 0, len(rows))
    for _, r := range rows {
        dataJSON := "{"
        for k, v := range r.Data {
            if len(dataJSON) > 1 {
                dataJSON += ","
            }
            dataJSON += fmt.Sprintf(`"%s":"%s"`, k, v)
        }
        dataJSON += "}"
        mr := model.DataRow{
            TaskID:      dbTask.ID,
            Adapter:     a.Name(),
            SourceURL:   r.SourceURL,
            DataJSON:    dataJSON,
            CollectedAt: now,
        }
        mr.SetChecksum()
        modelRows = append(modelRows, mr)
    }
    ins, skp, err := e.repo.BatchInsertDataRows(ctx, modelRows)
    collected = int(ins)
    skipped = int(skp)
    if err != nil {
        errors++
    }

    dbTask.Status = "done"
    dbTask.EndedAt = &now
    dbTask.StatsJSON = fmt.Sprintf(`{"collected":%d,"skipped":%d,"errors":%d}`, collected, skipped, errors)
    e.repo.UpdateTask(ctx, dbTask)
}
```

- [ ] **步骤 5：运行测试验证通过**

```bash
go test ./internal/engine/... -v -run TestEngine
```
预期：PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/engine/ && git commit -m "feat: add engine core with collection orchestration and proxy pool stub"
```

---

### 任务 8：国家统计局 Adapter

**文件：**
- 创建：`internal/adapter/stats_gov/adapter.go`
- 创建：`internal/adapter/stats_gov/adapter_test.go`

- [ ] **步骤 1：编写测试（模拟 HTTP）**

创建 `internal/adapter/stats_gov/adapter_test.go`：

```go
package stats_gov

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "crawler/internal/adapter"
)

func TestValidate(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`<html><body><table class="data"><tr><td>GDP</td><td>100</td></tr></table></body></html>`))
    }))
    defer srv.Close()
    a := New(srv.URL)
    if err := a.Validate(); err != nil {
        t.Fatalf("Validate failed: %v", err)
    }
}

func TestCollectReturnsData(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`
<html><body>
  <table class="MsoNormalTable">
    <tr><td>指标名称</td><td>2023年</td><td>2024年</td></tr>
    <tr><td>年末总人口</td><td>140967</td><td>140828</td></tr>
    <tr><td>国内生产总值</td><td>1260582</td><td>1349084</td></tr>
  </table>
</body></html>`))
    }))
    defer srv.Close()
    a := New(srv.URL)
    task := &adapter.Task{ID: "t1", Adapter: "stats_gov"}
    rows, err := a.Collect(context.Background(), task)
    if err != nil {
        t.Fatalf("Collect failed: %v", err)
    }
    if len(rows) == 0 {
        t.Fatal("expected some data rows")
    }
    t.Logf("collected %d rows", len(rows))
    for _, row := range rows {
        if row.SourceURL == "" {
            t.Error("SourceURL should not be empty")
        }
        if len(row.Data) == 0 {
            t.Error("Data should not be empty")
        }
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/adapter/stats_gov/... -v
```
预期：FAIL — New not defined

- [ ] **步骤 3：编写国家统计局 Adapter**

创建 `internal/adapter/stats_gov/adapter.go`：

```go
package stats_gov

import (
    "context"
    "crawler/internal/adapter"
    "fmt"
    "net/url"
    "strings"

    "github.com/gocolly/colly/v2"
)

type Adapter struct {
    baseURL    string
    collector  *colly.Collector
}

func New(baseURL string) *Adapter {
    return &Adapter{
        baseURL:   baseURL,
        collector: colly.NewCollector(colly.AllowedDomains()),
    }
}

func (a *Adapter) Name() string { return "stats_gov" }

func (a *Adapter) Validate() error {
    c := colly.NewCollector()
    c.AllowURLRevisit = false
    var visitErr error
    c.OnError(func(r *colly.Response, err error) {
        visitErr = err
    })
    c.Visit(a.baseURL)
    return visitErr
}

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    var rows []adapter.DataRow
    c := colly.NewCollector()
    seen := make(map[string]bool)

    // discover links to sub-pages with data tables
    c.OnHTML("a[href]", func(e *colly.HTMLElement) {
        link := e.Attr("href")
        if strings.Contains(link, "html") || strings.Contains(link, "sj") || strings.Contains(link, "data") {
            absURL := e.Request.AbsoluteURL(link)
            if !seen[absURL] {
                seen[absURL] = true
                c.Visit(absURL)
            }
        }
    })

    // extract table rows
    c.OnHTML("table tbody tr, table tr", func(e *colly.HTMLElement) {
        cells := e.ChildTexts("td, th")
        if len(cells) < 2 {
            return
        }
        data := make(map[string]string)
        data["indicator"] = strings.TrimSpace(cells[0])
        for i := 1; i < len(cells); i++ {
            data[fmt.Sprintf("value_%d", i)] = strings.TrimSpace(cells[i])
        }
        rows = append(rows, adapter.DataRow{
            SourceURL: e.Request.URL.String(),
            Data:      data,
        })
    })

    // support pagination — click "next page"
    c.OnHTML("a.next, a[rel=next], a:contains(下一页), a:contains(下页)", func(e *colly.HTMLElement) {
        e.Request.Visit(e.Attr("href"))
    })

    u, err := url.Parse(a.baseURL)
    if err != nil {
        return nil, fmt.Errorf("parse base URL: %w", err)
    }
    c.AllowedDomains = []string{u.Host}

    if err := c.Visit(a.baseURL); err != nil {
        return nil, fmt.Errorf("visit %s: %w", a.baseURL, err)
    }
    c.Wait()

    return rows, nil
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/adapter/stats_gov/... -v
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/adapter/stats_gov/ && git commit -m "feat: add stats_gov adapter with full collection"
```

---

### 任务 9：行业排行榜 Adapter

**文件：**
- 创建：`internal/adapter/hangye_paihang/adapter.go`
- 创建：`internal/adapter/hangye_paihang/adapter_test.go`

- [ ] **步骤 1：编写测试**

创建 `internal/adapter/hangye_paihang/adapter_test.go`：

```go
package hangye_paihang

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "crawler/internal/adapter"
)

func TestCollectIncremental(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`
<html><body>
  <div class="ranking-list">
    <div class="rank-item">
      <span class="rank">1</span>
      <span class="name">品牌A</span>
      <span class="score">98.5</span>
    </div>
    <div class="rank-item">
      <span class="rank">2</span>
      <span class="name">品牌B</span>
      <span class="score">95.2</span>
    </div>
  </div>
</body></html>`))
    }))
    defer srv.Close()
    a := New(srv.URL)
    task := &adapter.Task{ID: "t2", Adapter: "hangye_paihang"}
    rows, err := a.Collect(context.Background(), task)
    if err != nil {
        t.Fatalf("Collect failed: %v", err)
    }
    if len(rows) == 0 {
        t.Fatal("expected ranking data")
    }
    t.Logf("collected %d ranking items", len(rows))
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/adapter/hangye_paihang/... -v
```
预期：FAIL

- [ ] **步骤 3：编写行业排行榜 Adapter**

创建 `internal/adapter/hangye_paihang/adapter.go`：

```go
package hangye_paihang

import (
    "context"
    "crawler/internal/adapter"
    "fmt"
    "net/url"
    "strings"

    "github.com/gocolly/colly/v2"
)

type Adapter struct {
    baseURL   string
    collector *colly.Collector
}

func New(baseURL string) *Adapter {
    return &Adapter{baseURL: baseURL, collector: colly.NewCollector()}
}

func (a *Adapter) Name() string { return "hangye_paihang" }

func (a *Adapter) Validate() error {
    c := colly.NewCollector()
    var visitErr error
    c.OnError(func(r *colly.Response, err error) {
        visitErr = err
    })
    c.Visit(a.baseURL)
    return visitErr
}

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    var rows []adapter.DataRow
    c := colly.NewCollector()

    // Extract rank items — the selectors below are templates; adjust per actual site
    c.OnHTML(".rank-item, .ranking-item, li[class*=rank], tr[class*=rank]", func(e *colly.HTMLElement) {
        rank := strings.TrimSpace(e.ChildText(".rank, .ranking, td:first-child"))
        name := strings.TrimSpace(e.ChildText(".name, .title, td:nth-child(2)"))
        score := strings.TrimSpace(e.ChildText(".score, .value, td:nth-child(3)"))
        if name == "" {
            // fallback: grab all text
            texts := e.ChildTexts("span, td, p")
            if len(texts) >= 2 {
                rank = texts[0]
                name = texts[1]
                if len(texts) >= 3 {
                    score = texts[2]
                }
            }
        }
        if name != "" {
            rows = append(rows, adapter.DataRow{
                SourceURL: e.Request.URL.String(),
                Data: map[string]string{
                    "rank":  rank,
                    "name":  name,
                    "score": score,
                },
            })
        }
    })

    // pagination
    c.OnHTML("a.next, a:contains(下一页)", func(e *colly.HTMLElement) {
        e.Request.Visit(e.Attr("href"))
    })

    u, err := url.Parse(a.baseURL)
    if err != nil {
        return nil, fmt.Errorf("parse base URL: %w", err)
    }
    c.AllowedDomains = []string{u.Host}

    if err := c.Visit(a.baseURL); err != nil {
        return nil, fmt.Errorf("visit %s: %w", a.baseURL, err)
    }
    c.Wait()

    return rows, nil
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/adapter/hangye_paihang/... -v
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/adapter/hangye_paihang/ && git commit -m "feat: add hangye_paihang adapter with incremental collection"
```

---

### 任务 10：Scheduler

**文件：**
- 创建：`internal/scheduler/scheduler.go`

- [ ] **步骤 1：编写 Scheduler**

创建 `internal/scheduler/scheduler.go`：

```go
package scheduler

import (
    "context"
    "fmt"
    "sync"

    "github.com/robfig/cron/v3"
    "go.uber.org/zap"
)

type Engine interface {
    Collect(ctx context.Context, adapterName string) (int64, error)
}

type Scheduler struct {
    cron   *cron.Cron
    engine Engine
    logger *zap.Logger
    mu     sync.Mutex
    jobs   map[string]cron.EntryID // adapter -> cron entry
}

func New(engine Engine, logger *zap.Logger) *Scheduler {
    return &Scheduler{
        cron:   cron.New(),
        engine: engine,
        logger: logger,
        jobs:   make(map[string]cron.EntryID),
    }
}

func (s *Scheduler) AddJob(adapterName, cronExpr string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if existing, ok := s.jobs[adapterName]; ok {
        s.cron.Remove(existing)
    }
    if cronExpr == "" {
        return nil // manual only
    }
    id, err := s.cron.AddFunc(cronExpr, func() {
        s.logger.Info("cron triggered", zap.String("adapter", adapterName))
        _, err := s.engine.Collect(context.Background(), adapterName)
        if err != nil {
            s.logger.Error("cron collect failed", zap.String("adapter", adapterName), zap.Error(err))
        }
    })
    if err != nil {
        return fmt.Errorf("add cron for %s: %w", adapterName, err)
    }
    s.jobs[adapterName] = id
    s.logger.Info("scheduled cron job", zap.String("adapter", adapterName), zap.String("cron", cronExpr))
    return nil
}

func (s *Scheduler) RemoveJob(adapterName string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if id, ok := s.jobs[adapterName]; ok {
        s.cron.Remove(id)
        delete(s.jobs, adapterName)
    }
}

func (s *Scheduler) Start() {
    s.logger.Info("scheduler started")
    s.cron.Start()
}

func (s *Scheduler) Stop() context.Context {
    s.logger.Info("scheduler stopping")
    return s.cron.Stop()
}

func (s *Scheduler) Trigger(adapterName string) (int64, error) {
    s.logger.Info("manual trigger", zap.String("adapter", adapterName))
    return s.engine.Collect(context.Background(), adapterName)
}
```

- [ ] **步骤 2：验证编译**

```bash
go build ./internal/scheduler/...
```
预期：成功

- [ ] **步骤 3：Commit**

```bash
git add internal/scheduler/ && git commit -m "feat: add cron scheduler with manual trigger support"
```

---

### 任务 11：API Handlers

**文件：**
- 创建：`internal/api/handler.go`

- [ ] **步骤 1：编写 Handler**

创建 `internal/api/handler.go`：

```go
package api

import (
    "context"
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
    QueryData(ctx context.Context, params QueryParams) (*QueryResult, error)
    CountByAdapter(ctx context.Context) (map[string]int64, error)
    ListEnabledConfigs(ctx context.Context) ([]model.CrawlConfig, error)
    UpsertConfig(ctx context.Context, cfg *model.CrawlConfig) error
}

type QueryParams = struct {
    Adapter  string
    From     string
    To       string
    Keyword  string
    Page     int
    PageSize int
}

type QueryResult struct {
    Total int64         `json:"total"`
    Page  int           `json:"page"`
    Rows  []model.DataRow `json:"rows"`
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

type Handler struct {
    engine    Engine
    repo      Repository
    scheduler interface {
        Trigger(adapterName string) (int64, error)
        AddJob(adapterName, cronExpr string) error
        RemoveJob(adapterName string)
    }
    reg      interface{ List() []string }
    logger   *zap.Logger
    // map adapter name → metadata
    adapterMeta map[string]AdapterMeta
}

type AdapterMeta struct {
    Label    string
    Category string
}

func NewHandler(
    engine Engine,
    repo Repository,
    scheduler interface {
        Trigger(adapterName string) (int64, error)
        AddJob(adapterName, cronExpr string) error
        RemoveJob(adapterName string)
    },
    reg interface{ List() []string },
    logger *zap.Logger,
) *Handler {
    return &Handler{
        engine:    engine,
        repo:      repo,
        scheduler: scheduler,
        reg:       reg,
        logger:    logger,
        adapterMeta: map[string]AdapterMeta{
            "stats_gov":       {Label: "国家统计局", Category: "government"},
            "hangye_paihang":  {Label: "行业排行榜", Category: "ranking"},
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
        Mode     string   `json:"mode"` // full | incremental
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
    params := QueryParams{
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
        "total_rows":  total,
        "by_adapter":  counts,
        "recent_tasks": recentTasks,
        "system_health": "ok",
    })
}
```

**注意：** handler.go 中 `model` 类型的 import 需要在 Task 11 router/main.go 中最终确定。此处假设 import 路径 `crawler/internal/model`。

- [ ] **步骤 2：验证编译**

```bash
go build ./internal/api/...
```
预期：成功（可能需要补充 import）

- [ ] **步骤 3：Commit**

```bash
git add internal/api/ && git commit -m "feat: add API handlers for 5 endpoints"
```

---

### 任务 12：Router + main.go 入口

**文件：**
- 创建：`internal/api/router.go`
- 创建：`cmd/crawler/main.go`

- [ ] **步骤 1：编写 Router**

创建 `internal/api/router.go`：

```go
package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, h *Handler) {
    v1 := r.Group("/api/v1")
    {
        v1.GET("/adapters", h.ListAdapters)
        v1.POST("/crawl", h.TriggerCrawl)
        v1.GET("/crawl/:task_id", h.GetTaskStatus)
        v1.GET("/data", h.QueryData)
        v1.GET("/stats", h.GetStats)
    }
}
```

- [ ] **步骤 2：编写 main.go**

创建 `cmd/crawler/main.go`：

```go
package main

import (
    "fmt"
    "log"
    "strings"

    "crawler/internal/adapter"
    "crawler/internal/adapter/hangye_paihang"
    "crawler/internal/adapter/stats_gov"
    "crawler/internal/api"
    "crawler/internal/engine"
    "crawler/internal/repository"
    "crawler/internal/scheduler"

    "github.com/gin-gonic/gin"
    "github.com/spf13/viper"
    "go.uber.org/zap"
)

type AdapterCfg struct {
    Name      string `mapstructure:"name"`
    Enabled   bool   `mapstructure:"enabled"`
    Cron      string `mapstructure:"cron"`
    RateLimit int    `mapstructure:"rate_limit"`
    Mode      string `mapstructure:"mode"`
}

type Config struct {
    Server struct {
        Port int    `mapstructure:"port"`
        Mode string `mapstructure:"mode"`
    } `mapstructure:"server"`
    Database struct {
        Host          string `mapstructure:"host"`
        Port          int    `mapstructure:"port"`
        User          string `mapstructure:"user"`
        Password      string `mapstructure:"password"`
        Name          string `mapstructure:"name"`
        MaxOpenConns  int    `mapstructure:"max_open_conns"`
    } `mapstructure:"database"`
    Engine struct {
        GlobalRateLimit float64 `mapstructure:"global_rate_limit"`
        Retry           struct {
            MaxRetries int    `mapstructure:"max_retries"`
            BaseDelay  string `mapstructure:"base_delay"`
            MaxDelay   string `mapstructure:"max_delay"`
        } `mapstructure:"retry"`
    } `mapstructure:"engine"`
    Adapters []AdapterCfg `mapstructure:"adapters"`
}

func main() {
    logger, _ := zap.NewDevelopment()
    defer logger.Sync()

    // --- Config ---
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("config")
    viper.AddConfigPath(".")
    viper.SetEnvPrefix("CRAWLER")
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    viper.AutomaticEnv()
    if err := viper.ReadInConfig(); err != nil {
        logger.Fatal("failed to read config", zap.Error(err))
    }
    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        logger.Fatal("failed to unmarshal config", zap.Error(err))
    }

    // --- Database ---
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
        cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)
    repo, err := repository.New(dsn)
    if err != nil {
        logger.Fatal("failed to connect database", zap.Error(err))
    }
    if err := repo.AutoMigrate(); err != nil {
        logger.Fatal("failed to auto migrate", zap.Error(err))
    }

    // --- Adapter Registry ---
    reg := adapter.NewRegistry()
    statsGov := stats_gov.New("https://www.stats.gov.cn/sj/")
    hangye := hangye_paihang.New("https://www.example.com/industry-ranking")
    reg.Register(statsGov)
    reg.Register(hangye)

    // --- Engine ---
    limiter := engine.NewMultiLimiter(cfg.Engine.GlobalRateLimit)
    for _, aCfg := range cfg.Adapters {
        if aCfg.RateLimit > 0 {
            limiter.SetAdapterLimit(aCfg.Name, float64(aCfg.RateLimit))
        }
    }
    retryMgr := engine.NewRetryManager(engine.RetryConfig{
        MaxRetries: cfg.Engine.Retry.MaxRetries,
        BaseDelay:  parseDuration(cfg.Engine.Retry.BaseDelay, "1s"),
        MaxDelay:   parseDuration(cfg.Engine.Retry.MaxDelay, "30s"),
    })
    eng := engine.New(repo, reg, limiter, retryMgr)
    eng.SetLogger(logger)

    // --- Scheduler ---
    sched := scheduler.New(eng, logger)
    for _, aCfg := range cfg.Adapters {
        if aCfg.Enabled && aCfg.Cron != "" {
            if err := sched.AddJob(aCfg.Name, aCfg.Cron); err != nil {
                logger.Error("failed to add cron job", zap.String("adapter", aCfg.Name), zap.Error(err))
            }
        }
    }
    sched.Start()

    // --- API ---
    if cfg.Server.Mode == "release" {
        gin.SetMode(gin.ReleaseMode)
    }
    router := gin.Default()
    handler := api.NewHandler(eng, repo, sched, reg, logger)
    api.RegisterRoutes(router, handler)

    addr := fmt.Sprintf(":%d", cfg.Server.Port)
    logger.Info("server starting", zap.String("addr", addr))
    if err := router.Run(addr); err != nil {
        logger.Fatal("server failed", zap.Error(err))
    }
}

func parseDuration(s, defaultVal string) time.Duration {
    d, err := time.ParseDuration(s)
    if err != nil {
        d, _ = time.ParseDuration(defaultVal)
    }
    return d
}
```

**注意：** main.go 需要 import `time`。

- [ ] **步骤 2：修复 API handler 中的 import**

更新 `internal/api/handler.go`，确保顶部 import 完整：

```go
import (
    "context"
    "crawler/internal/model"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)
```

- [ ] **步骤 3：验证完整编译**

```bash
go build ./...
go build -o crawler.exe ./cmd/crawler/
```
预期：全部成功

- [ ] **步骤 4：Commit**

```bash
git add cmd/ internal/api/router.go && \
  git commit -m "feat: add router, main entry point, wire everything together"
```

---

### 任务 13：集成验证

**文件：**
- 创建：`.env.example`

- [ ] **步骤 1：创建环境配置示例**

创建 `.env.example`：

```
CRAWLER_DB_PASSWORD=crawler123
```

- [ ] **步骤 2：初始化 MySQL 数据库**

```sql
CREATE DATABASE IF NOT EXISTS crawler_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS crawler_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

- [ ] **步骤 3：运行服务**

```bash
set CRAWLER_DB_PASSWORD=crawler123
go run ./cmd/crawler/
```
预期：服务启动在 :8080，日志显示 scheduler 已注册 cron 任务

- [ ] **步骤 4：测试 API**

```bash
# 列出适配器
curl http://localhost:8080/api/v1/adapters

# 手动触发采集
curl -X POST http://localhost:8080/api/v1/crawl \
  -H "Content-Type: application/json" \
  -d '{"adapters":["stats_gov"]}'

# 查询任务状态（用返回的 task id）
curl http://localhost:8080/api/v1/crawl/1

# 查询数据
curl "http://localhost:8080/api/v1/data?adapter=stats_gov&page=1&page_size=10"

# 统计概览
curl http://localhost:8080/api/v1/stats
```
预期：每个端点返回正确的 JSON 响应

- [ ] **步骤 5：Commit**

```bash
git add .env.example && git commit -m "chore: add env example and verify integration"
```

---

### 任务 14：最终验收

- [ ] **步骤 1：运行全部测试**

```bash
go test ./... -v
```
预期：全部 PASS（MySQL 相关测试在无 MySQL 时 SKIP）

- [ ] **步骤 2：检查代码覆盖率**

```bash
go test ./... -cover
```

- [ ] **步骤 3：确认所有文件已提交**

```bash
git status
```
预期：clean working tree

- [ ] **步骤 4：最终 Commit**

```bash
git add -A && git commit -m "chore: final verification — all tests pass, integration verified"
```

---

## 计划总结

| 任务 | 文件数 | 预估时间 |
|------|--------|----------|
| 1. 项目初始化 | 3 | 10 min |
| 2. 数据模型 | 2 | 15 min |
| 3. Repository | 2 | 20 min |
| 4. Adapter 接口 | 3 | 15 min |
| 5. RateLimiter | 2 | 15 min |
| 6. RetryManager | 2 | 15 min |
| 7. Engine 核心 | 3 | 20 min |
| 8. stats_gov Adapter | 2 | 15 min |
| 9. hangye_paihang Adapter | 2 | 15 min |
| 10. Scheduler | 1 | 10 min |
| 11. API Handlers | 1 | 20 min |
| 12. Router + main.go | 2 | 15 min |
| 13. 集成验证 | 1 | 20 min |
| 14. 最终验收 | 0 | 10 min |
| **合计** | **26+ 文件** | **~3.5 小时** |
