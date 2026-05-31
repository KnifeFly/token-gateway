package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ProtocolMode describes the external API dialect selected for a request.
type ProtocolMode string

const (
	// ProtocolAuto defers protocol selection to classifier and config defaults.
	ProtocolAuto ProtocolMode = "auto"
	// ProtocolUnified selects the gateway-native unified API dialect.
	ProtocolUnified ProtocolMode = "unified"
	// ProtocolNativeOpenAI selects OpenAI-compatible native APIs.
	ProtocolNativeOpenAI ProtocolMode = "native_openai"
	// ProtocolNativeClaude selects Claude-compatible native APIs.
	ProtocolNativeClaude ProtocolMode = "native_claude"
	// ProtocolNativeGemini selects Gemini-compatible native APIs.
	ProtocolNativeGemini ProtocolMode = "native_gemini"
)

// CanonicalAPI identifies the normalized API operation.
type CanonicalAPI string

const (
	// CanonicalOpenAIChatCompletions identifies the OpenAI chat completions operation.
	CanonicalOpenAIChatCompletions CanonicalAPI = "openai.chat_completions"
	// CanonicalOpenAIResponses identifies the OpenAI responses operation.
	CanonicalOpenAIResponses CanonicalAPI = "openai.responses"
	// CanonicalOpenAIEmbeddings identifies the OpenAI embeddings operation.
	CanonicalOpenAIEmbeddings CanonicalAPI = "openai.embeddings"
	// CanonicalOpenAIModerations identifies the OpenAI moderations operation.
	CanonicalOpenAIModerations CanonicalAPI = "openai.moderations"
	// CanonicalClaudeMessages identifies the Claude messages operation.
	CanonicalClaudeMessages CanonicalAPI = "claude.messages"
	// CanonicalGeminiGenerateContent identifies the Gemini generateContent operation.
	CanonicalGeminiGenerateContent CanonicalAPI = "gemini.generate_content"
	// CanonicalImageGeneration identifies unified image generation.
	CanonicalImageGeneration CanonicalAPI = "unified.image_generation"
	// CanonicalImageEdit identifies unified image editing.
	CanonicalImageEdit CanonicalAPI = "unified.image_edit"
	// CanonicalVideoGeneration identifies unified video generation.
	CanonicalVideoGeneration CanonicalAPI = "unified.video_generation"
	// CanonicalAudioSpeech identifies unified text-to-speech.
	CanonicalAudioSpeech CanonicalAPI = "unified.audio_speech"
	// CanonicalAudioTranscription identifies unified audio transcription.
	CanonicalAudioTranscription CanonicalAPI = "unified.audio_transcription"
	// CanonicalMusicGeneration identifies unified music generation.
	CanonicalMusicGeneration CanonicalAPI = "unified.music_generation"
	// CanonicalTaskGet identifies task read operations.
	CanonicalTaskGet CanonicalAPI = "task.get"
	// CanonicalTaskCancel identifies task cancellation operations.
	CanonicalTaskCancel CanonicalAPI = "task.cancel"
	// CanonicalFileUploadBase64 identifies base64 file uploads.
	CanonicalFileUploadBase64 CanonicalAPI = "file.upload_base64"
	// CanonicalFileUploadURL identifies URL file uploads.
	CanonicalFileUploadURL CanonicalAPI = "file.upload_url"
	// CanonicalFileUploadStream identifies streamed file uploads.
	CanonicalFileUploadStream CanonicalAPI = "file.upload_stream"
	// CanonicalFileQuota identifies file quota checks.
	CanonicalFileQuota CanonicalAPI = "file.quota"
)

// TaskOperation identifies a task read/control operation.
type TaskOperation string

const (
	// TaskOperationGet reads async task state.
	TaskOperationGet TaskOperation = "get"
	// TaskOperationCancel requests async task cancellation.
	TaskOperationCancel TaskOperation = "cancel"
)

// FileOperation identifies a file operation.
type FileOperation string

const (
	// FileOperationUploadBase64 uploads a file using an inline base64 body.
	FileOperationUploadBase64 FileOperation = "upload_base64"
	// FileOperationUploadURL imports a file from a remote URL.
	FileOperationUploadURL FileOperation = "upload_url"
	// FileOperationUploadStream uploads a file from a request stream.
	FileOperationUploadStream FileOperation = "upload_stream"
	// FileOperationQuota reads current file quota state.
	FileOperationQuota FileOperation = "quota"
)

// EndpointSpec records the matched public endpoint.
type EndpointSpec struct {
	Method      string
	Path        string
	Canonical   CanonicalAPI
	AllowedMode []ProtocolMode
}

// IncomingRequest is the transport-neutral request shape consumed by the engine.
type IncomingRequest struct {
	Method        string
	Path          string
	RawQuery      string
	Header        http.Header
	Body          io.ReadCloser
	RemoteAddr    string
	ContentLength int64
}

// GatewayResponse is the transport-neutral response shape returned by the engine.
type GatewayResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Stream     relay.ProviderStream
	Usage      tokenusage.Actual
}

// SnapshotRef pins the runtime snapshot version used for one request.
type SnapshotRef struct {
	Version   string
	CreatedAt time.Time
}

// SnapshotView is the indexed read-only runtime view used by the hot path.
type SnapshotView interface {
	Ref() SnapshotRef
	ListModels() []ModelView
	LookupAPIKeyHash(hash string) (APIKeyView, bool)
	LookupModel(publicModel string) (ModelView, bool)
	LookupRoute(publicModel string) (RoutePolicyView, bool)
	LookupChannel(channelID string) (ChannelView, bool)
	LookupPrice(publicModel string) (PriceRuleView, bool)
	LookupLimit(publicModel string) (LimitRuleView, bool)
	LookupLimits(scope LimitScope) []LimitRuleView
	LookupPluginBindings(phase string) []PluginBindingView
	IsAPIKeyRevoked(hash string) bool
}

// APIKeyView is the indexed API key view used by authentication.
type APIKeyView struct {
	ID            string
	TenantID      string
	ProjectID     string
	Name          string
	Hash          string
	Enabled       bool
	AllowedModels []string
}

// ModelView is the indexed public model view used by routing.
type ModelView struct {
	PublicModel      string
	Aliases          []string
	DisplayName      string
	Description      string
	Protocol         ProtocolMode
	Capability       string
	Schema           json.RawMessage
	ProviderMappings []ProviderModelMapping
	Enabled          bool
}

// ChannelView is the indexed provider channel view used by dispatch.
type ChannelView struct {
	ID              string
	ProviderType    string
	BaseURL         string
	APIKey          string
	CredentialRef   string
	EncryptedAPIKey string
	Enabled         bool
	Timeout         time.Duration
	Models          map[string]string
}

// RoutePolicyView is the indexed route policy view used by routing.
type RoutePolicyView struct {
	ID          string
	PublicModel string
	Strategy    string
	Candidates  []RouteCandidateView
}

// RouteCandidateView is one snapshot route candidate before resolution.
type RouteCandidateView struct {
	ChannelID string
	Priority  int
	Weight    int
}

// PriceRuleView is the pinned customer-facing model price.
type PriceRuleView struct {
	PublicModel           string
	Currency              string
	InputMicrosPerToken   int64
	OutputMicrosPerToken  int64
	EstimatedOutputTokens int64
	Enabled               bool
}

// LimitScope identifies the dimensions used by runtime limit rules.
type LimitScope struct {
	TenantID     string
	ProjectID    string
	APIKeyID     string
	PublicModel  string
	ProviderType string
	ChannelID    string
}

// LimitRuleView is the pinned multi-dimensional request limit config.
type LimitRuleView struct {
	ID                  string
	Scope               LimitScope
	RPM                 int64
	QPS                 int64
	TPM                 int64
	Concurrency         int64
	DailyBudgetMicros   int64
	CostPerMinuteMicros int64
	Enabled             bool
}

// ProviderModelMapping describes one catalog-owned provider model mapping.
type ProviderModelMapping struct {
	ProviderType  string
	ChannelID     string
	PublicModel   string
	UpstreamModel string
}

// PluginBindingView is a runtime plugin binding compiled from control-plane config.
type PluginBindingView struct {
	ID            string
	Name          string
	Phase         string
	TenantID      string
	ProjectID     string
	Model         string
	Priority      int
	Enabled       bool
	FailurePolicy string
	Config        json.RawMessage
}

// Principal is the authenticated caller identity.
type Principal struct {
	TenantID      string
	ProjectID     string
	APIKeyID      string
	AllowedModels []string
}

// ParsedRequest is the normalized request body extracted by parsers.
type ParsedRequest struct {
	RawBody        []byte
	Model          string
	Stream         bool
	OpenAIChat     *OpenAIChatRequest
	OpenAIResponse *OpenAIResponseRequest
	Embedding      *EmbeddingRequest
	Moderation     *ModerationRequest
	ClaudeMessage  *ClaudeMessageRequest
	Gemini         *GeminiRequest
	Media          *UnifiedMediaRequest
	Task           *TaskRequest
	File           *FileRequest
}

// OpenAIChatRequest contains M1 fields needed from an OpenAI-compatible chat request.
type OpenAIChatRequest struct {
	Model    string
	Messages []OpenAIChatMessage
	Stream   bool
}

// OpenAIResponseRequest contains M3 fields needed from a Responses request.
type OpenAIResponseRequest struct {
	Model  string
	Stream bool
}

// EmbeddingRequest contains M3 fields needed from an embeddings request.
type EmbeddingRequest struct {
	Model string
}

// ModerationRequest contains OpenAI-compatible moderation request fields.
type ModerationRequest struct {
	Model string
}

// ClaudeMessageRequest contains M3 fields needed from a Claude messages request.
type ClaudeMessageRequest struct {
	Model  string
	Stream bool
}

// GeminiRequest contains M3 fields needed from a Gemini generateContent request.
type GeminiRequest struct {
	Model  string
	Stream bool
}

// UnifiedMediaRequest contains M4 async media task fields.
type UnifiedMediaRequest struct {
	Kind        string
	MediaType   string
	Model       string
	CallbackURL string
	Metadata    map[string]string
	ModelParams map[string]any
}

// TaskRequest contains M4 task query/control fields.
type TaskRequest struct {
	Operation TaskOperation
	TaskID    string
}

// FileRequest contains M4 file upload/quota fields.
type FileRequest struct {
	Operation    FileOperation
	FileName     string
	OriginalName string
	SizeBytes    int64
	MIMEType     string
	UploadPath   string
	ContentHash  string
	SourceURL    string
}

// OpenAIChatMessage is a minimal OpenAI-compatible chat message.
type OpenAIChatMessage struct {
	Role    string
	Content any
}

// RoutePlan is the ordered provider attempt plan for one request.
type RoutePlan struct {
	PolicyID   string
	Candidates []ProviderCandidate
}

// ProviderCandidate is a resolved route candidate.
type ProviderCandidate struct {
	ChannelID     string
	ProviderType  string
	PublicModel   string
	UpstreamModel string
	Priority      int
	Weight        int
	Timeout       time.Duration
}

// ProviderAttempt records one upstream attempt without sensitive data.
type ProviderAttempt struct {
	AttemptIndex          int
	ChannelID             string
	ProviderType          string
	PublicModel           string
	StatusCode            int
	ErrorCode             string
	Success               bool
	StartedAt             time.Time
	Duration              time.Duration
	Retryable             bool
	RetryBudgetConsumed   int
	RetryBudgetRemaining  int
	FallbackFromChannelID string
	FallbackFromProvider  string
	CircuitState          string
	Final                 bool
}

// ProviderResult is the successful provider dispatch result.
type ProviderResult struct {
	Candidate ProviderCandidate
	Response  *GatewayResponse
	Usage     tokenusage.Actual
}

// PolicyAction is the normalized result of data-plane policy evaluation.
type PolicyAction string

const (
	// PolicyAllow permits the request to continue unchanged.
	PolicyAllow PolicyAction = "allow"
	// PolicyDeny blocks the request.
	PolicyDeny PolicyAction = "deny"
	// PolicyDegrade changes the request to a different model.
	PolicyDegrade PolicyAction = "degrade"
	// PolicyRouteOverride replaces the selected provider route.
	PolicyRouteOverride PolicyAction = "route_override"
)

// PolicyDecision is consumed by GatewayEngine before route dispatch.
type PolicyDecision struct {
	Action       PolicyAction
	Reason       string
	DegradeModel string
	RoutePlan    *RoutePlan
	Metadata     map[string]string
}

// SnapshotProvider attaches the current indexed snapshot to a request.
type SnapshotProvider interface {
	Attach(ctx context.Context, state *RequestState) error
}

// APIClassifier selects the canonical API for a request.
type APIClassifier interface {
	Classify(ctx context.Context, state *RequestState) error
}

// RequestParser parses the request body into normalized fields.
type RequestParser interface {
	Parse(ctx context.Context, state *RequestState) error
}

// Authenticator authenticates the caller against the pinned snapshot.
type Authenticator interface {
	Authenticate(ctx context.Context, state *RequestState) error
}

// PolicyEvaluator returns one explicit policy decision for the request.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, state *RequestState) (PolicyDecision, error)
}

// RoutePlanner resolves a model and ordered route candidates.
type RoutePlanner interface {
	Plan(ctx context.Context, state *RequestState) error
}

// ProviderDispatcher calls provider adapters according to the route plan.
type ProviderDispatcher interface {
	Dispatch(ctx context.Context, state *RequestState) (*ProviderResult, error)
}

// AdmissionController reserves balance before the provider is called.
type AdmissionController interface {
	Reserve(ctx context.Context, state *RequestState) error
	Release(ctx context.Context, state *RequestState, cause error) error
}

// LimitEnforcer acquires distributed request limits.
type LimitEnforcer interface {
	Acquire(ctx context.Context, state *RequestState) (LimitRelease, error)
}

// LimitRelease releases a previously acquired limit lease.
type LimitRelease interface {
	Release(ctx context.Context) error
}

// LimitRenewal keeps a long-running limit lease alive until release.
type LimitRenewal interface {
	Renew(ctx context.Context) error
	RenewalInterval() time.Duration
}

// StreamFinalizer wraps provider streams and performs close-time accounting.
type StreamFinalizer interface {
	Wrap(ctx context.Context, state *RequestState, result *ProviderResult) (*GatewayResponse, error)
}

// TaskBridge handles async task idempotency, creation, query, and cancel operations.
type TaskBridge interface {
	CheckIdempotency(ctx context.Context, state *RequestState) (*GatewayResponse, bool, error)
	CreateAndDispatch(ctx context.Context, state *RequestState) (*GatewayResponse, bool, error)
	HandleTaskOperation(ctx context.Context, state *RequestState) (*GatewayResponse, error)
}

// FileService handles file uploads and quota operations.
type FileService interface {
	HandleFileOperation(ctx context.Context, state *RequestState) (*GatewayResponse, error)
}

// SettlementService performs final usage settlement after provider success.
type SettlementService interface {
	Settle(ctx context.Context, state *RequestState) error
	RecordFailed(ctx context.Context, state *RequestState, cause error) error
}

// ObserveRecorder records hot-path metrics, traces, and logs.
type ObserveRecorder interface {
	StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span)
	RecordProviderAttempt(ctx context.Context, state *RequestState, attempt ProviderAttempt)
	FinishRequest(ctx context.Context, state *RequestState, response *GatewayResponse, err error)
}

// PluginManager executes configured data-plane plugins for a lifecycle phase.
type PluginManager interface {
	Run(ctx context.Context, phase string, state *RequestState) error
}
