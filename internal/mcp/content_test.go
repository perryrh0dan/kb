package mcp

import (
	"testing"

	"github.com/user/kb/internal/store"
)

func TestReconstructContentRemovesChunkOverlap(t *testing.T) {
	chunks := []store.Chunk{
		{ChunkIndex: 0, Content: "alpha beta gamma"},
		{ChunkIndex: 1, Content: "beta gamma delta"},
		{ChunkIndex: 2, Content: "delta epsilon"},
	}

	got := reconstructContent(chunks)
	want := "alpha beta gamma delta epsilon"
	if got != want {
		t.Fatalf("reconstructContent() = %q, want %q", got, want)
	}
}

func TestReconstructContentKeepsNonOverlappingChunks(t *testing.T) {
	chunks := []store.Chunk{
		{ChunkIndex: 0, Content: "first"},
		{ChunkIndex: 1, Content: "second"},
	}

	got := reconstructContent(chunks)
	want := "first second"
	if got != want {
		t.Fatalf("reconstructContent() = %q, want %q", got, want)
	}
}
