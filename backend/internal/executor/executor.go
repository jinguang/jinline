package executor

import (
	"context"
	"log"
	"sync"
	"time"

	"jinline/internal/config"
	"jinline/internal/queue"
	"jinline/internal/store"

	"gorm.io/gorm"
)

// Executor 从 Redis 队列消费构建任务并执行流水线。
type Executor struct {
	cfg   *config.Config
	db    *gorm.DB
	yamls *store.YamlStore
	q     *queue.Queue

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func New(cfg *config.Config, db *gorm.DB, yamls *store.YamlStore, q *queue.Queue) *Executor {
	return &Executor{
		cfg:     cfg,
		db:      db,
		yamls:   yamls,
		q:       q,
		cancels: make(map[string]context.CancelFunc),
	}
}

// Start 启动 worker 协程池。
func (e *Executor) Start(ctx context.Context) {
	for i := 0; i < e.cfg.Worker.Concurrency; i++ {
		go e.worker(ctx, i)
	}
	log.Printf("executor started with %d workers", e.cfg.Worker.Concurrency)
}

func (e *Executor) worker(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		runID, err := e.q.Dequeue(ctx, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[worker-%d] dequeue: %v", id, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if runID == "" {
			continue
		}

		log.Printf("[worker-%d] dequeued run %s", id, runID)
		e.Execute(ctx, runID)
	}
}

// Cancel 取消正在执行的运行，返回该运行是否处于执行中。
func (e *Executor) Cancel(runID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cancel, ok := e.cancels[runID]; ok {
		cancel()
		return true
	}
	return false
}

func (e *Executor) registerCancel(runID string, cancel context.CancelFunc) {
	e.mu.Lock()
	e.cancels[runID] = cancel
	e.mu.Unlock()
}

func (e *Executor) unregisterCancel(runID string) {
	e.mu.Lock()
	delete(e.cancels, runID)
	e.mu.Unlock()
}
