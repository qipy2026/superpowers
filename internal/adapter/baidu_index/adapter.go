package baidu_index

import (
	"context"
	"crawler/internal/adapter"
	"crawler/internal/engine"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/gocolly/colly/v2"
)

type Adapter struct {
	baseURL   string
	rodPool   *engine.RodPool
	session   *engine.SessionManager
	anti      *engine.AntiDetect
	proxyPool *engine.ProxyPool
	solver    engine.CaptchaSolver
}

func New(baseURL string, rodPool *engine.RodPool, session *engine.SessionManager, anti *engine.AntiDetect, proxyPool *engine.ProxyPool, solver engine.CaptchaSolver) *Adapter {
	return &Adapter{baseURL: baseURL, rodPool: rodPool, session: session, anti: anti, proxyPool: proxyPool, solver: solver}
}

func (a *Adapter) Name() string    { return "baidu_index" }
func (a *Adapter) Validate() error { c := colly.NewCollector(); c.Visit(a.baseURL); return nil }

func (a *Adapter) ensureLogin(ctx context.Context, page *rod.Page) error {
	cookies, _ := a.session.Load("baidu_index")
	if cookies != nil {
		params := make([]*proto.NetworkCookieParam, 0, len(cookies))
		for _, c := range cookies {
			params = append(params, &proto.NetworkCookieParam{Name: c.Name, Value: c.Value, Domain: c.Domain})
		}
		page.SetCookies(params)
		page.MustNavigate(a.baseURL)
		page.MustWaitStable()
		el, _ := page.Element("#login-btn, .login-link, a:contains(登录)")
		if el == nil {
			return nil
		}
	}
	return a.performLogin(ctx, page)
}

func (a *Adapter) performLogin(ctx context.Context, page *rod.Page) error {
	if a.solver == nil {
		return fmt.Errorf("captcha solver not available, cannot auto-login")
	}
	page.MustNavigate("https://index.baidu.com/")
	page.MustWaitStable()
	loginBtn, err := page.Element("#login-btn, a:contains(登录)")
	if err != nil {
		return fmt.Errorf("login button not found: %w", err)
	}
	loginBtn.MustClick()
	time.Sleep(2 * time.Second)

	for attempt := 0; attempt < 2; attempt++ {
		captchaEl, err := page.Element("#captcha_img, .verify-img, img[src*=captcha]")
		if err != nil {
			return fmt.Errorf("captcha image not found: %w", err)
		}
		img, err := captchaEl.Screenshot(proto.PageCaptureScreenshotFormatPng, 90)
		if err != nil {
			return fmt.Errorf("captcha screenshot: %w", err)
		}
		result, err := a.solver.Solve(ctx, img, "1902")
		if err != nil {
			return fmt.Errorf("captcha solve: %w", err)
		}
		inputEl, _ := page.Element("#captcha_input, input[name=captcha]")
		if inputEl != nil {
			inputEl.MustInput(result.Code)
		}
		submitBtn, _ := page.Element("#login-submit, button[type=submit]")
		if submitBtn != nil {
			submitBtn.MustClick()
		}
		time.Sleep(3 * time.Second)
		el, _ := page.Element("#login-btn, .login-link, a:contains(登录)")
		if el == nil {
			browserCookies, _ := page.Cookies(nil)
			httpCookies := make([]*http.Cookie, len(browserCookies))
			for i, c := range browserCookies {
				httpCookies[i] = &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain}
			}
			a.session.Save("baidu_index", httpCookies)
			return nil
		}
		a.solver.ReportError(ctx, result.ID)
	}
	return fmt.Errorf("login failed after 2 attempts")
}

func (a *Adapter) checkCaptcha(page *rod.Page) bool {
	el, _ := page.Element(".verify-img, #captcha, .passMod_dialog, .captcha-box")
	return el != nil
}

func (a *Adapter) handleCaptcha(ctx context.Context, page *rod.Page) error {
	if a.solver == nil {
		return fmt.Errorf("captcha solver not available")
	}
	for attempt := 0; attempt < 2; attempt++ {
		captchaEl, err := page.Element(".verify-img, #captcha img, .captcha-box img")
		if err != nil {
			return fmt.Errorf("captcha not found: %w", err)
		}
		img, _ := captchaEl.Screenshot(proto.PageCaptureScreenshotFormatPng, 90)
		result, err := a.solver.Solve(ctx, img, "1902")
		if err != nil {
			return fmt.Errorf("solve: %w", err)
		}
		inputEl, _ := page.Element("input.captcha-input, #captcha_code")
		if inputEl != nil {
			inputEl.MustInput(result.Code)
		}
		submitEl, _ := page.Element(".captcha-submit, #verify_btn")
		if submitEl != nil {
			submitEl.MustClick()
		}
		time.Sleep(2 * time.Second)
		if !a.checkCaptcha(page) {
			return nil
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
	if err != nil {
		return nil, fmt.Errorf("rod: %w", err)
	}
	defer rp.Close()

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
		if err != nil {
			break
		}
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
			info, _ := page.Info()
			sourceURL := ""
			if info != nil {
				sourceURL = info.URL
			}
			rows = append(rows, adapter.DataRow{
				SourceURL: sourceURL,
				Data:      map[string]string{"title": title, "value": value},
			})
		}
	}
	return rows
}
