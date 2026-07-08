package postgres

import (
	"testing"

	"github.com/shennawardana23/agentic-desk/internal/secondbrain"
)

func TestValidateSearch_ClampsKToMax(t *testing.T) {
	embedding := make([]float32, secondbrain.EmbeddingDim)

	k, err := validateSearch(embedding, maxSearchK+1000)
	if err != nil {
		t.Fatalf("validateSearch: %v", err)
	}
	if k != maxSearchK {
		t.Fatalf("expected k clamped to %d, got %d", maxSearchK, k)
	}
}

func TestValidateSearch_PassesThroughValidK(t *testing.T) {
	embedding := make([]float32, secondbrain.EmbeddingDim)

	k, err := validateSearch(embedding, 5)
	if err != nil {
		t.Fatalf("validateSearch: %v", err)
	}
	if k != 5 {
		t.Fatalf("expected k=5 unchanged, got %d", k)
	}
}

func TestValidateSearch_RejectsNonPositiveK(t *testing.T) {
	embedding := make([]float32, secondbrain.EmbeddingDim)

	for _, k := range []int{0, -1} {
		if _, err := validateSearch(embedding, k); err == nil {
			t.Fatalf("expected error for k=%d, got nil", k)
		}
	}
}

func TestValidateSearch_RejectsWrongDimension(t *testing.T) {
	if _, err := validateSearch(make([]float32, secondbrain.EmbeddingDim-1), 5); err == nil {
		t.Fatal("expected error for wrong embedding dimension, got nil")
	}
}
