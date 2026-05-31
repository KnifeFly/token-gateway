package parser

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const maxBase64FileBytes = 4 << 20

func (p *Parser) parseUnifiedMedia(state *engine.RequestState, body []byte) error {
	var req unifiedMediaRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	if req.Model == "" {
		return apperr.InvalidArgument("model is required")
	}
	if err := validateMediaBody(state.CanonicalAPI, req); err != nil {
		return err
	}
	kind, mediaType := mediaKind(state.CanonicalAPI)
	state.RequestedModel = req.Model
	state.Async = true
	state.IdempotencyKey = strings.TrimSpace(state.Incoming.Header.Get("Idempotency-Key"))
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		Model:   req.Model,
		Media: &engine.UnifiedMediaRequest{
			Kind:        string(kind),
			MediaType:   mediaType,
			Model:       req.Model,
			CallbackURL: strings.TrimSpace(req.CallbackURL),
			Metadata:    req.Metadata,
			ModelParams: req.ModelParams,
		},
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

func (p *Parser) parseAudioTranscription(state *engine.RequestState, body []byte) error {
	model := ""
	callbackURL := ""
	metadata := map[string]string(nil)
	modelParams := map[string]any(nil)
	contentType := state.Incoming.Header.Get("Content-Type")
	mediaType, params, _ := mime.ParseMediaType(contentType)
	if strings.HasPrefix(mediaType, "multipart/") {
		form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(defaultMaxBodyBytes)
		if err != nil {
			return apperr.InvalidArgument("multipart body is invalid", apperr.WithCause(err))
		}
		defer func() { _ = form.RemoveAll() }()
		model = firstFormValue(form, "model")
		callbackURL = firstFormValue(form, "callback_url", "callbackUrl")
	} else {
		var req unifiedMediaRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
		}
		model = req.Model
		callbackURL = req.CallbackURL
		metadata = req.Metadata
		modelParams = req.ModelParams
	}
	if model == "" {
		return apperr.InvalidArgument("model is required")
	}
	state.RequestedModel = model
	state.Async = true
	state.IdempotencyKey = strings.TrimSpace(state.Incoming.Header.Get("Idempotency-Key"))
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		Model:   model,
		Media: &engine.UnifiedMediaRequest{
			Kind:        "audio.transcription",
			MediaType:   "audio",
			Model:       model,
			CallbackURL: strings.TrimSpace(callbackURL),
			Metadata:    metadata,
			ModelParams: modelParams,
		},
	}
	state.EstimatedUsage = tokenusage.EstimateFromBytes(body)
	return nil
}

func (p *Parser) parseTaskOperation(state *engine.RequestState) error {
	taskID := taskIDFromPath(state.Incoming.Path)
	if taskID == "" {
		return apperr.InvalidArgument("task_id is required")
	}
	operation := engine.TaskOperationGet
	if state.CanonicalAPI == engine.CanonicalTaskCancel {
		operation = engine.TaskOperationCancel
	}
	state.Parsed = engine.ParsedRequest{
		Task: &engine.TaskRequest{Operation: operation, TaskID: taskID},
	}
	return nil
}

func (p *Parser) parseFileQuota(state *engine.RequestState) error {
	state.Parsed = engine.ParsedRequest{
		File: &engine.FileRequest{Operation: engine.FileOperationQuota},
	}
	return nil
}

func (p *Parser) parseFileBase64(state *engine.RequestState, body []byte) error {
	var req base64FileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	if strings.TrimSpace(req.Base64Data) == "" {
		return apperr.InvalidArgument("base64_data is required")
	}
	content, mimeType, err := decodeBase64Data(req.Base64Data)
	if err != nil {
		return err
	}
	if len(content) > maxBase64FileBytes {
		return apperr.InvalidArgument("base64 input asset exceeds decoded size limit")
	}
	fileName := req.FileName
	if fileName == "" {
		fileName = "upload"
	}
	state.IdempotencyKey = strings.TrimSpace(state.Incoming.Header.Get("Idempotency-Key"))
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		File: &engine.FileRequest{
			Operation:    engine.FileOperationUploadBase64,
			FileName:     fileName,
			OriginalName: fileName,
			SizeBytes:    int64(len(content)),
			MIMEType:     mimeType,
			UploadPath:   req.UploadPath,
			ContentHash:  hashBytes(content),
		},
	}
	return nil
}

func (p *Parser) parseFileURL(state *engine.RequestState, body []byte) error {
	var req urlFileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return apperr.InvalidArgument("request body must be valid json", apperr.WithCause(err))
	}
	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return apperr.InvalidArgument("url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return apperr.InvalidArgument("url must use http or https")
	}
	fileName := req.FileName
	if fileName == "" {
		fileName = filepath.Base(parsed.Path)
	}
	if fileName == "." || fileName == "/" || fileName == "" {
		fileName = "remote"
	}
	mimeType := mime.TypeByExtension(filepath.Ext(fileName))
	state.IdempotencyKey = strings.TrimSpace(state.Incoming.Header.Get("Idempotency-Key"))
	state.Parsed = engine.ParsedRequest{
		RawBody: body,
		File: &engine.FileRequest{
			Operation:    engine.FileOperationUploadURL,
			FileName:     fileName,
			OriginalName: fileName,
			MIMEType:     mimeType,
			UploadPath:   req.UploadPath,
			SourceURL:    parsed.String(),
		},
	}
	return nil
}

func (p *Parser) parseFileStream(_ *engine.RequestState, _ []byte) error {
	return apperr.FeatureNotEnabled("stream upload is not enabled; use url or base64 transient input assets")
}

type unifiedMediaRequest struct {
	Model       string            `json:"model"`
	Prompt      string            `json:"prompt"`
	Input       string            `json:"input"`
	Voice       string            `json:"voice"`
	CallbackURL string            `json:"callback_url"`
	Metadata    map[string]string `json:"metadata"`
	ModelParams map[string]any    `json:"model_params"`
}

type base64FileRequest struct {
	Base64Data string `json:"base64_data"`
	UploadPath string `json:"upload_path"`
	FileName   string `json:"file_name"`
}

type urlFileRequest struct {
	URL        string `json:"url"`
	UploadPath string `json:"upload_path"`
	FileName   string `json:"file_name"`
}

func validateMediaBody(api engine.CanonicalAPI, req unifiedMediaRequest) error {
	switch api {
	case engine.CanonicalImageGeneration, engine.CanonicalImageEdit, engine.CanonicalVideoGeneration:
		if strings.TrimSpace(req.Prompt) == "" {
			return apperr.InvalidArgument("prompt is required")
		}
	case engine.CanonicalAudioSpeech:
		if strings.TrimSpace(req.Input) == "" {
			return apperr.InvalidArgument("input is required")
		}
		if strings.TrimSpace(req.Voice) == "" {
			return apperr.InvalidArgument("voice is required")
		}
	}
	return nil
}

func mediaKind(api engine.CanonicalAPI) (string, string) {
	switch api {
	case engine.CanonicalImageGeneration:
		return "image.generation", "image"
	case engine.CanonicalImageEdit:
		return "image.edit", "image"
	case engine.CanonicalVideoGeneration:
		return "video.generation", "video"
	case engine.CanonicalAudioSpeech:
		return "audio.speech", "audio"
	case engine.CanonicalMusicGeneration:
		return "music.generation", "audio"
	default:
		return string(api), "text"
	}
}

func taskIDFromPath(path string) string {
	path = strings.TrimPrefix(path, "/v1/tasks/")
	path = strings.TrimSuffix(path, "/cancel")
	return strings.Trim(path, "/")
}

func decodeBase64Data(value string) ([]byte, string, error) {
	value = strings.TrimSpace(value)
	mimeType := ""
	if strings.HasPrefix(value, "data:") {
		parts := strings.SplitN(value, ",", 2)
		if len(parts) != 2 {
			return nil, "", apperr.InvalidArgument("base64_data is invalid")
		}
		header := strings.TrimPrefix(parts[0], "data:")
		if idx := strings.Index(header, ";"); idx >= 0 {
			mimeType = header[:idx]
		}
		value = parts[1]
	}
	content, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, "", apperr.InvalidArgument("base64_data is invalid", apperr.WithCause(err))
	}
	if mimeType == "" {
		mimeType = detectMIME(content)
	}
	return content, mimeType, nil
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

func inspectMultipartFile(header *multipart.FileHeader) (int64, string, string, error) {
	file, err := header.Open()
	if err != nil {
		return 0, "", "", apperr.InvalidArgument("file could not be read", apperr.WithCause(err))
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return 0, "", "", apperr.InvalidArgument("file could not be read", apperr.WithCause(err))
	}
	mimeType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = detectMIME(content)
	}
	return int64(len(content)), mimeType, hashBytes(content), nil
}

func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func firstFormValue(form *multipart.Form, names ...string) string {
	for _, name := range names {
		values := form.Value[name]
		if len(values) > 0 && strings.TrimSpace(values[0]) != "" {
			return strings.TrimSpace(values[0])
		}
	}
	return ""
}
