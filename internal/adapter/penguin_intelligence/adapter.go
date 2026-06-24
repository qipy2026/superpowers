package penguin_intelligence

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"crawler/internal/adapter"

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

	c.OnHTML(".post-item, .report-item, .article-item", func(e *colly.HTMLElement) {
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
