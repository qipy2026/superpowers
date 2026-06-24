# 打码平台集成 — 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。

**目标：** 定义 CaptchaSolver 接口 + 超级鹰实现，百度指数支持自动登录和运行时验证码处理

**架构：** engine 包新增 CaptchaSolver 接口和 ChaojiyingSolver，baidu_index adapter 重写为 Rod-Only 模式集成打码

**技术栈：** 无新依赖，纯 net/http + multipart

---

## 文件蓝图

| 文件 | 职责 | 变更 |
|------|------|------|
| `internal/engine/captcha.go` | CaptchaSolver 接口 + CaptchaResult | 新建 |
| `internal/engine/chaojiying.go` | 超级鹰 HTTP 客户端 | 新建 |
| `internal/engine/chaojiying_test.go` | 超级鹰测试 | 新建 |
| `internal/adapter/baidu_index/adapter.go` | 重写：登录+运行时验证码 | 修改 |
| `internal/adapter/baidu_index/adapter_test.go` | 扩展测试 | 修改 |
| `cmd/crawler/main.go` | 初始化 Solver，注入 adapter | 修改 |
| `config/config.yaml` | captcha 配置段 | 修改 |

---

### 任务 1：CaptchaSolver 接口

**文件：**
- 创建：`internal/engine/captcha.go`

- [ ] **步骤 1：编写接口**

```go
package engine

import "context"

type CaptchaSolver interface {
    Solve(ctx context.Context, img []byte, captchaType string) (*CaptchaResult, error)
    ReportError(ctx context.Context, id string) error
}

type CaptchaResult struct {
    ID   string
    Code string
}
```

- [ ] **步骤 2：验证编译**

```bash
export GOPROXY="https://goproxy.cn,direct" && export PATH="/c/Go/bin:$PATH" && cd e:/work/code/superpowers && go build ./internal/engine/...
```
预期：成功

- [ ] **步骤 3：Commit**

```bash
git add internal/engine/captcha.go && git commit -m "feat: add CaptchaSolver interface"
```

---

### 任务 2：超级鹰客户端

**文件：**
- 创建：`internal/engine/chaojiying.go`
- 创建：`internal/engine/chaojiying_test.go`

- [ ] **步骤 1：编写测试**

创建 `internal/engine/chaojiying_test.go`：

```go
package engine

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestChaojiyingSolve(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        r.ParseMultipartForm(10 << 20)
        if r.FormValue("user") != "testuser" {
            t.Error("expected testuser in form")
        }
        w.Write([]byte(`{"err_no":0,"err_str":"OK","pic_id":"123","pic_str":"AB3D"}`))
    }))
    defer srv.Close()

    solver := NewChaojiyingSolver("testuser", "testpass", "96001")
    solver.baseURL = srv.URL // override for testing

    result, err := solver.Solve(context.Background(), []byte("fake-image-data"), "1902")
    if err != nil {
        t.Fatalf("Solve failed: %v", err)
    }
    if result.Code != "AB3D" {
        t.Errorf("expected AB3D, got %s", result.Code)
    }
    if result.ID != "123" {
        t.Errorf("expected id 123, got %s", result.ID)
    }
}

func TestChaojiyingSolveError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"err_no":1001,"err_str":"no balance"}`))
    }))
    defer srv.Close()

    solver := NewChaojiyingSolver("testuser", "testpass", "96001")
    solver.baseURL = srv.URL

    _, err := solver.Solve(context.Background(), []byte("fake"), "1902")
    if err == nil {
        t.Fatal("expected error for no balance")
    }
    if !strings.Contains(err.Error(), "no balance") {
        t.Errorf("expected 'no balance' in error, got: %v", err)
    }
}

func TestChaojiyingReportError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"err_no":0,"err_str":"OK"}`))
    }))
    defer srv.Close()

    solver := NewChaojiyingSolver("testuser", "testpass", "96001")
    solver.baseURL = srv.URL

    if err := solver.ReportError(context.Background(), "123"); err != nil {
        t.Fatalf("ReportError failed: %v", err)
    }
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
go test ./internal/engine/... -v -run TestChaojiying
```
预期：FAIL

- [ ] **步骤 3：实现超级鹰客户端**

创建 `internal/engine/chaojiying.go`：

```go
package engine

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "time"
)

type ChaojiyingSolver struct {
    user   string
    pass   string
    softID string
    client *http.Client
    baseURL string
}

type chaojiyingResp struct {
    ErrNo  int    `json:"err_no"`
    ErrStr string `json:"err_str"`
    PicID  string `json:"pic_id"`
    PicStr string `json:"pic_str"`
}

func NewChaojiyingSolver(user, pass, softID string) *ChaojiyingSolver {
    return &ChaojiyingSolver{
        user:   user,
        pass:   pass,
        softID: softID,
        client: &http.Client{Timeout: 30 * time.Second},
        baseURL: "http://upload.chaojiying.net/Upload/Processing.php",
    }
}

func (s *ChaojiyingSolver) Solve(ctx context.Context, img []byte, captchaType string) (*CaptchaResult, error) {
    var buf bytes.Buffer
    w := multipart.NewWriter(&buf)
    w.WriteField("user", s.user)
    w.WriteField("pass", s.pass)
    w.WriteField("softid", s.softID)
    w.WriteField("codetype", captchaType)
    w.WriteField("len_min", "0")
    w.WriteField("time_out", "20")
    fw, _ := w.CreateFormFile("userfile", "captcha.png")
    fw.Write(img)
    w.Close()

    req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL, &buf)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", w.FormDataContentType())

    resp, err := s.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("chaojiying request: %w", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    var r chaojiyingResp
    if err := json.Unmarshal(body, &r); err != nil {
        return nil, fmt.Errorf("chaojiying parse: %w", err)
    }
    if r.ErrNo != 0 {
        return nil, fmt.Errorf("chaojiying error %d: %s", r.ErrNo, r.ErrStr)
    }
    return &CaptchaResult{ID: r.PicID, Code: r.PicStr}, nil
}

func (s *ChaojiyingSolver) ReportError(ctx context.Context, id string) error {
    url := fmt.Sprintf("%s?action=reportbad&id=%s&softid=%s",
        "http://upload.chaojiying.net/Upload/ReportBad.php", id, s.softID)
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    resp, err := s.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
go test ./internal/engine/... -v -run TestChaojiying
```
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/engine/chaojiying.go internal/engine/chaojiying_test.go && git commit -m "feat: add Chaojiying captcha solver"
```

---

### 任务 3：百度指数 Adapter 重写

**文件：**
- 修改：`internal/adapter/baidu_index/adapter.go`
- 修改：`internal/adapter/baidu_index/adapter_test.go`

- [ ] **步骤 1：重写 adapter.go**

关键新增方法：`ensureLogin`, `performLogin`, `checkCaptcha`, `handleCaptcha`, `solveWithRetry`, `extractPageItems`

```go
package baidu_index

import (
    "context"
    "crawler/internal/adapter"
    "crawler/internal/engine"
    "fmt"
    "strings"
    "time"

    "github.com/go-rod/rod"
    "github.com/gocolly/colly/v2"
)

type Adapter struct {
    baseURL   string
    rodPool   *engine.RodPool
    session   *engine.SessionManager
    anti      *engine.AntiDetect
    proxyPool *engine.ProxyPool
    solver    engine.CaptchaSolver // 新增
}

func New(baseURL string, rodPool *engine.RodPool, session *engine.SessionManager, anti *engine.AntiDetect, proxyPool *engine.ProxyPool, solver engine.CaptchaSolver) *Adapter {
    return &Adapter{baseURL: baseURL, rodPool: rodPool, session: session, anti: anti, proxyPool: proxyPool, solver: solver}
}

func (a *Adapter) Name() string    { return "baidu_index" }
func (a *Adapter) Validate() error { c := colly.NewCollector(); c.Visit(a.baseURL); return nil }

// ensureLogin 尝试 Cookie，无效则自动登录
func (a *Adapter) ensureLogin(ctx context.Context, page *rod.Page) error {
    cookies, _ := a.session.Load("baidu_index")
    if cookies != nil {
        for _, c := range cookies {
            page.SetCookies(&rod.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain})
        }
        page.MustNavigate(a.baseURL)
        page.MustWaitStable()
        // 检测是否已登录（页面无登录按钮）
        el, _ := page.Element("#login-btn, .login-link, a:contains(登录)")
        if el == nil {
            return nil // Cookie 有效
        }
    }
    return a.performLogin(ctx, page)
}

// performLogin 自动登录，遇到验证码调打码
func (a *Adapter) performLogin(ctx context.Context, page *rod.Page) error {
    if a.solver == nil {
        return fmt.Errorf("captcha solver not available, cannot auto-login")
    }
    page.MustNavigate("https://index.baidu.com/")
    page.MustWaitStable()

    // 点击登录按钮
    loginBtn, err := page.Element("#login-btn, a:contains(登录)")
    if err != nil { return fmt.Errorf("login button not found: %w", err) }
    loginBtn.MustClick()
    time.Sleep(2 * time.Second)

    // 截图验证码
    code := ""
    for attempt := 0; attempt < 2; attempt++ {
        captchaEl, err := page.Element("#captcha_img, .verify-img, img[src*=captcha]")
        if err != nil { return fmt.Errorf("captcha image not found: %w", err) }
        img, err := captchaEl.Screenshot(rod.ScreenshotOptions{Format: rod.ScreenshotFormatPNG})
        if err != nil { return fmt.Errorf("captcha screenshot: %w", err) }

        result, err := a.solver.Solve(ctx, img, "1902")
        if err != nil { return fmt.Errorf("captcha solve: %w", err) }

        // 填入验证码
        inputEl, _ := page.Element("#captcha_input, input[name=captcha]")
        if inputEl != nil { inputEl.MustInput(result.Code) }

        // 点击提交登录
        submitBtn, _ := page.Element("#login-submit, button[type=submit]")
        if submitBtn != nil { submitBtn.MustClick() }
        time.Sleep(3 * time.Second)

        // 检测登录成功
        el, _ := page.Element("#login-btn, .login-link, a:contains(登录)")
        if el == nil {
            code = result.Code
            // 保存 Cookie
            cookies := page.MustCookies()
            httpCookies := make([]*http.Cookie, len(cookies))
            for i, c := range cookies {
                httpCookies[i] = &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain}
            }
            a.session.Save("baidu_index", httpCookies)
            return nil
        }
        // 失败，上报
        a.solver.ReportError(ctx, result.ID)
    }
    return fmt.Errorf("login failed after 2 attempts, last code: %s", code)
}

// checkCaptcha 检测页面是否有验证码弹窗
func (a *Adapter) checkCaptcha(page *rod.Page) bool {
    el, _ := page.Element(".verify-img, #captcha, .passMod_dialog, .captcha-box")
    return el != nil
}

// handleCaptcha 处理运行时验证码
func (a *Adapter) handleCaptcha(ctx context.Context, page *rod.Page) error {
    if a.solver == nil {
        return fmt.Errorf("captcha solver not available")
    }
    for attempt := 0; attempt < 2; attempt++ {
        captchaEl, err := page.Element(".verify-img, #captcha img, .captcha-box img")
        if err != nil { return fmt.Errorf("captcha not found: %w", err) }
        img, _ := captchaEl.Screenshot(rod.ScreenshotOptions{Format: rod.ScreenshotFormatPNG})

        result, err := a.solver.Solve(ctx, img, "1902")
        if err != nil { return fmt.Errorf("solve: %w", err) }

        inputEl, _ := page.Element("input.captcha-input, #captcha_code")
        if inputEl != nil { inputEl.MustInput(result.Code) }

        submitEl, _ := page.Element(".captcha-submit, #verify_btn")
        if submitEl != nil { submitEl.MustClick() }
        time.Sleep(2 * time.Second)

        if !a.checkCaptcha(page) {
            return nil // 验证通过
        }
        a.solver.ReportError(ctx, result.ID)
    }
    return fmt.Errorf("runtime captcha failed after 2 attempts")
}

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    if a.rodPool == nil {
        return nil, fmt.Errorf("baidu_index requires Rod")
    }

    rp, err := a.rodPool.NewRodPage(a.baseURL, 60*time.Second)
    if err != nil { return nil, fmt.Errorf("rod: %w", err) }
    defer rp.Close()

    // 登录
    if err := a.ensureLogin(ctx, rp.Page()); err != nil {
        return nil, fmt.Errorf("login: %w", err)
    }

    var rows []adapter.DataRow
    pageCount := 0

    for {
        pageCount++
        items := a.extractPageItems(rp.Page())
        rows = append(rows, items...)

        if pageCount%10 == 0 && a.checkCaptcha(rp.Page()) {
            if err := a.handleCaptcha(ctx, rp.Page()); err != nil {
                task.Error = fmt.Sprintf("captcha failed at page %d: %v", pageCount, err)
                break
            }
        }

        nextBtn, err := rp.Page().Element(".next-page:not(.disabled), a:contains(下一页)")
        if err != nil { break }
        nextBtn.MustClick()
        time.Sleep(1 * time.Second)
        rp.Page().MustWaitStable()
    }

    return rows, nil
}

func (a *Adapter) extractPageItems(page *rod.Page) []adapter.DataRow {
    var rows []adapter.DataRow
    els, _ := page.Elements(".trend-item, .chart-card, .data-row, .index-item")
    for _, el := range els {
        title := ""
        value := ""
        if t, err := el.Element(".title, .name, .keyword"); err == nil {
            title = strings.TrimSpace(t.MustText())
        }
        if v, err := el.Element(".value, .index, .num"); err == nil {
            value = strings.TrimSpace(v.MustText())
        }
        if title != "" {
            rows = append(rows, adapter.DataRow{
                SourceURL: page.MustInfo().URL,
                Data:      map[string]string{"title": title, "value": value},
            })
        }
    }
    return rows
}
```

- [ ] **步骤 2：更新测试**

修改 `internal/adapter/baidu_index/adapter_test.go`：

```go
package baidu_index

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "crawler/internal/adapter"
    "crawler/internal/engine"
)

type mockSolver struct{}

func (m *mockSolver) Solve(ctx context.Context, img []byte, typ string) (*engine.CaptchaResult, error) {
    return &engine.CaptchaResult{ID: "mock1", Code: "ABCD"}, nil
}
func (m *mockSolver) ReportError(ctx context.Context, id string) error { return nil }

func TestCollect(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`<html><body>
          <div class="trend-item"><span class="keyword">关键词1</span><span class="index">12345</span></div>
          <div class="trend-item"><span class="keyword">关键词2</span><span class="index">67890</span></div>
        </body></html>`))
    }))
    defer srv.Close()
    a := New(srv.URL, nil, nil, nil, nil, &mockSolver{})
    task := &adapter.Task{ID: "t", Adapter: "baidu_index"}
    rows, err := a.Collect(context.Background(), task)
    if err != nil { t.Fatalf("Collect failed: %v", err) }
    if len(rows) == 0 { t.Fatal("expected data") }
    t.Logf("collected %d items", len(rows))
}
```

- [ ] **步骤 3：运行测试**

```bash
go test ./internal/adapter/baidu_index/... -v
```
预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/adapter/baidu_index/ && git commit -m "feat: rewrite baidu_index adapter with login+captcha support"
```

---

### 任务 4：Wire — main.go + 配置

**文件：**
- 修改：`cmd/crawler/main.go`
- 修改：`config/config.yaml`

- [ ] **步骤 1：更新 main.go**

在 import 中无需新增。在 Engine 初始化后添加：

```go
// --- Captcha Solver ---
var captchaSolver engine.CaptchaSolver
if cfg.Captcha.Provider == "chaojiying" {
    captchaSolver = engine.NewChaojiyingSolver(
        cfg.Captcha.Chaojiying.User,
        cfg.Captcha.Chaojiying.Password,
        cfg.Captcha.Chaojiying.SoftID,
    )
    logger.Info("captcha solver: chaojiying")
}
```

在 Config struct 中新增：

```go
Captcha struct {
    Provider   string `mapstructure:"provider"`
    Chaojiying struct {
        User   string `mapstructure:"user"`
        Password string `mapstructure:"password"`
        SoftID string `mapstructure:"soft_id"`
    } `mapstructure:"chaojiying"`
} `mapstructure:"captcha"`
```

在百度指数注册处改为：

```go
reg.Register(baidu_index.New(getBaseURL(cfg.Adapters, "baidu_index"), rodPool, sessionMgr, antiDetect, proxyPool, captchaSolver))
```

- [ ] **步骤 2：更新 config.yaml**

```yaml
captcha:
  provider: chaojiying
  chaojiying:
    user: ${CAPTCHA_USER}
    password: ${CAPTCHA_PASS}
    soft_id: "96001"
```

- [ ] **步骤 3：编译验证**

```bash
go build ./... && go build -o crawler.exe ./cmd/crawler/
```
预期：BUILD OK

- [ ] **步骤 4：运行全部测试**

```bash
go test ./internal/... -count=1 2>&1 | grep -E "^(ok|FAIL)"
```
预期：全部 ok

- [ ] **步骤 5：Commit**

```bash
git add -A && git commit -m "feat: wire captcha solver into baidu_index adapter"
```

---

## 计划总结

| 任务 | 预估 |
|------|------|
| 1. CaptchaSolver 接口 | 5 min |
| 2. 超级鹰客户端 + 测试 | 20 min |
| 3. 百度指数适配器重写 | 25 min |
| 4. Wire 集成 | 15 min |
| **合计** | **~1 小时** |
