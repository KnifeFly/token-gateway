package pricing

import (
	"testing"

	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

func TestTokenPriceQuotesMicros(t *testing.T) {
	price := TokenPrice{Currency: "USD", InputMicrosPerToken: 2, OutputMicrosPerToken: 5}
	got := price.QuoteEstimate(tokenusage.Estimate{InputTokens: 10, OutputTokens: 3})
	if got.Currency != "USD" || got.Micros != 35 {
		t.Fatalf("amount = %#v", got)
	}
}

func TestNormalizePriceBookBackfillsLegacyTokenComponents(t *testing.T) {
	book, err := NormalizePriceBook(PriceBook{Category: CategoryChat, Currency: "usd"}, TokenPrice{
		InputMicrosPerToken:  2,
		OutputMicrosPerToken: 5,
	})
	if err != nil {
		t.Fatalf("NormalizePriceBook() error = %v", err)
	}
	if book.Currency != "USD" || len(book.Components) != 2 {
		t.Fatalf("book = %#v", book)
	}

	amount := book.QuoteEstimate(tokenusage.Estimate{InputTokens: 10, OutputTokens: 3})
	if amount.Micros != 35 {
		t.Fatalf("amount = %d, want 35", amount.Micros)
	}
}

func TestNormalizePriceBookValidatesCategoryUnits(t *testing.T) {
	_, err := NormalizePriceBook(PriceBook{
		Category: CategoryEmbedding,
		Currency: "USD",
		Components: []Component{{
			Unit:          UnitVideoSecond,
			MicrosPerUnit: 10,
		}},
	}, TokenPrice{})
	if err == nil {
		t.Fatal("NormalizePriceBook() succeeded, want invalid unit error")
	}
}

func TestComponentPriceBookQuotesRequestAndTaskUnits(t *testing.T) {
	book := PriceBook{
		Category: CategoryImage,
		Currency: "USD",
		Components: []Component{
			{Unit: UnitRequest, MicrosPerUnit: 100},
			{Unit: UnitTask, MicrosPerUnit: 250},
			{Unit: UnitInputToken, MicrosPerUnit: 1},
		},
	}
	amount := book.QuoteActual(tokenusage.Actual{InputTokens: 10})
	if amount.Micros != 360 {
		t.Fatalf("amount = %d, want 360", amount.Micros)
	}
}

func TestInferCategoryUsesCapabilityAndModelHints(t *testing.T) {
	category, err := InferCategory("", "text-to-speech", "tts-1")
	if err != nil {
		t.Fatalf("InferCategory() error = %v", err)
	}
	if category != CategoryAudioSpeech {
		t.Fatalf("category = %q, want %q", category, CategoryAudioSpeech)
	}
}
