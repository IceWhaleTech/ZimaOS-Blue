package server

import (
	"sync"
	"time"
)

// BatchProcessor processes requests in batches for efficiency
type BatchProcessor struct {
	batchSize    int
	batchTimeout time.Duration
	queue        chan interface{}
	processor    func([]interface{}) error
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(batchSize int, batchTimeout time.Duration, processor func([]interface{}) error) *BatchProcessor {
	bp := &BatchProcessor{
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		queue:        make(chan interface{}, batchSize*2),
		processor:    processor,
		stopChan:     make(chan struct{}),
	}

	bp.wg.Add(1)
	go bp.processBatches()

	return bp
}

// Add adds an item to the batch queue
func (bp *BatchProcessor) Add(item interface{}) error {
	select {
	case bp.queue <- item:
		return nil
	case <-bp.stopChan:
		return ErrProcessorStopped
	}
}

// processBatches processes items in batches
func (bp *BatchProcessor) processBatches() {
	defer bp.wg.Done()

	batch := make([]interface{}, 0, bp.batchSize)
	ticker := time.NewTicker(bp.batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case item := <-bp.queue:
			batch = append(batch, item)
			if len(batch) >= bp.batchSize {
				if err := bp.processor(batch); err != nil {
					// Log error but continue processing
				}
				batch = make([]interface{}, 0, bp.batchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				if err := bp.processor(batch); err != nil {
					// Log error but continue processing
				}
				batch = make([]interface{}, 0, bp.batchSize)
			}

		case <-bp.stopChan:
			// Process remaining items
			if len(batch) > 0 {
				bp.processor(batch)
			}
			return
		}
	}
}

// Stop stops the batch processor
func (bp *BatchProcessor) Stop() {
	close(bp.stopChan)
	bp.wg.Wait()
}

// Error definitions
var (
	ErrProcessorStopped = &ProcessorError{Code: "PROCESSOR_STOPPED", Message: "batch processor has been stopped"}
)

// ProcessorError represents a processor error
type ProcessorError struct {
	Code    string
	Message string
}

func (e *ProcessorError) Error() string {
	return e.Message
}
