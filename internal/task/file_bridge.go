package task

import (
	"context"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// FileBridge connects file operations to the data-plane engine.
type FileBridge struct {
	service *FileService
}

// NewFileBridge returns a file bridge.
func NewFileBridge(service *FileService) *FileBridge {
	return &FileBridge{service: service}
}

// HandleFileOperation handles upload and quota operations after authentication.
func (b *FileBridge) HandleFileOperation(ctx context.Context, state *engine.RequestState) (*engine.GatewayResponse, error) {
	if b == nil || b.service == nil {
		return nil, apperr.ConfigUnavailable("file bridge is unavailable")
	}
	if state.Parsed.File == nil {
		return nil, apperr.InvalidArgument("file operation is required")
	}
	if state.Parsed.File.Operation == engine.FileOperationQuota {
		quota, err := b.service.Quota(ctx, state.TenantID, state.ProjectID)
		if err != nil {
			return nil, err
		}
		return FileQuotaResponse(quota)
	}
	request := FileCreateRequest{
		TenantID:       state.TenantID,
		ProjectID:      state.ProjectID,
		APIKeyID:       state.APIKeyID,
		RequestID:      state.RequestID,
		Endpoint:       state.Endpoint.Path,
		IdempotencyKey: state.IdempotencyKey,
		RequestBody:    state.Parsed.RawBody,
		FileName:       state.Parsed.File.FileName,
		OriginalName:   state.Parsed.File.OriginalName,
		SizeBytes:      state.Parsed.File.SizeBytes,
		MIMEType:       state.Parsed.File.MIMEType,
		UploadPath:     state.Parsed.File.UploadPath,
		Source:         string(state.Parsed.File.Operation),
		ContentHash:    state.Parsed.File.ContentHash,
		SourceURL:      state.Parsed.File.SourceURL,
	}
	file, _, err := b.service.CreateFile(ctx, request)
	if err != nil {
		return nil, err
	}
	return FileResponse(file)
}
