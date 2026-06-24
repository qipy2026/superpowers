package engine

import "net/url"

type ProxyPool struct{}

func NewProxyPool() *ProxyPool { return &ProxyPool{} }

func (p *ProxyPool) Next() (*url.URL, error) {
	return nil, nil // Phase 2: return actual proxy
}

func (p *ProxyPool) MarkBad(u *url.URL) {
	// Phase 2: remove from pool
}
