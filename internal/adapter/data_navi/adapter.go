package data_navi

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
func (a *Adapter) Name() string    { return "data_navi" }
func (a *Adapter) Validate() error { c := colly.NewCollector(); c.Visit(a.baseURL); return nil }

func (a *Adapter) Collect(ctx context.Context, task *adapter.Task) ([]adapter.DataRow, error) {
    var rows []adapter.DataRow
    u, err := url.Parse(a.baseURL)
    if err != nil { return nil, fmt.Errorf("parse URL: %w", err) }
    c := colly.NewCollector(colly.AllowedDomains(u.Hostname()))

    // Links
    c.OnHTML("a[href]", func(e *colly.HTMLElement) {
        name := strings.TrimSpace(e.Text)
        link := e.Attr("href")
        if name != "" && link != "" && len(name) < 200 {
            rows = append(rows, adapter.DataRow{
                SourceURL: e.Request.AbsoluteURL(link),
                Data:      map[string]string{"name": name, "link": link},
            })
        }
    })

    // Data cards
    c.OnHTML(".data-card, .site-item, .nav-item, .link-item", func(e *colly.HTMLElement) {
        title := strings.TrimSpace(e.ChildText(".title, .name, .text"))
        value := strings.TrimSpace(e.ChildText(".value, .num"))
        if title != "" {
            rows = append(rows, adapter.DataRow{
                SourceURL: e.Request.URL.String(),
                Data:      map[string]string{"title": title, "value": value},
            })
        }
    })

    if err := c.Visit(a.baseURL); err != nil { return nil, fmt.Errorf("visit: %w", err) }
    c.Wait()
    return rows, nil
}
