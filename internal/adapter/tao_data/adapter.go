package tao_data

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
    baseURL   string
    rodPool   *engine.RodPool
    session   *engine.SessionManager
    anti      *engine.AntiDetect
    proxyPool *engine.ProxyPool
}

func New(baseURL string, rodPool *engine.RodPool, session *engine.SessionManager, anti *engine.AntiDetect, proxyPool *engine.ProxyPool) *Adapter {
    return &Adapter{baseURL: baseURL, rodPool: rodPool, session: session, anti: anti, proxyPool: proxyPool}
}

func (a *Adapter) Name() string    { return "tao_data" }
func (a *Adapter) Validate() error { c := colly.NewCollector(); c.Visit(a.baseURL); return nil }

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    var rows []adapter.DataRow
    u, err := url.Parse(a.baseURL)
    if err != nil { return nil, fmt.Errorf("parse URL: %w", err) }
    c := colly.NewCollector(colly.AllowedDomains(u.Hostname()))

    c.OnHTML(".data-card, .goods-item, .shop-card, .data-row", func(e *colly.HTMLElement) {
        title := strings.TrimSpace(e.ChildText(".title, .name, h3, a"))
        value := strings.TrimSpace(e.ChildText(".value, .score, .index, .num, .count"))
        if title != "" {
            rows = append(rows, adapter.DataRow{
                SourceURL: e.Request.URL.String(),
                Data:      map[string]string{"title": title, "value": value},
            })
        }
    })

    c.OnHTML("a.next, a:contains(下一页)", func(e *colly.HTMLElement) { e.Request.Visit(e.Attr("href")) })
    if err := c.Visit(a.baseURL); err != nil { return nil, fmt.Errorf("visit: %w", err) }
    c.Wait()

    if len(rows) == 0 && a.rodPool != nil {
        rp, err := a.rodPool.NewRodPage(a.baseURL, 30_000_000_000)
        if err != nil { return nil, fmt.Errorf("rod: %w", err) }
        defer rp.Close()
        rp.Page().MustWaitStable()
        els, _ := rp.Page().Elements(".data-card, .goods-item, .shop-card, .data-row")
        for _, el := range els {
            if text := strings.TrimSpace(el.MustText()); text != "" {
                rows = append(rows, adapter.DataRow{SourceURL: a.baseURL, Data: map[string]string{"text": text}})
            }
        }
    }
    return rows, nil
}
