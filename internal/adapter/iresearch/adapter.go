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
	baseURL string
	rodPool *engine.RodPool
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

	// Rod fallback for JS-rendered content
	if len(rows) == 0 && a.rodPool != nil {
		rp, err := a.rodPool.NewRodPage(a.baseURL, 30_000_000_000) // 30s
		if err != nil {
			return nil, fmt.Errorf("rod page: %w", err)
		}
		defer rp.Close()

		rp.Page().MustWaitStable()
		els, _ := rp.Page().Elements(".report-item, .article-item, .list-item")
		for _, el := range els {
			title := ""
			date := ""
			if t, err := el.Element(".title"); err == nil {
				title = strings.TrimSpace(t.MustText())
			}
			if d, err := el.Element(".date, .time"); err == nil {
				date = strings.TrimSpace(d.MustText())
			}
			if title != "" {
				rows = append(rows, adapter.DataRow{
					SourceURL: a.baseURL,
					Data: map[string]string{
						"title": title,
						"date":  date,
					},
				})
			}
		}
	}

	return rows, nil
}

var _ = rod.Try
