package engine

import (
    "encoding/json"
    "net/http"
    "os"
    "path/filepath"
    "sync"
)

type SessionManager struct {
    dir string
    mu  sync.RWMutex
}

func NewSessionManager(dir string) *SessionManager {
    os.MkdirAll(dir, 0755)
    return &SessionManager{dir: dir}
}

func (s *SessionManager) path(name string) string {
    return filepath.Join(s.dir, name+".json")
}

func (s *SessionManager) Save(name string, cookies []*http.Cookie) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    data, err := json.MarshalIndent(cookies, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(s.path(name), data, 0644)
}

func (s *SessionManager) Load(name string) ([]*http.Cookie, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    data, err := os.ReadFile(s.path(name))
    if os.IsNotExist(err) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    var cookies []*http.Cookie
    if err := json.Unmarshal(data, &cookies); err != nil {
        return nil, err
    }
    return cookies, nil
}

func (s *SessionManager) Delete(name string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    return os.Remove(s.path(name))
}

func (s *SessionManager) InjectCookies(name string, req *http.Request) error {
    cookies, err := s.Load(name)
    if err != nil {
        return err
    }
    for _, c := range cookies {
        req.AddCookie(c)
    }
    return nil
}
