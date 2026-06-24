# 网络爬虫 Phase 2 — 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 接入 Rod 浏览器引擎支持 JS 动态渲染，激活代理池，新增 5 个中等复杂度平台适配器

**架构：** 在现有 Engine + Colly 架构上增加 Rod BrowserPool，适配器按需选择 Colly 或 Rod；ProxyPool 从空实现升级为轮转代理

**技术栈：** go-rod/rod, 沿用 Phase 1 全部依赖

---

## 文件蓝图

| 文件 | 职责 | 变更类型 |
|------|------|---------|
| `internal/engine/rod_pool.go` | Rod 浏览器池管理（启动/借用/归还/关闭） | 新建 |
| `internal/engine/rod_pool_test.go` | Rod 池测试 | 新建 |
| `internal/engine/proxy.go` | 从空实现升级为可用的代理轮转池 | 修改 |
| `internal/engine/proxy_test.go` | 代理池测试 | 新建 |
| `internal/adapter/iresearch/adapter.go` | 艾瑞数据适配器（Colly + Rod） | 新建 |
| `internal/adapter/guduo/adapter.go` | 骨朵数据适配器（Rod） | 新建 |
| `internal/adapter/maoyan/adapter.go` | 猫眼票房适配器（Rod） | 新建 |
| `internal/adapter/penguin_intelligence/adapter.go` | 企鹅智库适配器（Colly） | 新建 |
| `internal/adapter/tencent_research/adapter.go` | 腾讯研究院适配器（Colly） | 新建 |
| `cmd/crawler/main.go` | 注册新适配器，加载代理配置 | 修改 |

---

### 任务 1：Rod 浏览器池

**文件：**
- 创建：`internal/engine/rod_pool.go`
- 创建：`internal/engine/rod_pool_test.go`

Rod 为每个 adapter 提供无头浏览器实例，自动管理启动/销毁生命周期。

- [ ] **步骤 1：编写浏览器池测试**

创建 `internal/engine/rod_pool_test.go`：

```go
package engine

import (
    "testing"
    "time"
)

func TestRodPoolBorrowReturn(t *testing.T) {
    pool := NewRodPool(2) // max 2 browsers
    if err := pool.Start(); err != nil {
        t.Skipf("browser not available: %v", err)
    }
    defer pool.Close()

    b1, err := pool.Borrow()
    if err != nil {
        t.Fatalf("Borrow failed: %v", err)
    }
    if b1 == nil {
        t.Fatal("expected non-nil browser")
    }
    pool.Return(b1)

    b2, err := pool.Borrow()
    if err != nil {
        t.Fatalf("second Borrow failed: %v", err)
    }
    pool.Return(b2)
}

func TestRodPoolExhausted(t *testing.T) {
    pool := NewRodPool(1)
    if err := pool.Start(); err != nil {
        t.Skipf("browser not available: %v", err)
    }
    defer pool.Close()

    b1, _ := pool.Borrow()
    // second borrow with 100ms timeout should fail
    b2, err := pool.BorrowTimeout(100 * time.Millisecond)
    if err == nil {
        pool.Return(b2)
        t.Error("expected timeout error when pool exhausted")
    }
    pool.Return(b1)
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
export GOPROXY="https://goproxy.cn,direct" && export PATH="/c/Go/bin:$PATH" && cd e:/work/code/superpowers && go test ./internal/engine/... -v -run TestRod
```
预期：FAIL — NewRodPool not defined

- [ ] **步骤 3：安装 Rod 依赖**

```bash
export GOPROXY="https://goproxy.cn,direct" && export PATH="/c/Go/bin:$PATH" && cd e:/work/code/superpowers && go get github.com/go-rod/rod && go mod tidy
```

- [ ] **步骤 4：编写浏览器池实现**

创建 `internal/engine/rod_pool.go`：

```go
package engine

import (
    "fmt"
    "sync"
    "time"

    "github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/launcher"
)

type RodPool struct {
    maxSize   int
    pool      chan *rod.Browser
    launcher  string // browser launcher path
    started   bool
    mu        sync.Mutex
}

func NewRodPool(maxSize int) *RodPool {
    return &RodPool{
        maxSize: maxSize,
        pool:    make(chan *rod.Browser, maxSize),
    }
}

func (p *RodPool) Start() error {
    p.mu.Lock()
    defer p.mu.Unlock()
    if p.started {
        return nil
    }
    path, found := launcher.LookPath()
    if !found {
        return fmt.Errorf("no browser found for Rod")
    }
    p.launcher = path
    // pre-warm pool
    for i := 0; i < p.maxSize; i++ {
        b, err := p.newBrowser()
        if err != nil {
            return fmt.Errorf("pre-warm browser %d: %w", i, err)
        }
        p.pool <- b
    }
    p.started = true
    return nil
}

func (p *RodPool) newBrowser() (*rod.Browser, error) {
    u := launcher.New().Bin(p.launcher).Headless(true).MustLaunch()
    b := rod.New().ControlURL(u).MustConnect()
    return b, nil
}

func (p *RodPool) Borrow() (*rod.Browser, error) {
    select {
    case b := <-p.pool:
        return b, nil
    default:
        // pool empty, create new up to limit
        b, err := p.newBrowser()
        if err != nil {
            return nil, err
        }
        return b, nil
    }
}

func (p *RodPool) BorrowTimeout(d time.Duration) (*rod.Browser, error) {
    select {
    case b := <-p.pool:
        return b, nil
    case <-time.After(d):
        return nil, fmt.Errorf("borrow timeout after %v", d)
    }
}

func (p *RodPool) Return(b *rod.Browser) {
    if b == nil {
        return
    }
    select {
    case p.pool <- b:
    default:
        // pool full, close this one
        b.Close()
    }
}

func (p *RodPool) Close() {
    p.mu.Lock()
    defer p.mu.Unlock()
    close(p.pool)
    for b := range p.pool {
        b.Close()
    }
    p.started = false
}

type RodPage struct {
    browser *rod.Browser
    pool    *RodPool
    page    *rod.Page
}

// NewRodPage borrows a browser and navigates to a URL, returning a helper.
// Call Close() when done to return the browser to the pool.
func (p *RodPool) NewRodPage(url string, timeout time.Duration) (*RodPage, error) {
    b, err := p.BorrowTimeout(timeout)
    if err != nil {
        return nil, err
    }
    page := b.MustPage(url)
    page.Timeout(timeout)
    return &RodPage{browser: b, pool: p, page: page}, nil
}

func (rp *RodPage) Page() *rod.Page { return rp.page }

func (rp *RodPage) Close() {
    if rp.page != nil {
        rp.page.Close()
    }
    rp.pool.Return(rp.browser)
}
```

- [ ] **步骤 5：运行测试验证通过**

```bash
export GOPROXY="https://goproxy.cn,direct" && export PATH="/c/Go/bin:$PATH" && cd e:/work/code/superpowers && go test ./internal/engine/... -v -run TestRod
```
预期：PASS（若浏览器不可用则 SKIP）

- [ ] **步骤 6：Commit**

```bash
git add internal/engine/rod_pool.go internal/engine/rod_pool_test.go go.mod go.sum && \
  git commit -m "feat: add Rod browser pool for JS-rendered page scraping"
```

---

### 任务 2：代理池激活

**文件：**
- 修改：`internal/engine/proxy.go`
- 创建：`internal/engine/proxy_test.go`

- [ ] **步骤 1：编写代理池测试**

创建 `internal/engine/proxy_test.go`：

```go
package engine

import (
    "testing"
)

func TestProxyPoolRotation(t *testing.T) {
    proxies := []string{
        "http://proxy1:8080",
        "http://proxy2:8080",
        "http://proxy3:8080",
    }
    pool := NewProxyPool(proxies)
    u1, err := pool.Next()
    if err != nil {
        t.Fatalf("Next failed: %v", err)
    }
    if u1.Host == "" {
        t.Error("expected a proxy URL")
    }
    // should rotate through all
    hosts := make(map[string]int)
    for i := 0; i < 6; i++ {
        u, _ := pool.Next()
        hosts[u.Host]++
    }
    if len(hosts) != 3 {
        t.Errorf("expected 3 distinct hosts, got %d", len(hosts))
    }
}

func TestProxyPoolEmptyConfig(t *testing.T) {
    pool := NewProxyPool(nil)
    u, err := pool.Next()
    if err != nil {
        t.Fatalf("Next failed: %v", err)
    }
    if u != nil {
        t.Error("expected nil on empty pool")
    }
}

func TestProxyPoolMarkBad(t *testing.T) {
    proxies := []string{"http://proxy1:8080", "http://proxy2:8080"}
    pool := NewProxyPool(proxies)
    u, _ := pool.Next()
    pool.MarkBad(u)
    // marked proxy should be removed
    for i := 0; i < 4; i++ {
        u2, _ := pool.Next()
        if u2.Host == u.Host {
            t.Error("marked-bad proxy should not appear again")
        }
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/engine/... -v -run TestProxy
```
预期：FAIL — NewProxyPool signature changed

- [ ] **步骤 3：重写 ProxyPool**

重写 `internal/engine/proxy.go`：

```go
package engine

import (
    "net/url"
    "sync"
    "sync/atomic"
)

type ProxyPool struct {
    proxies []*url.URL
    cursor  atomic.Uint32
    mu      sync.RWMutex
}

func NewProxyPool(proxyURLs []string) *ProxyPool {
    p := &ProxyPool{}
    for _, raw := range proxyURLs {
        if u, err := url.Parse(raw); err == nil {
            p.proxies = append(p.proxies, u)
        }
    }
    return p
}

func (p *ProxyPool) Next() (*url.URL, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    if len(p.proxies) == 0 {
        return nil, nil // Phase 1 behavior: no proxy
    }
    idx := p.cursor.Add(1) % uint32(len(p.proxies))
    return p.proxies[idx], nil
}

func (p *ProxyPool) MarkBad(u *url.URL) {
    p.mu.Lock()
    defer p.mu.Unlock()
    for i, proxy := range p.proxies {
        if proxy.Host == u.Host {
            p.proxies = append(p.proxies[:i], p.proxies[i+1:]...)
            return
        }
    }
}

func (p *ProxyPool) Size() int {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return len(p.proxies)
}
```

- [ ] **步骤 4：检查 main.go 是否需要适配**

`cmd/crawler/main.go` 中 `NewProxyPool()` 调用需更新为 `engine.NewProxyPool(nil)`（Phase 2 先从配置读取代理列表，暂传 nil）。

```bash
grep "NewProxyPool" cmd/crawler/main.go
```
如果未使用，无需修改。

- [ ] **步骤 5：运行测试验证通过**

```bash
go test ./internal/engine/... -v -run TestProxy
```
预期：PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/engine/proxy.go internal/engine/proxy_test.go && \
  git commit -m "feat: activate proxy pool with rotation and bad-proxy removal"
```

---

### 任务 3：艾瑞数据 Adapter

**文件：**
- 创建：`internal/adapter/iresearch/adapter.go`
- 创建：`internal/adapter/iresearch/adapter_test.go`

艾瑞数据 (iResearch) — 市场研究数据，部分页面需 JS 渲染。

- [ ] **步骤 1：编写测试**

创建 `internal/adapter/iresearch/adapter_test.go`：

```go
package iresearch

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "crawler/internal/adapter"
)

func TestIResearchCollect(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`
<html><body>
  <div class="report-list">
    <div class="report-item">
      <h3 class="title">2024年中国移动互联网报告</h3>
      <span class="date">2024-06-15</span>
      <span class="category">移动互联网</span>
    </div>
    <div class="report-item">
      <h3 class="title">2024年电商行业洞察</h3>
      <span class="date">2024-05-20</span>
      <span class="category">电子商务</span>
    </div>
  </div>
</body></html>`))
    }))
    defer srv.Close()
    a := New(srv.URL, nil) // nil RodPool -> Colly only
    task := &adapter.Task{ID: "t1", Adapter: "iresearch"}
    rows, err := a.Collect(context.Background(), task)
    if err != nil {
        t.Fatalf("Collect failed: %v", err)
    }
    if len(rows) == 0 {
        t.Fatal("expected report data")
    }
    t.Logf("collected %d reports", len(rows))
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/adapter/iresearch/... -v
```
预期：FAIL

- [ ] **步骤 3：实现 Adapter**

创建 `internal/adapter/iresearch/adapter.go`：

```go
package iresearch

import (
    "context"
    "crawler/internal/adapter"
    "crawler/internal/engine"
    "fmt"
    "net/url"
    "strings"

    "github.com/gocolly/colly/v2"
    "github.com/go-rod/rod"
)

type Adapter struct {
    baseURL  string
    rodPool  *engine.RodPool
}

func New(baseURL string, rodPool *engine.RodPool) *Adapter {
    return &Adapter{baseURL: baseURL, rodPool: rodPool}
}

func (a *Adapter) Name() string { return "iresearch" }

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
    u, err := url.Parse(a.baseURL)
    if err != nil {
        return nil, fmt.Errorf("parse URL: %w", err)
    }

    // Try Colly first for static content
    c := colly.NewCollector(colly.AllowedDomains(u.Hostname()))

    c.OnHTML(".report-item, .article-item, .list-item", func(e *colly.HTMLElement) {
        title := strings.TrimSpace(e.ChildText(".title, h3, h2, a"))
        date := strings.TrimSpace(e.ChildText(".date, .time, span:last-child"))
        category := strings.TrimSpace(e.ChildText(".category, .tag"))
        if title != "" {
            rows = append(rows, adapter.DataRow{
                SourceURL: e.Request.URL.String(),
                Data: map[string]string{
                    "title":    title,
                    "date":     date,
                    "category": category,
                },
            })
        }
    })

    c.OnHTML("a.next, a:contains(下一页)", func(e *colly.HTMLElement) {
        e.Request.Visit(e.Attr("href"))
    })

    if err := c.Visit(a.baseURL); err != nil {
        return nil, fmt.Errorf("colly visit: %w", err)
    }
    c.Wait()

    // If Colly got nothing, try Rod for JS-rendered content
    if len(rows) == 0 && a.rodPool != nil {
        rp, err := a.rodPool.NewRodPage(a.baseURL, 30_000_000_000) // 30s
        if err != nil {
            return nil, fmt.Errorf("rod page: %w", err)
        }
        defer rp.Close()

        rp.Page().MustWaitStable()
        els, _ := rp.Page().Elements(".report-item, .article-item, .list-item")
        for _, el := range els {
            title, _ := el.Element(".title")
            date, _ := el.Element(".date, .time")
            titleText := ""
            dateText := ""
            if title != nil {
                titleText = strings.TrimSpace(title.MustText())
            }
            if date != nil {
                dateText = strings.TrimSpace(date.MustText())
            }
            if titleText != "" {
                rows = append(rows, adapter.DataRow{
                    SourceURL: a.baseURL,
                    Data: map[string]string{
                        "title": titleText,
                        "date":  dateText,
                    },
                })
            }
        }
    }

    return rows, nil
}

// Ensure rod is used, avoid unused import
var _ = rod.Try
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/adapter/iresearch/... -v
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/adapter/iresearch/ && \
  git commit -m "feat: add iresearch adapter with Colly+Rod fallback"
```

---

### 任务 4：骨朵数据 Adapter

**文件：**
- 创建：`internal/adapter/guduo/adapter.go`
- 创建：`internal/adapter/guduo/adapter_test.go`

骨朵数据 — 影视/综艺数据，大量 JS 渲染，主要用 Rod。

- [ ] **步骤 1：编写测试**

创建 `internal/adapter/guduo/adapter_test.go`：

```go
package guduo

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "crawler/internal/adapter"
)

func TestGuduoCollect(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`
<html><body>
  <div class="drama-list">
    <div class="drama-card">
      <span class="drama-name">庆余年2</span>
      <span class="heat-index">9876</span>
      <span class="platform">腾讯视频</span>
    </div>
    <div class="drama-card">
      <span class="drama-name">狐妖小红娘</span>
      <span class="heat-index">8765</span>
      <span class="platform">爱奇艺</span>
    </div>
  </div>
</body></html>`))
    }))
    defer srv.Close()
    a := New(srv.URL, nil)
    task := &adapter.Task{ID: "t2", Adapter: "guduo"}
    rows, err := a.Collect(context.Background(), task)
    if err != nil {
        t.Fatalf("Collect failed: %v", err)
    }
    if len(rows) == 0 {
        t.Fatal("expected drama data")
    }
    t.Logf("collected %d dramas", len(rows))
}
```

- [ ] **步骤 2：运行测试验证失败** → **步骤 3：实现** → **步骤 4：验证** → **步骤 5：Commit**

实现 `internal/adapter/guduo/adapter.go`：

```go
package guduo

import (
    "context"
    "crawler/internal/adapter"
    "crawler/internal/engine"
    "fmt"
    "net/url"
    "strings"

    "github.com/gocolly/colly/v2"
)

type Adapter struct {
    baseURL string
    rodPool *engine.RodPool
}

func New(baseURL string, rodPool *engine.RodPool) *Adapter {
    return &Adapter{baseURL: baseURL, rodPool: rodPool}
}

func (a *Adapter) Name() string { return "guduo" }

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
    u, err := url.Parse(a.baseURL)
    if err != nil {
        return nil, fmt.Errorf("parse URL: %w", err)
    }
    c := colly.NewCollector(colly.AllowedDomains(u.Hostname()))

    c.OnHTML(".drama-card, .variety-card, .show-item", func(e *colly.HTMLElement) {
        name := strings.TrimSpace(e.ChildText(".drama-name, .show-name, .title"))
        heat := strings.TrimSpace(e.ChildText(".heat-index, .hot-value, .score"))
        platform := strings.TrimSpace(e.ChildText(".platform, .source"))
        if name != "" {
            rows = append(rows, adapter.DataRow{
                SourceURL: e.Request.URL.String(),
                Data: map[string]string{
                    "name":     name,
                    "heat":     heat,
                    "platform": platform,
                },
            })
        }
    })

    c.OnHTML("a.next, a:contains(下一页)", func(e *colly.HTMLElement) {
        e.Request.Visit(e.Attr("href"))
    })

    // Rod fallback for JS-rendered cards
    if a.rodPool != nil {
        c.OnHTML("script", func(e *colly.HTMLElement) {
            // trigger Rod for dynamic content detection
        })
    }

    if err := c.Visit(a.baseURL); err != nil {
        return nil, fmt.Errorf("visit: %w", err)
    }
    c.Wait()

    // Rod fallback
    if len(rows) == 0 && a.rodPool != nil {
        rp, err := a.rodPool.NewRodPage(a.baseURL, 30_000_000_000)
        if err != nil {
            return nil, fmt.Errorf("rod: %w", err)
        }
        defer rp.Close()
        rp.Page().MustWaitStable()
        els, _ := rp.Page().Elements(".drama-card, .variety-card, .show-item")
        for _, el := range els {
            name := strings.TrimSpace(el.MustText())
            if name != "" {
                rows = append(rows, adapter.DataRow{
                    SourceURL: a.baseURL,
                    Data:      map[string]string{"name": name},
                })
            }
        }
    }

    return rows, nil
}
```

Commit: `feat: add guduo adapter for entertainment data`

---

### 任务 5：猫眼票房 Adapter

**文件：**
- 创建：`internal/adapter/maoyan/adapter.go`
- 创建：`internal/adapter/maoyan/adapter_test.go`

猫眼票房 — 实时票房数据，JS 渲染重度，主要用 Rod。

- [ ] **步骤 1-5：同 Task 3/4 模式实现**

创建 `internal/adapter/maoyan/adapter.go`：

```go
package maoyan

import (
    "context"
    "crawler/internal/adapter"
    "crawler/internal/engine"
    "fmt"
    "strings"

    "github.com/gocolly/colly/v2"
)

type Adapter struct {
    baseURL string
    rodPool *engine.RodPool
}

func New(baseURL string, rodPool *engine.RodPool) *Adapter {
    return &Adapter{baseURL: baseURL, rodPool: rodPool}
}

func (a *Adapter) Name() string     { return "maoyan" }
func (a *Adapter) Validate() error  { c := colly.NewCollector(); c.Visit(a.baseURL); return nil }

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    if a.rodPool == nil {
        return nil, fmt.Errorf("maoyan requires Rod for JS rendering")
    }
    var rows []adapter.DataRow
    rp, err := a.rodPool.NewRodPage(a.baseURL, 30_000_000_000)
    if err != nil {
        return nil, fmt.Errorf("rod page: %w", err)
    }
    defer rp.Close()

    rp.Page().MustWaitStable()
    // wait for ticket data to load
    rp.Page().MustElement(".movie-box, .movie-item")
    els, _ := rp.Page().Elements(".movie-box, .movie-item, .detail-block")
    for _, el := range els {
        name := strings.TrimSpace(el.MustText())
        if name == "" {
            continue
        }
        // extract sub-fields if available
        boxOffice := ""
        if bo, err := el.Element(".box-office, .total-boxoffice, .revenue"); err == nil {
            boxOffice = strings.TrimSpace(bo.MustText())
        }
        rows = append(rows, adapter.DataRow{
            SourceURL: a.baseURL,
            Data: map[string]string{
                "name":       name,
                "box_office": boxOffice,
            },
        })
    }
    return rows, nil
}
```

测试同上模式（httptest mock HTML），Commit: `feat: add maoyan box office adapter with Rod`

---

### 任务 6：企鹅智库 Adapter

**文件：**
- 创建：`internal/adapter/penguin_intelligence/adapter.go`
- 创建：`internal/adapter/penguin_intelligence/adapter_test.go`

企鹅智库 — 研究报告列表，静态页面为主，Colly 足够。

- [ ] **步骤 1-5：同 Task 3 模式实现**

创建 `internal/adapter/penguin_intelligence/adapter.go`：

```go
package penguin_intelligence

import (
    "context"
    "crawler/internal/adapter"
    "fmt"
    "net/url"
    "strings"

    "github.com/gocolly/colly/v2"
)

type Adapter struct{ baseURL string }

func New(baseURL string) *Adapter { return &Adapter{baseURL: baseURL} }

func (a *Adapter) Name() string    { return "penguin_intelligence" }
func (a *Adapter) Validate() error { c := colly.NewCollector(); c.Visit(a.baseURL); return nil }

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    var rows []adapter.DataRow
    u, err := url.Parse(a.baseURL)
    if err != nil {
        return nil, fmt.Errorf("parse URL: %w", err)
    }
    c := colly.NewCollector(colly.AllowedDomains(u.Hostname()))

    c.OnHTML(".report-item, .article-item, .post-item", func(e *colly.HTMLElement) {
        title := strings.TrimSpace(e.ChildText("h3, h2, .title, a"))
        date := strings.TrimSpace(e.ChildText(".date, .time"))
        link := e.ChildAttr("a", "href")
        if title != "" {
            rows = append(rows, adapter.DataRow{
                SourceURL: e.Request.AbsoluteURL(link),
                Data: map[string]string{
                    "title": title,
                    "date":  date,
                    "link":  link,
                },
            })
        }
    })

    c.OnHTML("a.next, a:contains(下一页)", func(e *colly.HTMLElement) {
        e.Request.Visit(e.Attr("href"))
    })

    if err := c.Visit(a.baseURL); err != nil {
        return nil, fmt.Errorf("visit: %w", err)
    }
    c.Wait()
    return rows, nil
}
```

Commit: `feat: add penguin_intelligence adapter for research reports`

---

### 任务 7：腾讯研究院 Adapter

**文件：**
- 创建：`internal/adapter/tencent_research/adapter.go`
- 创建：`internal/adapter/tencent_research/adapter_test.go`

腾讯研究院 — 研究报告，静态页面，Colly 足够。

实现同企鹅智库模式（New/Name/Validate/Collect），调整 CSS 选择器为腾讯研究院网站结构。

Commit: `feat: add tencent_research adapter for research reports`

---

### 任务 8：Wire — main.go 集成 + 配置更新

**文件：**
- 修改：`cmd/crawler/main.go`
- 修改：`config/config.yaml`

- [ ] **步骤 1：更新配置文件**

在 `config/config.yaml` 的 `adapters` 列表追加：

```yaml
  - name: iresearch
    enabled: true
    cron: "0 7 * * *"
    rate_limit: 5
    mode: incremental
    base_url: "https://www.iresearch.cn/"
  - name: guduo
    enabled: true
    cron: "0 8 * * *"
    rate_limit: 5
    mode: incremental
    base_url: "https://www.guduo.com/"
  - name: maoyan
    enabled: true
    cron: "0 */2 * * *"
    rate_limit: 3
    mode: incremental
    base_url: "https://piaofang.maoyan.com/"
  - name: penguin_intelligence
    enabled: true
    cron: "0 9 * * 1"
    rate_limit: 10
    mode: incremental
    base_url: "https://re.qq.com/"
  - name: tencent_research
    enabled: true
    cron: "0 9 * * 1"
    rate_limit: 10
    mode: incremental
    base_url: "https://www.tisi.org/"
```

- [ ] **步骤 2：在 AdapterCfg 中增加 base_url**

修改 `cmd/crawler/main.go` 中的 `AdapterCfg`：

```go
type AdapterCfg struct {
    Name      string `mapstructure:"name"`
    Enabled   bool   `mapstructure:"enabled"`
    Cron      string `mapstructure:"cron"`
    RateLimit int    `mapstructure:"rate_limit"`
    Mode      string `mapstructure:"mode"`
    BaseURL   string `mapstructure:"base_url"`
}
```

- [ ] **步骤 3：初始化 RodPool + 注册新适配器**

在 `main()` 中的 adapter registry 部分追加：

```go
// --- Rod Pool ---
rodPool := engine.NewRodPool(2) // max 2 browser instances
if err := rodPool.Start(); err != nil {
    logger.Warn("Rod browser pool not available, some adapters may fail", zap.Error(err))
} else {
    defer rodPool.Close()
    logger.Info("Rod browser pool started", zap.Int("size", 2))
}

// --- Adapter Registry ---
reg := adapter.NewRegistry()
reg.Register(stats_gov.New("https://www.stats.gov.cn/sj/"))
reg.Register(hangye_paihang.New("https://www.example.com/industry-ranking"))
// Phase 2 adapters
reg.Register(iresearch.New(getBaseURL(cfg.Adapters, "iresearch"), rodPool))
reg.Register(guduo.New(getBaseURL(cfg.Adapters, "guduo"), rodPool))
reg.Register(maoyan.New(getBaseURL(cfg.Adapters, "maoyan"), rodPool))
reg.Register(penguin_intelligence.New(getBaseURL(cfg.Adapters, "penguin_intelligence")))
reg.Register(tencent_research.New(getBaseURL(cfg.Adapters, "tencent_research")))
```

添加辅助函数：

```go
func getBaseURL(adapters []AdapterCfg, name string) string {
    for _, a := range adapters {
        if a.Name == name && a.BaseURL != "" {
            return a.BaseURL
        }
    }
    return ""
}
```

更新 API handler 的 `adapterMeta`：

```go
adapterMeta: map[string]AdapterMeta{
    "stats_gov":              {Label: "国家统计局", Category: "government"},
    "hangye_paihang":         {Label: "行业排行榜", Category: "ranking"},
    "iresearch":              {Label: "艾瑞数据", Category: "market_research"},
    "guduo":                  {Label: "骨朵数据", Category: "entertainment"},
    "maoyan":                 {Label: "猫眼票房", Category: "entertainment"},
    "penguin_intelligence":   {Label: "企鹅智库", Category: "research"},
    "tencent_research":       {Label: "腾讯研究院", Category: "research"},
},
```

添加 import：
```go
"crawler/internal/adapter/iresearch"
"crawler/internal/adapter/guduo"
"crawler/internal/adapter/maoyan"
"crawler/internal/adapter/penguin_intelligence"
"crawler/internal/adapter/tencent_research"
```

- [ ] **步骤 4：验证编译**

```bash
export GOPROXY="https://goproxy.cn,direct" && export PATH="/c/Go/bin:$PATH" && cd e:/work/code/superpowers && go build ./cmd/crawler/
```
预期：成功

- [ ] **步驟 5：運行所有測試**

```bash
go test ./... -v -count=1
```
预期：Phase 2 相关 PASS，repository SKIP（无 MySQL）

- [ ] **步骤 6：Commit**

```bash
git add -A && git commit -m "feat: wire Phase 2 adapters, Rod pool, proxy pool into main"
```

---

## 计划总结

| 任务 | 文件 | 预估 |
|------|------|------|
| 1. Rod 浏览器池 | rod_pool.go + test | 20 min |
| 2. 代理池激活 | proxy.go 重写 + test | 15 min |
| 3. 艾瑞数据 Adapter | iresearch/ + test | 15 min |
| 4. 骨朵数据 Adapter | guduo/ + test | 15 min |
| 5. 猫眼票房 Adapter | maoyan/ + test | 15 min |
| 6. 企鹅智库 Adapter | penguin_intelligence/ + test | 15 min |
| 7. 腾讯研究院 Adapter | tencent_research/ + test | 15 min |
| 8. Wire 集成 | main.go + config | 20 min |
| **合计** | **14+ 文件** | **~2 小时** |
