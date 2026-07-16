package file

import (
	"errors"
	"strings"
	"testing"
)

func TestExtractPDFText(t *testing.T) {
	text, err := extractPDFText("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("extractPDFText: %v", err)
	}
	if !strings.Contains(text, "Hello from PDF") {
		t.Errorf("expected 'Hello from PDF' in extracted text, got: %q", text)
	}
	if !strings.Contains(text, "test content for extraction") {
		t.Errorf("expected 'test content for extraction' in extracted text, got: %q", text)
	}
}

func TestExtractPDFTextCorrupted(t *testing.T) {
	_, err := extractPDFText("testdata/corrupted.pdf")
	if err == nil {
		t.Fatal("expected error for corrupted PDF, got nil")
	}
	// Should not be errNoText — it should be an open/parse error
	if errors.Is(err, errNoText) {
		t.Errorf("expected open error, not errNoText")
	}
}

func TestExtractPDFTextNotFound(t *testing.T) {
	_, err := extractPDFText("testdata/nonexistent.pdf")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
