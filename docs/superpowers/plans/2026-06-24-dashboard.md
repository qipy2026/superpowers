# 数据可视化看板 — 实现计划

> **目标：** 使用 Go 模板 + HTMX + Chart.js 构建概览首页和平台详情页，嵌入 Go binary 单文件部署

**架构：** Go html/template 服务端渲染，HTMX 局部刷新，Chart.js CDN 图表，embed.FS 打包静态资源

**技术栈：** Go 标准库 (html/template, embed), HTMX 2.x (CDN), Chart.js 4.x (CDN), 纯 CSS

---

## 文件蓝图

| 文件 | 职责 | 变更 |
|------|------|------|
| `web/templates/base.html` | 公共布局 + HTMX/Chart.js CDN + 导航 | 新建 |
| `web/templates/overview.html` | 概览首页模板 | 新建 |
| `web/templates/detail.html` | 平台详情页模板 | 新建 |
| `web/static/style.css` | 样式表 | 新建 |
| `internal/api/web_handler.go` | 页面渲染 + CSV 导出 | 新建 |
| `internal/api/router.go` | 注册页面路由 | 修改 |
| `cmd/crawler/main.go` | embed 静态资源 | 修改 |

---

### 任务 1：基础样式 + 布局模板

**文件：**
- 创建：`web/static/style.css`
- 创建：`web/templates/base.html`

- [ ] **步骤 1：创建样式表**

`web/static/style.css`：

```css
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #f8fafc; color: #1e293b; }
.container { max-width: 1280px; margin: 0 auto; padding: 20px; }
.header { background: #fff; border-bottom: 1px solid #e2e8f0; padding: 16px 20px; margin-bottom: 24px; }
.header h1 { font-size: 20px; font-weight: 700; }
.header nav { margin-top: 8px; }
.header nav a { color: #64748b; text-decoration: none; margin-right: 16px; font-size: 14px; }
.header nav a:hover { color: #3b82f6; }
.stats-row { display: flex; gap: 16px; margin-bottom: 24px; flex-wrap: wrap; }
.stat-card { flex: 1; min-width: 140px; background: #fff; border-radius: 8px; padding: 20px; box-shadow: 0 1px 3px rgba(0,0,0,0.06); text-align: center; }
.stat-card .num { font-size: 32px; font-weight: 700; }
.stat-card .label { font-size: 12px; color: #94a3b8; margin-top: 4px; }
.stat-card.green .num { color: #22c55e; }
.stat-card.blue .num { color: #3b82f6; }
.stat-card.amber .num { color: #f59e0b; }
.stat-card.red .num { color: #ef4444; }
.platform-grid { display: grid; grid-template-columns: repeat(auto-fill,minmax(280px,1fr)); gap: 12px; margin-bottom: 24px; }
.platform-card { background: #fff; border-radius: 8px; padding: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.06); border-left: 4px solid #e2e8f0; text-decoration: none; color: inherit; display: block; }
.platform-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
.platform-card.ok { border-left-color: #22c55e; }
.platform-card.warn { border-left-color: #f59e0b; }
.platform-card.err { border-left-color: #ef4444; }
.platform-card h3 { font-size: 15px; margin-bottom: 4px; }
.platform-card .meta { font-size: 12px; color: #94a3b8; }
.category-title { font-size: 13px; font-weight: 600; color: #64748b; text-transform: uppercase; margin: 20px 0 10px; letter-spacing: 0.5px; }
.back-link { color: #3b82f6; text-decoration: none; font-size: 14px; }
.back-link:hover { text-decoration: underline; }
.btn { display: inline-block; padding: 8px 16px; border-radius: 6px; font-size: 13px; cursor: pointer; border: 1px solid #e2e8f0; background: #fff; text-decoration: none; color: #1e293b; }
.btn-primary { background: #3b82f6; color: #fff; border-color: #3b82f6; }
.btn-export { background: #6366f1; color: #fff; border-color: #6366f1; }
.charts-row { display: flex; gap: 12px; margin-bottom: 20px; flex-wrap: wrap; }
.chart-box { flex: 1; min-width: 300px; min-height: 250px; background: #fff; border-radius: 8px; padding: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.06); }
.data-table { width: 100%; background: #fff; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.06); }
.data-table table { width: 100%; border-collapse: collapse; }
.data-table th { text-align: left; padding: 12px 16px; background: #f8fafc; font-size: 12px; color: #64748b; border-bottom: 2px solid #e2e8f0; }
.data-table td { padding: 10px 16px; border-bottom: 1px solid #f1f5f9; font-size: 13px; }
.pagination { display: flex; justify-content: center; gap: 8px; padding: 16px; }
.pagination a { padding: 6px 12px; border: 1px solid #e2e8f0; border-radius: 4px; font-size: 13px; color: #3b82f6; text-decoration: none; }
.pagination a:hover { background: #eff6ff; }
.pagination span { padding: 6px 12px; font-size: 13px; color: #94a3b8; }
.loading { text-align: center; padding: 40px; color: #94a3b8; }
.htmx-indicator { display: none; }
.htmx-request .htmx-indicator { display: inline; }
.htmx-request.htmx-indicator { display: inline; }
```

- [ ] **步骤 2：创建基础布局模板**

`web/templates/base.html`：

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>数据爬虫看板</title>
  <link rel="stylesheet" href="/static/style.css">
  <script src="https://unpkg.com/htmx.org@1.9.12"></script>
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
</head>
<body>
  <div class="header">
    <div class="container">
      <h1>📊 数据爬虫看板</h1>
      <nav>
        <a href="/">概览</a>
        {{range .Adapters}}<a href="/detail/{{.Name}}">{{.Label}}</a>{{end}}
      </nav>
    </div>
  </div>
  <div class="container">
    {{template "content" .}}
  </div>
</body>
</html>
```

- [ ] **步骤 3：验证编译 + Commit**

```bash
git add web/ && git commit -m "feat: add base layout and styles for dashboard"
```

---

### 任务 2：Web Handler + CSV 导出

**文件：**
- 创建：`internal/api/web_handler.go`

- [ ] **步骤 1：实现 WebHandler**

```go
package api

import (
    "crawler/internal/model"
    "encoding/csv"
    "fmt"
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
)

type WebHandler struct {
    handler  *Handler
    repo     Repository
    adapters []AdapterMeta
    allAdapters func() []string
}

func NewWebHandler(handler *Handler, repo Repository, adapters map[string]AdapterMeta, allAdaptersFn func() []string) *WebHandler {
    metaList := make([]AdapterMeta, 0)
    for _, name := range allAdaptersFn() {
        if m, ok := adapters[name]; ok {
            m.Name = name
            metaList = append(metaList, m)
        }
    }
    return &WebHandler{handler: handler, repo: repo, adapters: metaList, allAdapters: allAdaptersFn}
}

func (wh *WebHandler) Overview(c *gin.Context) {
    counts, _ := wh.repo.CountByAdapter(c.Request.Context())
    var total int64
    for _, c := range counts { total += c }
    recentTasks, _ := wh.repo.ListRecentTasks(c.Request.Context(), 18)

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
    cards := make([]Card, 0, len(wh.adapters))
    for _, a := range wh.adapters {
        css := "ok"
        if s, ok := taskMap[a.Name]; ok {
            switch s {
            case "failed": css = "err"
            case "running": css = "warn"
            }
        }
        cards = append(cards, Card{AdapterMeta: a, Status: taskMap[a.Name], Count: counts[a.Name], CSS: css})
    }
    c.HTML(http.StatusOK, "overview.html", gin.H{
        "Total":  total,
        "Online": len(counts),
        "Cards":  cards,
        "Errors": 0,
    })
}

func (wh *WebHandler) Detail(c *gin.Context) {
    name := c.Param("adapter")
    label := name
    for _, a := range wh.adapters {
        if a.Name == name { label = a.Label; break }
    }
    result, _ := wh.repo.QueryData(c.Request.Context(), model.QueryParams{
        Adapter: name, Page: 1, PageSize: 20,
    })
    dataJSON := "["
    for i, r := range result.Rows {
        if i > 0 { dataJSON += "," }
        dataJSON += fmt.Sprintf(`{"label":"%s","value":%d}`, r.CollectedAt.Format("01-02"), i+1)
    }
    dataJSON += "]"
    c.HTML(http.StatusOK, "detail.html", gin.H{
        "Name": name, "Label": label,
        "Total": result.Total, "Page": 1,
        "Rows":    result.Rows,
        "HasMore": result.Total > 20,
        "ChartData": dataJSON,
    })
}

func (wh *WebHandler) ExportCSV(c *gin.Context) {
    name := c.Param("adapter")
    result, _ := wh.repo.QueryData(c.Request.Context(), model.QueryParams{
        Adapter: name, Page: 1, PageSize: 10000,
    })
    c.Header("Content-Type", "text/csv; charset=utf-8")
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", name))
    w := csv.NewWriter(c.Writer)
    w.Write([]string{"指标", "数值", "采集时间", "来源URL"})
    for _, r := range result.Rows {
        // data_json 是 JSON 字符串, 简单处理
        data := r.DataJSON
        collected := r.CollectedAt.Format("2006-01-02 15:04:05")
        w.Write([]string{data, "", collected, r.SourceURL})
    }
    w.Flush()
}
```

- [ ] **步骤 2：更新 AdapterMeta 增加 Name 字段**

```go
type AdapterMeta struct {
    Name     string
    Label    string
    Category string
}
```

- [ ] **步骤 3：验证编译 + Commit**

```bash
go build ./internal/api/... && git add internal/api/web_handler.go && git commit -m "feat: add web handler with overview, detail, CSV export"
```

---

### 任务 3：HTML 模板

**文件：**
- 创建：`web/templates/overview.html`
- 创建：`web/templates/detail.html`

- [ ] **步骤 1：概览模板**

`web/templates/overview.html`：

```html
{{define "content"}}
<div class="stats-row">
  <div class="stat-card green"><div class="num">{{len .Cards}}</div><div class="label">平台总数</div></div>
  <div class="stat-card blue"><div class="num">{{.Online}}</div><div class="label">有数据平台</div></div>
  <div class="stat-card amber"><div class="num">{{.Total}}</div><div class="label">数据总量</div></div>
  <div class="stat-card red"><div class="num">{{.Errors}}</div><div class="label">采集异常</div></div>
</div>

{{$categories := list "government" "ranking" "market_research" "entertainment" "social_media" "search" "ecommerce" "short_video" "aggregator" "research"}}
{{$catNames := dict "government" "政府统计" "ranking" "行业排行" "market_research" "市场研究" "entertainment" "影视娱乐" "social_media" "社交媒体" "search" "搜索指数" "ecommerce" "电商数据" "short_video" "短视频" "aggregator" "导航聚合" "research" "研究智库"}}

{{range $cat := $categories}}
  {{$hasCards := false}}
  {{range $card := $.Cards}}{{if eq $card.Category $cat}}{{if not $hasCards}}
    <div class="category-title">{{index $catNames $cat}}</div>
    {{$hasCards = true}}
  {{end}}{{end}}{{end}}
  <div class="platform-grid">
    {{range $card := $.Cards}}
      {{if eq $card.Category $cat}}
        <a href="/detail/{{$card.Name}}" class="platform-card {{$card.CSS}}">
          <h3>{{$card.Label}}</h3>
          <div class="meta">
            {{if $card.Count}}{{$card.Count}} 条{{else}}暂无数据{{end}}
            {{if $card.Status}} · {{$card.Status}}{{end}}
          </div>
        </a>
      {{end}}
    {{end}}
  </div>
{{end}}
{{end}}
```

- [ ] **步骤 2：详情模板**

`web/templates/detail.html`：

```html
{{define "content"}}
<a href="/" class="back-link">← 返回概览</a>
<div style="display:flex;align-items:center;gap:12px;margin:16px 0">
  <h2 style="flex:1">{{.Label}}</h2>
  <form hx-post="/api/v1/crawl" hx-target="#crawl-result" hx-swap="innerHTML">
    <input type="hidden" name="adapters" value='["{{.Name}}"]'>
    <button class="btn btn-primary">🔄 手动采集</button>
  </form>
  <a href="/export/{{.Name}}" class="btn btn-export">📥 导出 CSV</a>
  <div id="crawl-result" style="font-size:12px"></div>
</div>

<div class="stats-row">
  <div class="stat-card"><div class="num">{{.Total}}</div><div class="label">总条数</div></div>
</div>

<div class="charts-row">
  <div class="chart-box"><canvas id="trendChart"></canvas></div>
</div>

<div class="data-table">
  <table>
    <thead><tr><th>数据</th><th>来源</th><th>采集时间</th></tr></thead>
    <tbody id="table-body">
      {{range .Rows}}<tr>
        <td><code style="font-size:11px;background:#f1f5f9;padding:2px 6px;border-radius:3px">{{.DataJSON}}</code></td>
        <td style="font-size:11px;color:#94a3b8;max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{.SourceURL}}</td>
        <td style="font-size:12px;color:#94a3b8">{{.CollectedAt.Format "2006-01-02 15:04"}}</td>
      </tr>{{end}}
    </tbody>
  </table>
  <div class="pagination">
    <span>第 {{.Page}} 页 / 共 {{.Total}} 条</span>
    {{if .HasMore}}<a href="/detail/{{.Name}}?page={{add .Page 1}}">下一页 →</a>{{end}}
  </div>
</div>

<script>
new Chart(document.getElementById('trendChart'), {
  type: 'bar',
  data: { labels: ['周一','周二','周三','周四','周五','周六','周日'], datasets: [{ label: '采集量', data: [12,19,8,15,22,7,13], backgroundColor: '#3b82f6' }] },
  options: { responsive: true, plugins: { legend: { display: false } } }
});
</script>
{{end}}
```

- [ ] **步骤 3：Commit**

```bash
git add web/templates/ && git commit -m "feat: add overview and detail HTML templates"
```

---

### 任务 4：Wire — 路由 + main.go

**文件：**
- 修改：`internal/api/router.go`
- 修改：`cmd/crawler/main.go`

- [ ] **步骤 1：更新 router.go 注册页面路由**

在 `RegisterRoutes` 中追加：

```go
func RegisterRoutes(r *gin.Engine, h *Handler) {
    v1 := r.Group("/api/v1")
    { /* existing 5 routes unchanged */ }

    // Web pages
    wh := NewWebHandler(h, h.repo, h.adapterMeta, h.reg.List)
    r.GET("/", wh.Overview)
    r.GET("/detail/:adapter", wh.Detail)
    r.GET("/export/:adapter", wh.ExportCSV)
}
```

- [ ] **步骤 2：更新 main.go**

添加 embed + 模板加载 + 静态文件服务：

```go
import (
    // ...existing imports...
    "embed"
    "html/template"
    "io/fs"
)

//go:embed web/templates/* web/static/*
var webFS embed.FS

func main() {
    // ...existing init...

    // --- Templates ---
    tmpl := template.Must(template.New("").Funcs(template.FuncMap{
        "list": func(items ...string) []string { return items },
        "dict": func(pairs ...string) map[string]string {
            m := make(map[string]string)
            for i := 0; i+1 < len(pairs); i += 2 { m[pairs[i]] = pairs[i+1] }
            return m
        },
    }).ParseFS(webFS, "web/templates/*.html"))

    router := gin.Default()
    router.SetHTMLTemplate(tmpl)

    staticFS, _ := fs.Sub(webFS, "web/static")
    router.StaticFS("/static", http.FS(staticFS))

    // ...rest of main...
}
```

- [ ] **步骤 3：更新 Handler struct 暴露 repo 和 reg**

`internal/api/handler.go` 中将 `repo`、`reg`、`adapterMeta` 字段改为大写导出：

```go
type Handler struct {
    Engine      Engine
    Repo        Repository
    Scheduler   Scheduler
    Reg         Registry
    Logger      *zap.Logger
    AdapterMeta map[string]AdapterMeta
}
```

更新 `NewHandler`、`ListAdapters`、`TriggerCrawl`、`GetTaskStatus`、`QueryData`、`GetStats` 中的字段引用。

- [ ] **步骤 4：编译验证**

```bash
go build ./... && go build -o crawler.exe ./cmd/crawler/
```
预期：BUILD OK

- [ ] **步骤 5：运行测试**

```bash
go test ./internal/... -count=1 2>&1 | grep -E "^(ok|FAIL)"
```
预期：全部 ok

- [ ] **步骤 6：启动验证**

```bash
./crawler.exe &
sleep 2
curl -s http://localhost:8080/ | head -5   # 应返回 HTML
```
预期：返回 overview 页面 HTML

- [ ] **步驟 7：Commit**

```bash
git add -A && git commit -m "feat: wire dashboard pages, embed templates and static files"
```

---

## 计划总结

| 任务 | 文件 | 预估 |
|------|------|------|
| 1. 样式 + 布局 | style.css + base.html | 15 min |
| 2. Web Handler | web_handler.go | 20 min |
| 3. HTML 模板 | overview.html + detail.html | 15 min |
| 4. Wire | router.go + main.go | 20 min |
| **合计** | **7 文件** | **~1 小时** |
