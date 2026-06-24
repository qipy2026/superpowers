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

	// Extract rank items
	c.OnHTML(".rank-item, .ranking-item, li[class*=rank], tr[class*=rank]", func(e *colly.HTMLElement) {
		rank := strings.TrimSpace(e.ChildText(".rank, .ranking, td:first-child"))
		name := strings.TrimSpace(e.ChildText(".name, .title, td:nth-child(2)"))
		score := strings.TrimSpace(e.ChildText(".score, .value, td:nth-child(3)"))
		if name == "" {
			// fallback: grab all text from spans or tds
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
	c.AllowedDomains = []string{u.Hostname()}

	if err := c.Visit(a.baseURL); err != nil {
		return nil, fmt.Errorf("visit %s: %w", a.baseURL, err)
	}
	c.Wait()

	return rows, nil
}
