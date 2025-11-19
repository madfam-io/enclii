package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// AsyncLogger handles asynchronous audit log writing to avoid blocking requests
type AsyncLogger struct {
	repos      *db.Repositories
	logChan    chan *types.AuditLog
	batchSize  int
	flushTime  time.Duration
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	errorCount int
	mu         sync.Mutex
}

// NewAsyncLogger creates a new async audit logger
func NewAsyncLogger(repos *db.Repositories, bufferSize int) *AsyncLogger {
	ctx, cancel := context.WithCancel(context.Background())

	logger := &AsyncLogger{
		repos:     repos,
		logChan:   make(chan *types.AuditLog, bufferSize),
		batchSize: 10,                // Write in batches of 10
		flushTime: 5 * time.Second,   // Flush every 5 seconds
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start background worker
	logger.wg.Add(1)
	go logger.worker()

	return logger
}

// Log enqueues an audit log for async writing
// Non-blocking - drops logs if buffer is full to prevent request slowdown
func (l *AsyncLogger) Log(log *types.AuditLog) {
	select {
	case l.logChan <- log:
		// Successfully enqueued
	default:
		// Buffer full - log dropped to prevent blocking
		// In production, this should increment a metric/alert
		l.mu.Lock()
		l.errorCount++
		l.mu.Unlock()
	}
}

// worker is the background goroutine that processes audit logs
func (l *AsyncLogger) worker() {
	defer l.wg.Done()

	batch := make([]*types.AuditLog, 0, l.batchSize)
	ticker := time.NewTicker(l.flushTime)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			// Shutdown signal received - flush remaining logs
			l.flushBatch(batch)
			return

		case log := <-l.logChan:
			// Add to batch
			batch = append(batch, log)

			// Flush if batch is full
			if len(batch) >= l.batchSize {
				l.flushBatch(batch)
				batch = make([]*types.AuditLog, 0, l.batchSize)
			}

		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				l.flushBatch(batch)
				batch = make([]*types.AuditLog, 0, l.batchSize)
			}
		}
	}
}

// flushBatch writes a batch of audit logs to the database
func (l *AsyncLogger) flushBatch(batch []*types.AuditLog) {
	if len(batch) == 0 {
		return
	}

	// Create a context with timeout for database operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Write logs individually (could be optimized with batch insert)
	for _, log := range batch {
		if err := l.repos.AuditLogs.Log(ctx, log); err != nil {
			// Log write failed - in production, should:
			// 1. Increment error metric
			// 2. Write to fallback storage (file, S3, etc.)
			// 3. Alert if error rate is high
			l.mu.Lock()
			l.errorCount++
			l.mu.Unlock()

			// For now, just continue - we don't want to crash on audit log failure
			continue
		}
	}
}

// Close gracefully shuts down the async logger
// Blocks until all pending logs are written
func (l *AsyncLogger) Close() error {
	// Signal worker to stop
	l.cancel()

	// Wait for worker to finish flushing
	l.wg.Wait()

	// Close channel
	close(l.logChan)

	// Check if there were any errors
	l.mu.Lock()
	errorCount := l.errorCount
	l.mu.Unlock()

	if errorCount > 0 {
		return fmt.Errorf("async logger encountered %d errors during operation", errorCount)
	}

	return nil
}

// Stats returns statistics about the async logger
func (l *AsyncLogger) Stats() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()

	return map[string]interface{}{
		"buffer_size":    cap(l.logChan),
		"buffer_pending": len(l.logChan),
		"error_count":    l.errorCount,
		"batch_size":     l.batchSize,
		"flush_interval": l.flushTime.String(),
	}
}
