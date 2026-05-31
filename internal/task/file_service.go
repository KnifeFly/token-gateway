package task

import (
	"context"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/egressguard"
)

// FileService owns file asset creation, idempotency, and quota reporting.
type FileService struct {
	repo     Repository
	ttl      time.Duration
	maxFiles int
	maxBytes int64
	egress   *egressguard.Guard
	now      func() time.Time
}

// FileServiceOption customizes FileService behavior.
type FileServiceOption func(*FileService)

// NewFileService returns a file service backed by repo.
func NewFileService(repo Repository, ttl time.Duration, options ...FileServiceOption) *FileService {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	service := &FileService{
		repo:     repo,
		ttl:      ttl,
		maxFiles: 1000,
		maxBytes: 100 << 30,
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

// WithFileEgressGuard validates file source URLs before registration.
func WithFileEgressGuard(guard *egressguard.Guard) FileServiceOption {
	return func(s *FileService) {
		s.egress = guard
	}
}

// FileCreateRequest contains normalized upload inputs.
type FileCreateRequest struct {
	TenantID       string
	ProjectID      string
	APIKeyID       string
	RequestID      string
	Endpoint       string
	IdempotencyKey string
	RequestBody    []byte
	FileName       string
	OriginalName   string
	SizeBytes      int64
	MIMEType       string
	UploadPath     string
	Source         string
	ContentHash    string
	SourceURL      string
}

// FindIdempotentFile returns an existing file for a matching idempotency key.
func (s *FileService) FindIdempotentFile(ctx context.Context, request FileCreateRequest) (*FileAsset, bool, error) {
	if s == nil || s.repo == nil || request.IdempotencyKey == "" {
		return nil, false, nil
	}
	file, record, ok, err := s.repo.GetFileByIdempotency(ctx, request.TenantID, request.APIKeyID, request.Endpoint, request.IdempotencyKey, s.now())
	if err != nil || !ok {
		return nil, false, err
	}
	if err := checkIdempotencyHash(record, requestHash(request.RequestBody)); err != nil {
		return nil, false, err
	}
	return file, true, nil
}

// CreateFile creates a normalized file asset.
func (s *FileService) CreateFile(ctx context.Context, request FileCreateRequest) (*FileAsset, bool, error) {
	if s == nil || s.repo == nil {
		return nil, false, apperr.ConfigUnavailable("file repository is unavailable")
	}
	if request.SizeBytes < 0 {
		return nil, false, apperr.InvalidArgument("file size is invalid")
	}
	if request.IdempotencyKey != "" {
		existing, hit, err := s.FindIdempotentFile(ctx, request)
		if err != nil || hit {
			return existing, hit, err
		}
	}
	now := s.now()
	fileName := sanitizeFileName(request.FileName)
	if fileName == "" {
		fileName = "upload"
	}
	originalName := sanitizeFileName(request.OriginalName)
	if originalName == "" {
		originalName = fileName
	}
	mimeType := request.MIMEType
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(fileName))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	sourceURL := strings.TrimSpace(request.SourceURL)
	if sourceURL != "" && s.egress != nil {
		if err := s.egress.ValidateURL(ctx, sourceURL); err != nil {
			return nil, false, apperr.InvalidArgument("file source url is not allowed", apperr.WithCause(err))
		}
	}
	expiresAt := now.Add(s.ttl)
	file := FileAsset{
		ID:           newID("file"),
		TenantID:     request.TenantID,
		ProjectID:    request.ProjectID,
		APIKeyID:     request.APIKeyID,
		RequestID:    request.RequestID,
		FileName:     fileName,
		OriginalName: originalName,
		SizeBytes:    request.SizeBytes,
		MIMEType:     mimeType,
		UploadPath:   strings.TrimSpace(request.UploadPath),
		FileURL:      sourceURL,
		Source:       request.Source,
		ContentHash:  strings.TrimSpace(request.ContentHash),
		SourceURL:    sourceURL,
		Transient:    true,
		CreatedAt:    now,
		ExpiresAt:    &expiresAt,
	}
	idem := newIdempotencyRecord(request.TenantID, request.APIKeyID, request.Endpoint, request.IdempotencyKey, requestHash(request.RequestBody), ResourceFile, s.ttl, now)
	created, err := s.repo.CreateFile(ctx, file, idem)
	return created, false, err
}

// Quota returns current file quota usage.
func (s *FileService) Quota(ctx context.Context, tenantID, projectID string) (FileQuota, error) {
	if s == nil || s.repo == nil {
		return FileQuota{}, apperr.ConfigUnavailable("file repository is unavailable")
	}
	return s.repo.FileQuota(ctx, tenantID, projectID, s.maxFiles, s.maxBytes)
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func detectMIME(sample []byte) string {
	if len(sample) == 0 {
		return ""
	}
	if len(sample) > 512 {
		sample = sample[:512]
	}
	return http.DetectContentType(sample)
}
