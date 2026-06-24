package guduo

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

	if err := c.Visit(a.baseURL); err != nil {
		return nil, fmt.Errorf("visit: %w", err)
	}
	c.Wait()

	// Rod fallback for JS-rendered content
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

var _ = rod.Try
