package scheduler

import (
	"context"
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Engine interface {
	Collect(ctx context.Context, adapterName string) (int64, error)
}

type Scheduler struct {
	cron   *cron.Cron
	engine Engine
	logger *zap.Logger
	mu     sync.Mutex
	jobs   map[string]cron.EntryID // adapter -> cron entry
}

func New(engine Engine, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		engine: engine,
		logger: logger,
		jobs:   make(map[string]cron.EntryID),
	}
}

func (s *Scheduler) AddJob(adapterName, cronExpr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.jobs[adapterName]; ok {
		s.cron.Remove(existing)
	}
	if cronExpr == "" {
		return nil // manual only
	}
	id, err := s.cron.AddFunc(cronExpr, func() {
		s.logger.Info("cron triggered", zap.String("adapter", adapterName))
		_, err := s.engine.Collect(context.Background(), adapterName)
		if err != nil {
			s.logger.Error("cron collect failed", zap.String("adapter", adapterName), zap.Error(err))
		}
	})
	if err != nil {
		return fmt.Errorf("add cron for %s: %w", adapterName, err)
	}
	s.jobs[adapterName] = id
	s.logger.Info("scheduled cron job", zap.String("adapter", adapterName), zap.String("cron", cronExpr))
	return nil
}

func (s *Scheduler) RemoveJob(adapterName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.jobs[adapterName]; ok {
		s.cron.Remove(id)
		delete(s.jobs, adapterName)
	}
}

func (s *Scheduler) Start() {
	s.logger.Info("scheduler started")
	s.cron.Start()
}

func (s *Scheduler) Stop() context.Context {
	s.logger.Info("scheduler stopping")
	return s.cron.Stop()
}

func (s *Scheduler) Trigger(adapterName string) (int64, error) {
	s.logger.Info("manual trigger", zap.String("adapter", adapterName))
	return s.engine.Collect(context.Background(), adapterName)
}
