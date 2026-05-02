package worker

import (
	"context"
	"log"
	"sync"

	"fraud-scorer/internal/models"
	"fraud-scorer/internal/store"
)

// IngestWorkerPool manages async transaction writes via a buffered channel.
type IngestWorkerPool struct {
	queue   chan *models.Transaction
	store   store.TransactionStore
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewIngestWorkerPool creates a pool with the given buffer size and worker count.
func NewIngestWorkerPool(bufSize, workers int, store store.TransactionStore) *IngestWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &IngestWorkerPool{
		queue:   make(chan *models.Transaction, bufSize),
		store:   store,
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.runWorker(i)
	}
	return pool
}

// Queue submits a transaction for async persistence. Non-blocking; drops if full.
func (p *IngestWorkerPool) Queue(tx *models.Transaction) {
	select {
	case p.queue <- tx:
	default:
		// Channel full; drop to protect latency. In production, log/metric this.
		log.Println("async write queue full, dropping transaction")
	}
}

// QueueBlocking submits a transaction and blocks until accepted.
func (p *IngestWorkerPool) QueueBlocking(tx *models.Transaction) {
	p.queue <- tx
}

// Stop gracefully shuts down workers by draining the queue first.
func (p *IngestWorkerPool) Stop() {
	close(p.queue)
	p.wg.Wait()
	p.cancel()
}

func (p *IngestWorkerPool) runWorker(id int) {
	defer p.wg.Done()
	for tx := range p.queue {
		if err := p.store.Create(p.ctx, tx); err != nil {
			log.Printf("worker %d: failed to create transaction: %v", id, err)
		}
	}
}
