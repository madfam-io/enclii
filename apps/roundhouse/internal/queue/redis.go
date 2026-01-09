package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	buildQueueKey    = "roundhouse:queue:builds"
	priorityQueueKey = "roundhouse:queue:priority"
	jobHashKeyPrefix = "roundhouse:job:"
	logsStreamPrefix = "roundhouse:logs:"
	statsKey         = "roundhouse:stats"
	activeWorkersKey = "roundhouse:workers:active"
)

// RedisQueue implements the build queue using Redis
type RedisQueue struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisQueue creates a new Redis-backed queue
func NewRedisQueue(redisURL string, logger *zap.Logger) (*RedisQueue, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &RedisQueue{
		client: client,
		logger: logger,
	}, nil
}

// Enqueue adds a build job to the queue
func (q *RedisQueue) Enqueue(ctx context.Context, job *BuildJob) error {
	job.ID = uuid.New()
	job.CreatedAt = time.Now()

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Store job details in hash
	jobKey := jobHashKeyPrefix + job.ID.String()
	if err := q.client.HSet(ctx, jobKey, map[string]interface{}{
		"data":       string(data),
		"status":     string(StatusQueued),
		"created_at": job.CreatedAt.Format(time.RFC3339),
	}).Err(); err != nil {
		return fmt.Errorf("failed to store job: %w", err)
	}

	// Set expiry for job data (7 days)
	q.client.Expire(ctx, jobKey, 7*24*time.Hour)

	// Add to queue (priority queue uses sorted set)
	if job.Priority > 0 {
		score := float64(time.Now().Unix()) - float64(job.Priority*1000)
		if err := q.client.ZAdd(ctx, priorityQueueKey, redis.Z{
			Score:  score,
			Member: job.ID.String(),
		}).Err(); err != nil {
			return fmt.Errorf("failed to enqueue priority job: %w", err)
		}
	} else {
		// Normal FIFO queue
		if err := q.client.LPush(ctx, buildQueueKey, job.ID.String()).Err(); err != nil {
			return fmt.Errorf("failed to enqueue job: %w", err)
		}
	}

	q.logger.Info("job enqueued",
		zap.String("job_id", job.ID.String()),
		zap.String("service_id", job.ServiceID.String()),
		zap.String("git_sha", job.GitSHA),
		zap.Int("priority", job.Priority),
	)

	return nil
}

// Dequeue retrieves the next job from the queue (blocking)
func (q *RedisQueue) Dequeue(ctx context.Context, timeout time.Duration) (*BuildJob, error) {
	// First check priority queue
	result, err := q.client.ZPopMin(ctx, priorityQueueKey, 1).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to check priority queue: %w", err)
	}

	var jobID string
	if len(result) > 0 {
		jobID = result[0].Member.(string)
	} else {
		// Fall back to regular queue with blocking pop
		res, err := q.client.BRPop(ctx, timeout, buildQueueKey).Result()
		if err != nil {
			if err == redis.Nil {
				return nil, nil // No job available
			}
			return nil, fmt.Errorf("failed to dequeue: %w", err)
		}
		jobID = res[1]
	}

	// Retrieve job data
	jobKey := jobHashKeyPrefix + jobID
	data, err := q.client.HGet(ctx, jobKey, "data").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get job data: %w", err)
	}

	var job BuildJob
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	// Update status to building
	q.client.HSet(ctx, jobKey, "status", string(StatusBuilding))

	return &job, nil
}

// UpdateStatus updates the status of a job
func (q *RedisQueue) UpdateStatus(ctx context.Context, jobID uuid.UUID, status JobStatus, workerID string) error {
	jobKey := jobHashKeyPrefix + jobID.String()

	updates := map[string]interface{}{
		"status":    string(status),
		"worker_id": workerID,
	}

	if status == StatusBuilding {
		updates["started_at"] = time.Now().Format(time.RFC3339)
	}

	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		updates["completed_at"] = time.Now().Format(time.RFC3339)
	}

	return q.client.HSet(ctx, jobKey, updates).Err()
}

// SetResult stores the build result
func (q *RedisQueue) SetResult(ctx context.Context, jobID uuid.UUID, result *BuildResult) error {
	jobKey := jobHashKeyPrefix + jobID.String()

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	return q.client.HSet(ctx, jobKey, "result", string(data)).Err()
}

// GetJob retrieves a job by ID
func (q *RedisQueue) GetJob(ctx context.Context, jobID uuid.UUID) (*BuildJob, JobStatus, error) {
	jobKey := jobHashKeyPrefix + jobID.String()

	result, err := q.client.HGetAll(ctx, jobKey).Result()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get job: %w", err)
	}

	if len(result) == 0 {
		return nil, "", fmt.Errorf("job not found: %s", jobID.String())
	}

	var job BuildJob
	if err := json.Unmarshal([]byte(result["data"]), &job); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, JobStatus(result["status"]), nil
}

// GetResult retrieves the build result for a job
func (q *RedisQueue) GetResult(ctx context.Context, jobID uuid.UUID) (*BuildResult, error) {
	jobKey := jobHashKeyPrefix + jobID.String()

	data, err := q.client.HGet(ctx, jobKey, "result").Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	var result BuildResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return &result, nil
}

// AppendLog adds a log line to the build logs stream
func (q *RedisQueue) AppendLog(ctx context.Context, jobID uuid.UUID, line string) error {
	streamKey := logsStreamPrefix + jobID.String()

	_, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"line":      line,
			"timestamp": time.Now().Format(time.RFC3339Nano),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to append log: %w", err)
	}

	// Set expiry on logs stream (7 days)
	q.client.Expire(ctx, streamKey, 7*24*time.Hour)

	return nil
}

// StreamLogs streams build logs from a job
func (q *RedisQueue) StreamLogs(ctx context.Context, jobID uuid.UUID, fromID string) (<-chan string, error) {
	streamKey := logsStreamPrefix + jobID.String()
	logChan := make(chan string, 100)

	if fromID == "" {
		fromID = "0"
	}

	go func() {
		defer close(logChan)

		lastID := fromID
		for {
			select {
			case <-ctx.Done():
				return
			default:
				result, err := q.client.XRead(ctx, &redis.XReadArgs{
					Streams: []string{streamKey, lastID},
					Count:   100,
					Block:   time.Second,
				}).Result()

				if err != nil {
					if err == redis.Nil {
						continue
					}
					q.logger.Error("failed to read logs", zap.Error(err))
					return
				}

				for _, stream := range result {
					for _, msg := range stream.Messages {
						lastID = msg.ID
						if line, ok := msg.Values["line"].(string); ok {
							logChan <- line
						}
					}
				}
			}
		}
	}()

	return logChan, nil
}

// QueueLength returns the number of jobs in the queue
func (q *RedisQueue) QueueLength(ctx context.Context) (int64, error) {
	regular, err := q.client.LLen(ctx, buildQueueKey).Result()
	if err != nil {
		return 0, err
	}

	priority, err := q.client.ZCard(ctx, priorityQueueKey).Result()
	if err != nil {
		return 0, err
	}

	return regular + priority, nil
}

// RegisterWorker registers a worker as active
func (q *RedisQueue) RegisterWorker(ctx context.Context, workerID string) error {
	return q.client.SAdd(ctx, activeWorkersKey, workerID).Err()
}

// UnregisterWorker removes a worker from active set
func (q *RedisQueue) UnregisterWorker(ctx context.Context, workerID string) error {
	return q.client.SRem(ctx, activeWorkersKey, workerID).Err()
}

// ActiveWorkers returns the list of active workers
func (q *RedisQueue) ActiveWorkers(ctx context.Context) ([]string, error) {
	return q.client.SMembers(ctx, activeWorkersKey).Result()
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
}
