package haosou

import (
    "context"
    "crawler/internal/adapter"
    "fmt"
    "net/url"
    "strings"

    "github.com/gocolly/colly/v2"
)

type Adapter struct{ baseURL string }

func New(baseURL string) *Adapter { return &Adapter{baseURL: baseURL} }
func (a *Adapter) Name() string    { return "haosou" }
func (a *Adapter) Validate() error { c := colly.NewCollector(); c.Visit(a.baseURL); return nil }

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    var rows []adapter.DataRow
    u, err := url.Parse(a.baseURL)
    if err != nil { return nil, fmt.Errorf("parse URL: %w", err) }
    c := colly.NewCollector(colly.AllowedDomains(u.Hostname()))

    c.OnHTML(".search-item, .result-item, .card-item, .data-card", func(e *colly.HTMLElement) {
        title := strings.TrimSpace(e.ChildText(".title, .name, h3, a"))
        desc := strings.TrimSpace(e.ChildText(".desc, .summary, .description"))
        if title != "" {
            rows = append(rows, adapter.DataRow{
                SourceURL: e.Request.URL.String(),
                Data:      map[string]string{"title": title, "description": desc},
            })
        }
    })

    c.OnHTML("a.next, a:contains(下一页)", func(e *colly.HTMLElement) { e.Request.Visit(e.Attr("href")) })
    if err := c.Visit(a.baseURL); err != nil { return nil, fmt.Errorf("visit: %w", err) }
    c.Wait()
    return rows, nil
}
