package maoyan

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

func (a *Adapter) Name() string    { return "maoyan" }
func (a *Adapter) Validate() error { c := colly.NewCollector(); c.Visit(a.baseURL); return nil }

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
	var rows []adapter.DataRow
	u, err := url.Parse(a.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}

	// Try Colly first (static fallback)
	c := colly.NewCollector(colly.AllowedDomains(u.Hostname()))
	c.OnHTML(".movie-box, .movie-item, .detail-block", func(e *colly.HTMLElement) {
		name := strings.TrimSpace(e.ChildText(".movie-name, .name, .title"))
		boxOffice := strings.TrimSpace(e.ChildText(".box-office, .total-boxoffice, .revenue"))
		if name != "" {
			rows = append(rows, adapter.DataRow{
				SourceURL: e.Request.URL.String(),
				Data: map[string]string{
					"name":       name,
					"box_office": boxOffice,
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

	// Colly got results, return
	if len(rows) > 0 {
		return rows, nil
	}

	// Rod required for JS-rendered content
	if a.rodPool == nil {
		return nil, fmt.Errorf("maoyan requires Rod for JS rendering, but rodPool is nil")
	}
	rp, err := a.rodPool.NewRodPage(a.baseURL, 30_000_000_000) // 30s
	if err != nil {
		return nil, fmt.Errorf("rod page: %w", err)
	}
	defer rp.Close()

	rp.Page().MustWaitStable()
	els, _ := rp.Page().Elements(".movie-box, .movie-item, .detail-block")
	for _, el := range els {
		name := ""
		boxOffice := ""
		if n, err := el.Element(".movie-name, .name"); err == nil {
			name = strings.TrimSpace(n.MustText())
		}
		if bo, err := el.Element(".box-office, .total-boxoffice, .revenue"); err == nil {
			boxOffice = strings.TrimSpace(bo.MustText())
		}
		if name == "" {
			name = strings.TrimSpace(el.MustText())
		}
		if name != "" {
			rows = append(rows, adapter.DataRow{
				SourceURL: a.baseURL,
				Data: map[string]string{
					"name":       name,
					"box_office": boxOffice,
				},
			})
		}
	}
	return rows, nil
}

var _ = rod.Try
