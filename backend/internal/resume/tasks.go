package resume

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/config"
	"github.com/hibiken/asynq"
)

const (
	TaskTypeExtractText = "resume:extract_text"
	defaultTaskQueue    = "default"
)

type ExtractTextPayload struct {
	ResumeID string `json:"resume_id"`
}

type AsynqTextExtractionEnqueuer struct {
	client *asynq.Client
}

func NewAsynqTextExtractionEnqueuer(redis config.RedisConfig) *AsynqTextExtractionEnqueuer {
	return &AsynqTextExtractionEnqueuer{
		client: asynq.NewClient(asynq.RedisClientOpt{
			Addr:     redis.Addr,
			Password: redis.Password,
			DB:       redis.DB,
		}),
	}
}

func (e *AsynqTextExtractionEnqueuer) Close() error {
	return e.client.Close()
}

func (e *AsynqTextExtractionEnqueuer) EnqueueTextExtraction(ctx context.Context, resumeID string) error {
	resumeID = strings.TrimSpace(resumeID)
	if resumeID == "" {
		return ErrInvalidResumeID
	}

	payload, err := json.Marshal(ExtractTextPayload{ResumeID: resumeID})
	if err != nil {
		return fmt.Errorf("marshal text extraction task: %w", err)
	}

	task := asynq.NewTask(TaskTypeExtractText, payload,
		asynq.MaxRetry(5),
		asynq.Queue(defaultTaskQueue),
		asynq.Timeout(2*time.Minute),
	)
	if _, err := e.client.EnqueueContext(ctx, task); err != nil {
		return fmt.Errorf("enqueue text extraction task: %w", err)
	}
	return nil
}

func ParseExtractTextPayload(task *asynq.Task) (ExtractTextPayload, error) {
	var payload ExtractTextPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return ExtractTextPayload{}, fmt.Errorf("%w: decode text extraction payload: %v", asynq.SkipRetry, err)
	}
	payload.ResumeID = strings.TrimSpace(payload.ResumeID)
	if payload.ResumeID == "" {
		return ExtractTextPayload{}, fmt.Errorf("%w: %v", asynq.SkipRetry, ErrInvalidResumeID)
	}
	return payload, nil
}
