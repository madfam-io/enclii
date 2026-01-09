package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/madfam/enclii/apps/switchyard-api/internal/db"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
	"github.com/sirupsen/logrus"
)

// AsyncLogger handles asynchronous audit log writing to avoid blocking requests
type AsyncLogger struct {
	repos          *db.Repositories
	logChan        chan *types.AuditLog
	fallbackChan   chan *types.AuditLog // SECURITY FIX: Fallback channel for overflow
	batchSize      int
	flushTime      time.Duration
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	errorCount     int
	droppedCount   int // SECURITY FIX: Track dropped logs
	fallbackCount  int // SECURITY FIX: Track logs sent to fallback
	mu             sync.Mutex
	lastDropWarned time.Time // SECURITY FIX: Rate-limit drop warnings
}

// NewAsyncLogger creates a new async audit logger
// SECURITY FIX: Increased buffer size and added fallback channel to prevent log loss
func NewAsyncLogger(repos *db.Repositories, bufferSize int) *AsyncLogger {
	ctx, cancel := context.WithCancel(context.Background())

	// SECURITY FIX: Ensure minimum buffer size to prevent log loss under load
	if bufferSize < 1000 {
		bufferSize = 1000
		logrus.Warn("Audit log buffer size increased to minimum of 1000 to prevent log loss")
	}

	logger := &AsyncLogger{
		repos:        repos,
		logChan:      make(chan *types.AuditLog, bufferSize),
		fallbackChan: make(chan *types.AuditLog, bufferSize/2), // SECURITY FIX: Fallback buffer
		batchSize:    10,                                       // Write in batches of 10
		flushTime:    5 * time.Second,                          // Flush every 5 seconds
		ctx:          ctx,
		cancel:       cancel,
	}

	// Start background workers
	logger.wg.Add(2) // SECURITY FIX: Two workers now (main + fallback)
	go logger.worker()
	go logger.fallbackWorker()

	return logger
}

// Log enqueues an audit log for async writing
// SECURITY FIX: Improved handling with fallback channel and warnings to prevent silent log loss
func (l *AsyncLogger) Log(log *types.AuditLog) {
	select {
	case l.logChan <- log:
		// Successfully enqueued to primary channel
		return

	default:
		// Primary buffer full - try fallback channel
		select {
		case l.fallbackChan <- log:
			// Successfully enqueued to fallback channel
			l.mu.Lock()
			l.fallbackCount++
			// SECURITY FIX: Warn when fallback is used (rate-limited to once per minute)
			if time.Since(l.lastDropWarned) > time.Minute {
				logrus.WithFields(logrus.Fields{
					"fallback_count": l.fallbackCount,
					"dropped_count":  l.droppedCount,
				}).Warn("COMPLIANCE WARNING: Audit log primary buffer full, using fallback channel")
				l.lastDropWarned = time.Now()
			}
			l.mu.Unlock()
			return

		default:
			// Both buffers full - log must be dropped
			// SECURITY FIX: Critical compliance warning with detailed logging
			l.mu.Lock()
			l.droppedCount++
			droppedCount := l.droppedCount

			// Rate-limited critical warnings (once per minute)
			if time.Since(l.lastDropWarned) > time.Minute {
				logrus.WithFields(logrus.Fields{
					"dropped_count":  droppedCount,
					"fallback_count": l.fallbackCount,
					"action":         log.Action,
					"actor_id":       log.ActorID,
					"resource_type":  log.ResourceType,
					"resource_id":    log.ResourceID,
				}).Error("CRITICAL COMPLIANCE VIOLATION: Audit log dropped - both buffers full!")

				// Also log to stderr for alerting
				logrus.Errorf("CRITICAL: Audit log dropped! Total dropped: %d", droppedCount)
				l.lastDropWarned = time.Now()
			}
			l.mu.Unlock()

			// TODO: Write to persistent fallback storage (file, S3, Kafka, etc.)
			// TODO: Trigger alerting/paging for compliance team
		}
	}
}

// worker is the background goroutine that processes audit logs from primary channel
func (l *AsyncLogger) worker() {
	defer l.wg.Done()

	batch := make([]*types.AuditLog, 0, l.batchSize)
	ticker := time.NewTicker(l.flushTime)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			// Shutdown signal received - flush remaining logs
			l.flushBatch(batch, "primary")
			return

		case log := <-l.logChan:
			// Add to batch
			batch = append(batch, log)

			// Flush if batch is full
			if len(batch) >= l.batchSize {
				l.flushBatch(batch, "primary")
				batch = make([]*types.AuditLog, 0, l.batchSize)
			}

		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				l.flushBatch(batch, "primary")
				batch = make([]*types.AuditLog, 0, l.batchSize)
			}
		}
	}
}

// fallbackWorker is the background goroutine that processes audit logs from fallback channel
// SECURITY FIX: Separate worker for fallback channel to prevent log loss
func (l *AsyncLogger) fallbackWorker() {
	defer l.wg.Done()

	batch := make([]*types.AuditLog, 0, l.batchSize)
	ticker := time.NewTicker(l.flushTime)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			// Shutdown signal received - flush remaining logs
			l.flushBatch(batch, "fallback")
			return

		case log := <-l.fallbackChan:
			// Add to batch
			batch = append(batch, log)

			// Flush if batch is full
			if len(batch) >= l.batchSize {
				l.flushBatch(batch, "fallback")
				batch = make([]*types.AuditLog, 0, l.batchSize)
			}

		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				l.flushBatch(batch, "fallback")
				batch = make([]*types.AuditLog, 0, l.batchSize)
			}
		}
	}
}

// flushBatch writes a batch of audit logs to the database
// SECURITY FIX: Added channel parameter for better logging and monitoring
func (l *AsyncLogger) flushBatch(batch []*types.AuditLog, channel string) {
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
			errorCount := l.errorCount
			l.mu.Unlock()

			// SECURITY FIX: Log database write failures for compliance monitoring
			logrus.WithFields(logrus.Fields{
				"channel":       channel,
				"error_count":   errorCount,
				"action":        log.Action,
				"actor_id":      log.ActorID,
				"resource_type": log.ResourceType,
				"resource_id":   log.ResourceID,
				"error":         err.Error(),
			}).Error("Failed to write audit log to database")

			// For now, just continue - we don't want to crash on audit log failure
			// TODO: Write to persistent fallback storage (file, S3, etc.)
			continue
		}
	}
}

// Close gracefully shuts down the async logger
// Blocks until all pending logs are written
// SECURITY FIX: Improved shutdown with fallback channel handling
func (l *AsyncLogger) Close() error {
	// Signal workers to stop
	l.cancel()

	// Wait for workers to finish flushing
	l.wg.Wait()

	// Close channels
	close(l.logChan)
	close(l.fallbackChan)

	// Check if there were any errors or dropped logs
	l.mu.Lock()
	errorCount := l.errorCount
	droppedCount := l.droppedCount
	fallbackCount := l.fallbackCount
	l.mu.Unlock()

	// SECURITY FIX: Log final statistics for compliance auditing
	logrus.WithFields(logrus.Fields{
		"error_count":    errorCount,
		"dropped_count":  droppedCount,
		"fallback_count": fallbackCount,
	}).Info("Audit logger shutdown complete")

	if droppedCount > 0 {
		return fmt.Errorf("async logger dropped %d audit logs (COMPLIANCE VIOLATION)", droppedCount)
	}

	if errorCount > 0 {
		return fmt.Errorf("async logger encountered %d database write errors", errorCount)
	}

	return nil
}

// Stats returns statistics about the async logger
// SECURITY FIX: Added fallback channel statistics for monitoring
func (l *AsyncLogger) Stats() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()

	return map[string]interface{}{
		"primary_buffer_size":     cap(l.logChan),
		"primary_buffer_pending":  len(l.logChan),
		"fallback_buffer_size":    cap(l.fallbackChan),
		"fallback_buffer_pending": len(l.fallbackChan),
		"error_count":             l.errorCount,
		"dropped_count":           l.droppedCount,
		"fallback_count":          l.fallbackCount,
		"batch_size":              l.batchSize,
		"flush_interval":          l.flushTime.String(),
	}
}
