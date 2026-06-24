package engine

import (
    "math/rand"
    "net/http"
    "time"
)

var defaultUserAgents = []string{
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0",
}

type AntiDetect struct {
    userAgents []string
    rng        *rand.Rand
}

func NewAntiDetect(userAgents []string) *AntiDetect {
    if len(userAgents) == 0 {
        userAgents = defaultUserAgents
    }
    return &AntiDetect{
        userAgents: userAgents,
        rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

func (a *AntiDetect) RandomUA() string {
    return a.userAgents[a.rng.Intn(len(a.userAgents))]
}

func (a *AntiDetect) RandomDelay(min, max time.Duration) {
    d := min + time.Duration(a.rng.Int63n(int64(max-min)))
    time.Sleep(d)
}

func (a *AntiDetect) SetHeaders(req *http.Request, referrer string) {
    req.Header.Set("User-Agent", a.RandomUA())
    req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
    req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
    if referrer != "" {
        req.Header.Set("Referer", referrer)
    }
}
