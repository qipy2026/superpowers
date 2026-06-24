package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type RodPool struct {
	maxSize  int
	pool     chan *rod.Browser
	launcher string
	started  bool
	mu       sync.Mutex
}

func NewRodPool(maxSize int) *RodPool {
	return &RodPool{
		maxSize: maxSize,
		pool:    make(chan *rod.Browser, maxSize),
	}
}

func (p *RodPool) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return nil
	}
	path, found := launcher.LookPath()
	if !found {
		return fmt.Errorf("no browser found for Rod")
	}
	p.launcher = path
	for i := 0; i < p.maxSize; i++ {
		b, err := p.newBrowser()
		if err != nil {
			return fmt.Errorf("pre-warm browser %d: %w", i, err)
		}
		p.pool <- b
	}
	p.started = true
	return nil
}

func (p *RodPool) newBrowser() (*rod.Browser, error) {
	u, err := launcher.New().Bin(p.launcher).Headless(true).Launch()
	if err != nil {
		return nil, fmt.Errorf("launch browser: %w", err)
	}
	b := rod.New().ControlURL(u)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("connect browser: %w", err)
	}
	return b, nil
}

func (p *RodPool) Borrow() (*rod.Browser, error) {
	select {
	case b := <-p.pool:
		return b, nil
	default:
		b, err := p.newBrowser()
		if err != nil {
			return nil, err
		}
		return b, nil
	}
}

func (p *RodPool) BorrowTimeout(d time.Duration) (*rod.Browser, error) {
	select {
	case b := <-p.pool:
		return b, nil
	case <-time.After(d):
		return nil, fmt.Errorf("borrow timeout after %v", d)
	}
}

func (p *RodPool) Return(b *rod.Browser) {
	if b == nil {
		return
	}
	select {
	case p.pool <- b:
	default:
		b.Close()
	}
}

func (p *RodPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	close(p.pool)
	for b := range p.pool {
		b.Close()
	}
	p.started = false
}

type RodPage struct {
	browser *rod.Browser
	pool    *RodPool
	page    *rod.Page
}

func (p *RodPool) NewRodPage(url string, timeout time.Duration) (*RodPage, error) {
	b, err := p.BorrowTimeout(timeout)
	if err != nil {
		return nil, err
	}
	page := b.MustPage(url)
	page.Timeout(timeout)
	return &RodPage{browser: b, pool: p, page: page}, nil
}

func (rp *RodPage) Page() *rod.Page { return rp.page }

func (rp *RodPage) Close() {
	if rp.page != nil {
		rp.page.Close()
	}
	rp.pool.Return(rp.browser)
}
