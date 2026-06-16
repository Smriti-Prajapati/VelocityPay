package transaction

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// WorkerPool processes TransferJobs concurrently with a bounded worker count.
// Each job carries its own result channel so the HTTP handler can await the outcome.
type WorkerPool struct {
	jobs    chan TransferJob
	wg      sync.WaitGroup
	log     *zap.Logger
	process func(ctx context.Context, job TransferJob)
}

// NewWorkerPool creates a pool with workerCount goroutines and a buffered job queue.
func NewWorkerPool(workerCount, queueSize int, processor func(ctx context.Context, job TransferJob), log *zap.Logger) *WorkerPool {
	return &WorkerPool{
		jobs:    make(chan TransferJob, queueSize),
		log:     log,
		process: processor,
	}
}

// Start launches all worker goroutines. Cancel ctx to initiate a graceful shutdown.
func (p *WorkerPool) Start(ctx context.Context, workerCount int) {
	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go func(id int) {
			defer p.wg.Done()
			p.log.Debug("worker started", zap.Int("worker_id", id))
			for {
				select {
				case <-ctx.Done():
					p.log.Debug("worker stopping", zap.Int("worker_id", id))
					return
				case job, ok := <-p.jobs:
					if !ok {
						return
					}
					p.process(ctx, job)
				}
			}
		}(i)
	}
}

// Submit enqueues a job. Returns false if the queue is full (backpressure).
func (p *WorkerPool) Submit(job TransferJob) bool {
	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}

// Shutdown drains the job queue and waits for all workers to finish.
func (p *WorkerPool) Shutdown() {
	close(p.jobs)
	p.wg.Wait()
	p.log.Info("worker pool shut down")
}
