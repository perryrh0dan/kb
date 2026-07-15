// internal/chunker/chunker_test.go
package chunker_test

import (
	"strings"
	"testing"

	"github.com/user/kb/internal/chunker"
)

func TestSplitShortText(t *testing.T) {
	c := chunker.New(512, 50)
	chunks, err := c.Split("Hello world")
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	if len(chunks) != 1 {
		t.Errorf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0] != "Hello world" {
		t.Errorf("chunk = %q, want %q", chunks[0], "Hello world")
	}
}

func TestSplitRespectsParagraphs(t *testing.T) {
	// Build text with clear paragraph breaks
	para := strings.Repeat("word ", 100) // ~100 tokens per paragraph
	text := para + "\n\n" + para + "\n\n" + para
	c := chunker.New(150, 20)
	chunks, err := c.Split(text)
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	// Should produce multiple chunks
	if len(chunks) < 2 {
		t.Errorf("got %d chunks, want >= 2", len(chunks))
	}
}

func TestOverlapIsApplied(t *testing.T) {
	// 60-word sentences separated by newlines
	sentence := strings.Repeat("word ", 60)
	text := sentence + "\n" + sentence + "\n" + sentence
	c := chunker.New(100, 30)
	chunks, err := c.Split(text)
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("need at least 2 chunks to test overlap, got %d", len(chunks))
	}
	// The end of chunk[0] should appear at the beginning of chunk[1]
	end0 := chunks[0][len(chunks[0])-20:]
	if !strings.Contains(chunks[1], end0[:10]) {
		t.Errorf("overlap not found: end of chunk[0] not in chunk[1]")
	}
}

func TestEmptyText(t *testing.T) {
	c := chunker.New(512, 50)
	chunks, err := c.Split("")
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("got %d chunks for empty text, want 0", len(chunks))
	}
}

func TestWhitespaceOnlyInput(t *testing.T) {
	c := chunker.New(512, 50)
	chunks, err := c.Split("   \n\t  ")
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	// whitespace-only should return empty
	if len(chunks) != 0 {
		t.Errorf("got %d chunks for whitespace-only text, want 0", len(chunks))
	}
}

func TestSeparatorPreservedInChunk(t *testing.T) {
	para1 := strings.Repeat("word ", 20)
	para2 := strings.Repeat("term ", 20)
	text := para1 + "\n\n" + para2
	c := chunker.New(512, 50)
	chunks, err := c.Split(text)
	if err != nil {
		t.Fatalf("Split() error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	// Content from both paragraphs should be present
	combined := strings.Join(chunks, " ")
	if !strings.Contains(combined, "word") || !strings.Contains(combined, "term") {
		t.Errorf("chunks missing expected content: %v", chunks)
	}
}

