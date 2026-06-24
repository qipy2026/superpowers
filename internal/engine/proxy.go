package engine

import (
	"net/url"
	"sync"
	"sync/atomic"
)

type ProxyPool struct {
	proxies []*url.URL
	cursor  atomic.Uint32
	mu      sync.RWMutex
}

func NewProxyPool(proxyURLs []string) *ProxyPool {
	p := &ProxyPool{}
	for _, raw := range proxyURLs {
		if u, err := url.Parse(raw); err == nil {
			p.proxies = append(p.proxies, u)
		}
	}
	return p
}

func (p *ProxyPool) Next() (*url.URL, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.proxies) == 0 {
		return nil, nil
	}
	idx := p.cursor.Add(1) % uint32(len(p.proxies))
	return p.proxies[idx], nil
}

func (p *ProxyPool) MarkBad(u *url.URL) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, proxy := range p.proxies {
		if proxy.Host == u.Host {
			p.proxies = append(p.proxies[:i], p.proxies[i+1:]...)
			return
		}
	}
}

func (p *ProxyPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.proxies)
}
