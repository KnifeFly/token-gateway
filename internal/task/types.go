package task

import (
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const (
	// StatusQueued marks a task accepted but not yet dispatched.
	StatusQueued Status = "queued"
	// StatusRunning marks a task dispatched to a provider.
	StatusRunning Status = "running"
	// StatusSucceeded marks a task completed successfully.
	StatusSucceeded Status = "succeeded"
	// StatusFailed marks a task completed with provider or gateway failure.
	StatusFailed Status = "failed"
	// StatusCanceled marks a task canceled before successful completion.
	StatusCanceled Status = "canceled"
	// StatusExpired marks a task that passed its retention or provider deadline.
	StatusExpired Status = "expired"

	// KindImageGeneration identifies image generation tasks.
	KindImageGeneration Kind = "image.generation"
	// KindImageEdit identifies image editing tasks.
	KindImageEdit Kind = "image.edit"
	// KindVideoGeneration identifies video generation tasks.
	KindVideoGeneration Kind = "video.generation"
	// KindAudioSpeech identifies text-to-speech tasks.
	KindAudioSpeech Kind = "audio.speech"
	// KindAudioTranscription identifies audio transcription tasks.
	KindAudioTranscription Kind = "audio.transcription"
	// KindMusicGeneration identifies music generation tasks.
	KindMusicGeneration Kind = "music.generation"
	// ResourceTask identifies task idempotency records.
	ResourceTask ResourceType = "task"
	// ResourceFile identifies file idempotency records.
	ResourceFile ResourceType = "file"
	// IdempotencyStatusReserved marks a key reserved before resource creation.
	IdempotencyStatusReserved IdempotencyStatus = "reserved"
	// IdempotencyStatusBound marks a key bound to a durable resource.
	IdempotencyStatusBound IdempotencyStatus = "bound"

	// CallbackStatusPending marks a callback waiting for delivery.
	CallbackStatusPending CallbackStatus = "pending"
	// CallbackStatusDelivered marks a callback delivered successfully.
	CallbackStatusDelivered CallbackStatus = "delivered"
	// CallbackStatusFailed marks a callback that failed and may retry.
	CallbackStatusFailed CallbackStatus = "failed"
)

// Status is the durable internal async task state.
type Status string

// Kind identifies the unified media operation behind a task.
type Kind string

// ResourceType identifies the resource bound to an idempotency key.
type ResourceType string

// IdempotencyStatus is the durable idempotency record state.
type IdempotencyStatus string

// CallbackStatus is the callback outbox delivery state.
type CallbackStatus string

// Task is the durable async task aggregate used by M4 media workflows.
type Task struct {
	ID             string
	TenantID       string
	ProjectID      string
	APIKeyID       string
	RequestID      string
	IdempotencyKey string
	RequestHash    string
	Kind           Kind
	MediaType      string
	Model          string
	Status         Status
	Progress       int
	ProviderType   string
	ChannelID      string
	ProviderTaskID string
	Input          json.RawMessage
	Result         json.RawMessage
	Usage          tokenusage.Actual
	ErrorCode      string
	ErrorMessage   string
	CallbackURL    string
	Metadata       map[string]string
	BalanceHoldID  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// IdempotencyRecord binds a client key and request hash to one resource.
type IdempotencyRecord struct {
	ID             string
	TenantID       string
	APIKeyID       string
	Endpoint       string
	IdempotencyKey string
	RequestHash    string
	ResourceType   ResourceType
	ResourceID     string
	Status         IdempotencyStatus
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FileAsset is the normalized file object used by media inputs and results.
type FileAsset struct {
	ID           string
	TenantID     string
	ProjectID    string
	APIKeyID     string
	RequestID    string
	FileName     string
	OriginalName string
	SizeBytes    int64
	MIMEType     string
	UploadPath   string
	FileURL      string
	DownloadURL  string
	Source       string
	ContentHash  string
	SourceURL    string
	Transient    bool
	CreatedAt    time.Time
	ExpiresAt    *time.Time
}

// FileQuota summarizes file storage usage for one project.
type FileQuota struct {
	MaxFiles       int
	UsedFiles      int
	RemainingFiles int
	MaxBytes       int64
	UsedBytes      int64
}

// CallbackEvent is a retryable callback outbox row.
type CallbackEvent struct {
	ID          string
	TaskID      string
	TenantID    string
	ProjectID   string
	URL         string
	Payload     json.RawMessage
	Status      CallbackStatus
	RetryCount  int
	NextRetryAt time.Time
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProviderTask is the upstream async task handle returned by a provider.
type ProviderTask struct {
	ExternalID       string
	Status           Status
	Progress         int
	ProviderMetadata map[string]string
}

// ProviderTaskResult is the normalized provider polling result.
type ProviderTaskResult struct {
	Status           Status
	Progress         int
	Result           json.RawMessage
	Assets           []ResultAsset
	Usage            tokenusage.Actual
	ErrorCode        string
	ErrorMessage     string
	ProviderMetadata map[string]string
}

// ResultAsset describes one provider-hosted media result.
type ResultAsset struct {
	URL       string            `json:"url"`
	Type      string            `json:"type,omitempty"`
	MIMEType  string            `json:"mime_type,omitempty"`
	Provider  string            `json:"provider,omitempty"`
	ExpiresAt string            `json:"expires_at,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
