# 网络爬虫 Phase 3 — 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 新增 Session 管理器 + 反检测工具 + 8 个高难度平台适配器（需登录/JS/反爬）

**架构：** 在现有 Engine + Colly + Rod + ProxyPool 基础上，增加 SessionManager（Cookie 持久化/复用）、AntiDetect（UA 轮换/随机延迟），适配器通过组合这些工具实现登录+采集

**技术栈：** 沿用 Phase 1/2 全部依赖，无新第三方库

---

## 文件蓝图

| 文件 | 职责 | 变更 |
|------|------|------|
| `internal/engine/session.go` | Session 管理器：Cookie 加载/保存/注入 | 新建 |
| `internal/engine/antidetect.go` | 反检测：UA 轮换、随机延迟、Referrer 管理 | 新建 |
| `internal/adapter/qingbo/adapter.go` | 清博大数据指数 | 新建 |
| `internal/adapter/xinbang/adapter.go` | 新榜 | 新建 |
| `internal/adapter/baidu_index/adapter.go` | 百度指数 | 新建 |
| `internal/adapter/tao_data/adapter.go` | 淘数据 | 新建 |
| `internal/adapter/shengyi_canmou/adapter.go` | 生意参谋 | 新建 |
| `internal/adapter/douyin_kuaishou/adapter.go` | 抖音快手网 | 新建 |
| `internal/adapter/huoshaoyun/adapter.go` | 火烧云 | 新建 |
| `internal/adapter/jiujiu_doushang/adapter.go` | 九九抖商 | 新建 |
| `cmd/crawler/main.go` | 注册新适配器 | 修改 |
| `config/config.yaml` | 新适配器配置 | 修改 |
| `config/sessions/` | Cookie 持久化目录 | 新建 |

---

### 任务 1：Session 管理器

**文件：**
- 创建：`internal/engine/session.go`
- 创建：`internal/engine/session_test.go`

管理登录 Cookie 的持久化和注入。每个平台一个 JSON 文件，存储 Cookie 数组。

- [ ] **步骤 1：编写测试**

创建 `internal/engine/session_test.go`：

```go
package engine

import (
    "net/http"
    "os"
    "path/filepath"
    "testing"
)

func TestSessionManagerSaveLoad(t *testing.T) {
    dir := t.TempDir()
    sm := NewSessionManager(dir)

    cookies := []*http.Cookie{
        {Name: "session", Value: "abc123", Domain: "example.com"},
        {Name: "token", Value: "xyz789", Domain: "example.com"},
    }
    if err := sm.Save("test_site", cookies); err != nil {
        t.Fatalf("Save failed: %v", err)
    }
    loaded, err := sm.Load("test_site")
    if err != nil {
        t.Fatalf("Load failed: %v", err)
    }
    if len(loaded) != 2 {
        t.Errorf("expected 2 cookies, got %d", len(loaded))
    }
    if loaded[0].Name != "session" || loaded[0].Value != "abc123" {
        t.Error("cookie mismatch")
    }
}

func TestSessionManagerLoadMissing(t *testing.T) {
    dir := t.TempDir()
    sm := NewSessionManager(dir)
    cookies, err := sm.Load("nonexistent")
    if err != nil {
        t.Fatalf("Load should not error on missing file: %v", err)
    }
    if cookies != nil {
        t.Error("expected nil for missing session")
    }
}

func TestSessionManagerFileCreated(t *testing.T) {
    dir := t.TempDir()
    sm := NewSessionManager(dir)
    sm.Save("test", []*http.Cookie{{Name: "a", Value: "b"}})
    expected := filepath.Join(dir, "test.json")
    if _, err := os.Stat(expected); os.IsNotExist(err) {
        t.Errorf("session file %s not created", expected)
    }
}
```

- [ ] **步骤 2：运行测试验证失败** → **步骤 3：实现** → **步骤 4：验证** → **步骤 5：Commit**

实现 `internal/engine/session.go`：

```go
package engine

import (
    "encoding/json"
    "net/http"
    "os"
    "path/filepath"
    "sync"
)

type SessionManager struct {
    dir string
    mu  sync.RWMutex
}

func NewSessionManager(dir string) *SessionManager {
    os.MkdirAll(dir, 0755)
    return &SessionManager{dir: dir}
}

func (s *SessionManager) path(name string) string {
    return filepath.Join(s.dir, name+".json")
}

func (s *SessionManager) Save(name string, cookies []*http.Cookie) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    data, err := json.MarshalIndent(cookies, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(s.path(name), data, 0644)
}

func (s *SessionManager) Load(name string) ([]*http.Cookie, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    data, err := os.ReadFile(s.path(name))
    if os.IsNotExist(err) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    var cookies []*http.Cookie
    if err := json.Unmarshal(data, &cookies); err != nil {
        return nil, err
    }
    return cookies, nil
}

func (s *SessionManager) Delete(name string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    return os.Remove(s.path(name))
}

// InjectCookies adds saved cookies to an HTTP request
func (s *SessionManager) InjectCookies(name string, req *http.Request) error {
    cookies, err := s.Load(name)
    if err != nil {
        return err
    }
    for _, c := range cookies {
        req.AddCookie(c)
    }
    return nil
}
```

Commit: `feat: add session manager for cookie persistence and reuse`

---

### 任务 2：反检测工具

**文件：**
- 创建：`internal/engine/antidetect.go`
- 创建：`internal/engine/antidetect_test.go`

UA 轮换、随机延迟、Referrer 管理——降低被反爬检测的概率。

- [ ] **步骤 1-5：TDD 实现**

创建 `internal/engine/antidetect_test.go`：

```go
package engine

import (
    "net/http"
    "testing"
    "time"
)

func TestAntiDetectRandomUA(t *testing.T) {
    ad := NewAntiDetect(nil)
    ua1 := ad.RandomUA()
    ua2 := ad.RandomUA()
    if ua1 == "" {
        t.Error("UA should not be empty")
    }
    // should rotate through pool
    found := false
    for i := 0; i < 10; i++ {
        if ad.RandomUA() != ua1 {
            found = true
            break
        }
    }
    if !found {
        t.Error("expected UA rotation")
    }
}

func TestAntiDetectRandomDelay(t *testing.T) {
    ad := NewAntiDetect(nil)
    start := time.Now()
    ad.RandomDelay(50*time.Millisecond, 100*time.Millisecond)
    elapsed := time.Since(start)
    if elapsed < 50*time.Millisecond {
        t.Error("delay too short")
    }
}

func TestAntiDetectSetHeaders(t *testing.T) {
    ad := NewAntiDetect(nil)
    req, _ := http.NewRequest("GET", "https://example.com", nil)
    ad.SetHeaders(req, "https://www.google.com")
    if req.Header.Get("User-Agent") == "" {
        t.Error("User-Agent not set")
    }
    if req.Header.Get("Referer") == "" {
        t.Error("Referer not set")
    }
    if req.Header.Get("Accept-Language") == "" {
        t.Error("Accept-Language not set")
    }
}
```

实现 `internal/engine/antidetect.go`：

```go
package engine

import (
    "math/rand"
    "net/http"
    "time"
)

var defaultUserAgents = []string{
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/125.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/124.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/125.0.0.0 Safari/537.36",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/125.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0",
}

type AntiDetect struct {
    userAgents []string
    rng        *rand.Rand
}

func NewAntiDetect(userAgents []string) *AntiDetect {
    if len(userAgents) == 0 {
        userAgents = defaultUserAgents
    }
    return &AntiDetect{
        userAgents: userAgents,
        rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

func (a *AntiDetect) RandomUA() string {
    return a.userAgents[a.rng.Intn(len(a.userAgents))]
}

func (a *AntiDetect) RandomDelay(min, max time.Duration) {
    d := min + time.Duration(a.rng.Int63n(int64(max-min)))
    time.Sleep(d)
}

func (a *AntiDetect) SetHeaders(req *http.Request, referrer string) {
    req.Header.Set("User-Agent", a.RandomUA())
    req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
    req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
    if referrer != "" {
        req.Header.Set("Referer", referrer)
    }
}
```

Commit: `feat: add anti-detection helpers (UA rotation, random delay, headers)`

---

### 任务 3-10：8 个高难度平台 Adapter

每个任务遵循 Phase 2 Adapter 模式，测试用 httptest mock HTML。共性特征：

| 平台 | 关键技术 | 选择器策略 |
|------|---------|-----------|
| 清博 (qingbo) | Rod + Session + Proxy | 舆情榜单卡片 |
| 新榜 (xinbang) | Rod + Session + Proxy | 内容排行列表 |
| 百度指数 (baidu_index) | Rod + Session + AntiDetect | 趋势图表数据 |
| 淘数据 (tao_data) | Rod + Session + Proxy | 商品/店铺数据卡片 |
| 生意参谋 (shengyi_canmou) | Rod + Session + Proxy | 经营数据面板 |
| 抖音快手网 (douyin_kuaishou) | Rod + Session + AntiDetect + Proxy | 视频/达人数据 |
| 火烧云 (huoshaoyun) | Rod + Session | 电商数据列表 |
| 九九抖商 (jiujiu_doushang) | Rod + Session + Proxy | 抖店数据卡片 |

每个 adapter 统一实现：

```go
package <name>

import (
    "context"
    "crawler/internal/adapter"
    "crawler/internal/engine"
    "fmt"
    "strings"
    "time"

    "github.com/gocolly/colly/v2"
)

type Adapter struct {
    baseURL  string
    rodPool  *engine.RodPool
    session  *engine.SessionManager
    anti     *engine.AntiDetect
    proxyPool *engine.ProxyPool
}

func New(baseURL string, rodPool *engine.RodPool, session *engine.SessionManager, anti *engine.AntiDetect, proxyPool *engine.ProxyPool) *Adapter {
    return &Adapter{baseURL: baseURL, rodPool: rodPool, session: session, anti: anti, proxyPool: proxyPool}
}

func (a *Adapter) Name() string { return "<name>" }

func (a *Adapter) Validate() error {
    c := colly.NewCollector()
    c.Visit(a.baseURL)
    return nil
}

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    // 1. Try loading saved session cookies
    // 2. Try Colly first (static fallback)
    // 3. If Colly empty, use Rod with session cookies + anti-detect + proxy
    // 4. Save cookies after Rod session
    // ...
    return rows, nil
}
```

每个 adapter 的测试：

```go
func Test<Name>Collect(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`<html><body>
          <div class="data-card"><span class="title">数据项1</span><span class="value">1234</span></div>
          <div class="data-card"><span class="title">数据项2</span><span class="value">5678</span></div>
        </body></html>`))
    }))
    defer srv.Close()
    a := New(srv.URL, nil, nil, nil, nil)
    task := &adapter.Task{ID: "t", Adapter: "<name>"}
    rows, err := a.Collect(context.Background(), task)
    if err != nil { t.Fatalf("Collect failed: %v", err) }
    if len(rows) == 0 { t.Fatal("expected data") }
}
```

Commit 格式：`feat: add <name> adapter for <description>`

---

### 任务 11：Wire — main.go + 配置

**文件：**
- 修改：`cmd/crawler/main.go`
- 修改：`config/config.yaml`

修改 main.go：
a) 添加 8 个新 adapter import
b) 初始化 SessionManager（目录 `config/sessions/`）
c) 初始化 AntiDetect（默认 UA 列表）
d) 注册 8 个 adapter，传入 rodPool, session, anti, proxyPool
e) 更新 adapterMeta map

config.yaml adapter 条目示例：

```yaml
  - name: qingbo
    enabled: true
    cron: "0 8 * * *"
    rate_limit: 3
    mode: incremental
    base_url: "https://www.gsdata.cn/"
  - name: xinbang
    enabled: true
    cron: "0 8 * * *"
    rate_limit: 3
    mode: incremental
    base_url: "https://www.newrank.cn/"
  # ... 其余 6 个
```

Commit: `feat: wire Phase 3 adapters, session manager, anti-detection`

---

## 计划总结

| 任务 | 内容 | 预估 |
|------|------|------|
| 1 | Session 管理器 | 15 min |
| 2 | 反检测工具 | 15 min |
| 3-10 | 8 个高难度 Adapter | 15 min × 8 = 2h |
| 11 | Wire 集成 | 20 min |
| **合计** | **18 文件** | **~3 小时** |
