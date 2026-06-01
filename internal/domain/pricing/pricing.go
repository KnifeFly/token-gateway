package pricing

import (
	"fmt"
	"strings"

	"github.com/KnifeFly/token-gateway/pkg/money"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

const (
	// CategoryChat covers chat, responses, Claude messages, and Gemini content.
	CategoryChat Category = "chat"
	// CategoryEmbedding covers embedding vector generation.
	CategoryEmbedding Category = "embedding"
	// CategoryRerank covers reranking operations.
	CategoryRerank Category = "rerank"
	// CategoryImage covers image generation and editing tasks.
	CategoryImage Category = "image"
	// CategoryVideo covers video generation tasks.
	CategoryVideo Category = "video"
	// CategoryAudioSpeech covers text-to-speech tasks.
	CategoryAudioSpeech Category = "audio_speech"
	// CategoryAudioTranscription covers audio transcription tasks.
	CategoryAudioTranscription Category = "audio_transcription"
	// CategoryMusic covers music generation tasks.
	CategoryMusic Category = "music"
	// CategoryModeration covers moderation classification requests.
	CategoryModeration Category = "moderation"
	// CategoryRealtimeReserved is reserved for realtime session pricing.
	CategoryRealtimeReserved Category = "realtime_reserved"

	// UnitInputToken prices input text tokens.
	UnitInputToken Unit = "input_token"
	// UnitOutputToken prices output text tokens.
	UnitOutputToken Unit = "output_token"
	// UnitCacheReadToken prices cache-read tokens.
	UnitCacheReadToken Unit = "cache_read_token"
	// UnitCacheWriteToken prices cache-write tokens.
	UnitCacheWriteToken Unit = "cache_write_token"
	// UnitReasoningToken prices reasoning tokens.
	UnitReasoningToken Unit = "reasoning_token"
	// UnitAudioInputToken prices audio input tokens.
	UnitAudioInputToken Unit = "audio_input_token"
	// UnitAudioOutputToken prices audio output tokens.
	UnitAudioOutputToken Unit = "audio_output_token"
	// UnitImageInputToken prices image input tokens.
	UnitImageInputToken Unit = "image_input_token"
	// UnitImageOutputToken prices image output tokens.
	UnitImageOutputToken Unit = "image_output_token"
	// UnitVideoInputToken prices video input tokens.
	UnitVideoInputToken Unit = "video_input_token"
	// UnitVideoOutputToken prices video output tokens.
	UnitVideoOutputToken Unit = "video_output_token"
	// UnitRequest prices each API request.
	UnitRequest Unit = "request"
	// UnitImage prices each generated or processed image.
	UnitImage Unit = "image"
	// UnitAudioSecond prices audio duration in seconds.
	UnitAudioSecond Unit = "audio_second"
	// UnitVideoSecond prices video duration in seconds.
	UnitVideoSecond Unit = "video_second"
	// UnitTask prices each durable async task.
	UnitTask Unit = "task"
)

// Category classifies a public model for pricing templates and catalog grouping.
type Category string

// Unit identifies the measured dimension for one price component.
type Unit string

// Component prices one measured unit in micros.
type Component struct {
	Unit          Unit  `json:"unit"`
	MicrosPerUnit int64 `json:"micros_per_unit"`
}

// Template declares the component units supported by one model category.
type Template struct {
	Category Category
	Units    []Unit
}

// PriceBook is the normalized customer or provider price component set.
type PriceBook struct {
	Category   Category
	Currency   string
	Components []Component
}

// MeteredUsage is the pricing-domain usage vector understood by PriceBook.
type MeteredUsage struct {
	InputTokens       int64
	OutputTokens      int64
	CacheReadTokens   int64
	CacheWriteTokens  int64
	ReasoningTokens   int64
	AudioInputTokens  int64
	AudioOutputTokens int64
	ImageInputTokens  int64
	ImageOutputTokens int64
	VideoInputTokens  int64
	VideoOutputTokens int64
	Requests          int64
	Images            int64
	AudioSeconds      int64
	VideoSeconds      int64
	Tasks             int64
}

// TokenPrice stores customer-facing token prices in micros per token.
type TokenPrice struct {
	Currency             string
	InputMicrosPerToken  int64
	OutputMicrosPerToken int64
}

// QuoteEstimate quotes a pre-provider usage estimate.
func (p TokenPrice) QuoteEstimate(usage tokenusage.Estimate) money.Amount {
	return money.New(p.Currency, usage.InputTokens*p.InputMicrosPerToken+usage.OutputTokens*p.OutputMicrosPerToken)
}

// QuoteActual quotes provider-reported usage.
func (p TokenPrice) QuoteActual(usage tokenusage.Actual) money.Amount {
	return money.New(p.Currency, usage.InputTokens*p.InputMicrosPerToken+usage.OutputTokens*p.OutputMicrosPerToken)
}

// PriceBook returns a component price book compatible with legacy token fields.
func (p TokenPrice) PriceBook(category Category) PriceBook {
	if !ValidCategory(normalizeCategoryOrDefault(category)) {
		category = CategoryChat
	}
	components, _ := NormalizeComponents(category, nil, p)
	return PriceBook{Category: normalizeCategoryOrDefault(category), Currency: normalizeCurrency(p.Currency), Components: components}
}

// QuoteEstimate quotes a pre-provider usage estimate with component pricing.
func (b PriceBook) QuoteEstimate(usage tokenusage.Estimate) money.Amount {
	return b.QuoteMetered(MeteredFromEstimate(usage))
}

// QuoteActual quotes provider-reported usage with component pricing.
func (b PriceBook) QuoteActual(usage tokenusage.Actual) money.Amount {
	return b.QuoteMetered(MeteredFromActual(usage))
}

// QuoteMetered quotes a complete usage vector with component pricing.
func (b PriceBook) QuoteMetered(usage MeteredUsage) money.Amount {
	var micros int64
	for _, component := range b.Components {
		micros += usage.quantity(component.Unit) * component.MicrosPerUnit
	}
	return money.New(normalizeCurrency(b.Currency), micros)
}

// MeteredFromEstimate converts admission token estimates into billable units.
func MeteredFromEstimate(usage tokenusage.Estimate) MeteredUsage {
	return MeteredUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Requests:     1,
		Tasks:        1,
	}
}

// MeteredFromActual converts provider token usage into billable units.
func MeteredFromActual(usage tokenusage.Actual) MeteredUsage {
	return MeteredUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Requests:     1,
		Tasks:        1,
	}
}

// InferCategory returns a category using explicit value first, then model hints.
func InferCategory(value string, hints ...string) (Category, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "" {
		category := Category(value)
		if !ValidCategory(category) {
			return "", fmt.Errorf("pricing category %q is not supported", value)
		}
		return category, nil
	}

	joined := strings.ToLower(strings.Join(hints, " "))
	switch {
	case strings.Contains(joined, "realtime"):
		return CategoryRealtimeReserved, nil
	case strings.Contains(joined, "embedding"):
		return CategoryEmbedding, nil
	case strings.Contains(joined, "rerank"):
		return CategoryRerank, nil
	case strings.Contains(joined, "moderation"):
		return CategoryModeration, nil
	case strings.Contains(joined, "transcription") || strings.Contains(joined, "transcribe"):
		return CategoryAudioTranscription, nil
	case strings.Contains(joined, "speech") || strings.Contains(joined, "tts"):
		return CategoryAudioSpeech, nil
	case strings.Contains(joined, "music"):
		return CategoryMusic, nil
	case strings.Contains(joined, "video"):
		return CategoryVideo, nil
	case strings.Contains(joined, "image"):
		return CategoryImage, nil
	default:
		return CategoryChat, nil
	}
}

// ValidCategory reports whether category has a built-in pricing template.
func ValidCategory(category Category) bool {
	_, ok := templates()[normalizeCategoryOrDefault(category)]
	return ok
}

// DefaultTemplate returns the allowed component units for a category.
func DefaultTemplate(category Category) Template {
	category = normalizeCategoryOrDefault(category)
	return Template{Category: category, Units: append([]Unit(nil), templates()[category]...)}
}

// NormalizePriceBook validates and canonicalizes a component price book.
func NormalizePriceBook(book PriceBook, legacy TokenPrice) (PriceBook, error) {
	category := normalizeCategoryOrDefault(book.Category)
	if !ValidCategory(category) {
		return PriceBook{}, fmt.Errorf("pricing category %q is not supported", book.Category)
	}
	components, err := NormalizeComponents(category, book.Components, legacy)
	if err != nil {
		return PriceBook{}, err
	}
	return PriceBook{
		Category:   category,
		Currency:   normalizeCurrency(firstNonEmpty(book.Currency, legacy.Currency)),
		Components: components,
	}, nil
}

// NormalizeComponents validates components and expands legacy token prices when needed.
func NormalizeComponents(category Category, components []Component, legacy TokenPrice) ([]Component, error) {
	category = normalizeCategoryOrDefault(category)
	if !ValidCategory(category) {
		return nil, fmt.Errorf("pricing category %q is not supported", category)
	}
	if len(components) == 0 {
		components = legacyComponents(legacy)
	}

	allowed := allowedUnits(category)
	seen := make(map[Unit]struct{}, len(components))
	out := make([]Component, 0, len(components))
	for _, component := range components {
		unit := Unit(strings.ToLower(strings.TrimSpace(string(component.Unit))))
		if unit == "" {
			return nil, fmt.Errorf("pricing component unit is required")
		}
		if _, ok := allowed[unit]; !ok {
			return nil, fmt.Errorf("pricing unit %q is not allowed for category %q", unit, category)
		}
		if _, ok := seen[unit]; ok {
			return nil, fmt.Errorf("pricing unit %q is duplicated", unit)
		}
		if component.MicrosPerUnit < 0 {
			return nil, fmt.Errorf("pricing component %q must be non-negative", unit)
		}
		seen[unit] = struct{}{}
		out = append(out, Component{Unit: unit, MicrosPerUnit: component.MicrosPerUnit})
	}
	return out, nil
}

// LegacyTokenPrice extracts legacy token fields from normalized components.
func LegacyTokenPrice(currency string, components []Component) TokenPrice {
	price := TokenPrice{Currency: normalizeCurrency(currency)}
	for _, component := range components {
		switch component.Unit {
		case UnitInputToken:
			price.InputMicrosPerToken = component.MicrosPerUnit
		case UnitOutputToken:
			price.OutputMicrosPerToken = component.MicrosPerUnit
		}
	}
	return price
}

func (u MeteredUsage) quantity(unit Unit) int64 {
	switch unit {
	case UnitInputToken:
		return u.InputTokens
	case UnitOutputToken:
		return u.OutputTokens
	case UnitCacheReadToken:
		return u.CacheReadTokens
	case UnitCacheWriteToken:
		return u.CacheWriteTokens
	case UnitReasoningToken:
		return u.ReasoningTokens
	case UnitAudioInputToken:
		return u.AudioInputTokens
	case UnitAudioOutputToken:
		return u.AudioOutputTokens
	case UnitImageInputToken:
		return u.ImageInputTokens
	case UnitImageOutputToken:
		return u.ImageOutputTokens
	case UnitVideoInputToken:
		return u.VideoInputTokens
	case UnitVideoOutputToken:
		return u.VideoOutputTokens
	case UnitRequest:
		return u.Requests
	case UnitImage:
		return u.Images
	case UnitAudioSecond:
		return u.AudioSeconds
	case UnitVideoSecond:
		return u.VideoSeconds
	case UnitTask:
		return u.Tasks
	default:
		return 0
	}
}

func legacyComponents(price TokenPrice) []Component {
	var components []Component
	if price.InputMicrosPerToken != 0 {
		components = append(components, Component{Unit: UnitInputToken, MicrosPerUnit: price.InputMicrosPerToken})
	}
	if price.OutputMicrosPerToken != 0 {
		components = append(components, Component{Unit: UnitOutputToken, MicrosPerUnit: price.OutputMicrosPerToken})
	}
	return components
}

func allowedUnits(category Category) map[Unit]struct{} {
	out := make(map[Unit]struct{})
	for _, unit := range templates()[category] {
		out[unit] = struct{}{}
	}
	return out
}

func templates() map[Category][]Unit {
	tokenUnits := []Unit{UnitInputToken, UnitOutputToken, UnitCacheReadToken, UnitCacheWriteToken, UnitReasoningToken, UnitRequest}
	return map[Category][]Unit{
		CategoryChat:               tokenUnits,
		CategoryEmbedding:          {UnitInputToken, UnitRequest},
		CategoryRerank:             {UnitInputToken, UnitRequest},
		CategoryImage:              {UnitInputToken, UnitOutputToken, UnitImageInputToken, UnitImageOutputToken, UnitImage, UnitRequest, UnitTask},
		CategoryVideo:              {UnitInputToken, UnitOutputToken, UnitVideoInputToken, UnitVideoOutputToken, UnitVideoSecond, UnitRequest, UnitTask},
		CategoryAudioSpeech:        {UnitInputToken, UnitOutputToken, UnitAudioOutputToken, UnitAudioSecond, UnitRequest, UnitTask},
		CategoryAudioTranscription: {UnitInputToken, UnitAudioInputToken, UnitAudioSecond, UnitRequest, UnitTask},
		CategoryMusic:              {UnitInputToken, UnitOutputToken, UnitAudioSecond, UnitRequest, UnitTask},
		CategoryModeration:         {UnitInputToken, UnitRequest},
		CategoryRealtimeReserved:   {UnitInputToken, UnitOutputToken, UnitCacheReadToken, UnitCacheWriteToken, UnitReasoningToken, UnitAudioInputToken, UnitAudioOutputToken, UnitRequest},
	}
}

func normalizeCategoryOrDefault(category Category) Category {
	category = Category(strings.ToLower(strings.TrimSpace(string(category))))
	if category == "" {
		return CategoryChat
	}
	return category
}

func normalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
