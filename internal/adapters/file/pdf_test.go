package file

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExtractPDFContent(t *testing.T) {
	// Vision disabled — behaves like old extractPDFText
	content, err := extractPDFContent(context.Background(), "testdata/sample.pdf", Options{})
	if err != nil {
		t.Fatalf("extractPDFContent: %v", err)
	}
	if !strings.Contains(content, "Hello from PDF") {
		t.Errorf("expected 'Hello from PDF' in content, got: %q", content)
	}
	if !strings.Contains(content, "test content for extraction") {
		t.Errorf("expected 'test content for extraction' in content, got: %q", content)
	}
}

func TestExtractPDFContentCorrupted(t *testing.T) {
	_, err := extractPDFContent(context.Background(), "testdata/corrupted.pdf", Options{})
	if err == nil {
		t.Fatal("expected error for corrupted PDF, got nil")
	}
	if errors.Is(err, errNoContent) {
		t.Errorf("expected open error, not errNoContent")
	}
}

func TestExtractPDFContentNotFound(t *testing.T) {
	_, err := extractPDFContent(context.Background(), "testdata/nonexistent.pdf", Options{})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
