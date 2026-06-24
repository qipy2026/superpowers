package stats_gov

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
	return &Adapter{
		baseURL:   baseURL,
		collector: colly.NewCollector(colly.AllowedDomains()),
	}
}

func (a *Adapter) Name() string { return "stats_gov" }

func (a *Adapter) Validate() error {
	c := colly.NewCollector()
	c.AllowURLRevisit = false
	var visitErr error
	c.OnError(func(r *colly.Response, err error) {
		visitErr = err
	})
	c.Visit(a.baseURL)
	return visitErr
}

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
	u, err := url.Parse(a.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}

	var rows []adapter.DataRow
	c := colly.NewCollector(colly.AllowedDomains(u.Hostname()))
	seen := make(map[string]bool)

	// discover links to sub-pages with data tables
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		if strings.Contains(link, "html") || strings.Contains(link, "sj") || strings.Contains(link, "data") {
			absURL := e.Request.AbsoluteURL(link)
			if !seen[absURL] {
				seen[absURL] = true
				c.Visit(absURL)
			}
		}
	})

	// extract table rows
	c.OnHTML("table tbody tr, table tr", func(e *colly.HTMLElement) {
		cells := e.ChildTexts("td, th")
		if len(cells) < 2 {
			return
		}
		data := make(map[string]string)
		data["indicator"] = strings.TrimSpace(cells[0])
		for i := 1; i < len(cells); i++ {
			data[fmt.Sprintf("value_%d", i)] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, adapter.DataRow{
			SourceURL: e.Request.URL.String(),
			Data:      data,
		})
	})

	// support pagination — click "next page"
	c.OnHTML("a.next, a[rel=next], a:contains(下一页), a:contains(下页)", func(e *colly.HTMLElement) {
		e.Request.Visit(e.Attr("href"))
	})

	if err := c.Visit(a.baseURL); err != nil {
		return nil, fmt.Errorf("visit %s: %w", a.baseURL, err)
	}
	c.Wait()

	return rows, nil
}
