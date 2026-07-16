package file

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gen2brain/go-fitz"
)

// errNoText is returned when a PDF contains no extractable text (image-only).
var errNoText = errors.New("pdf contains no extractable text")

// extractPDFText extracts plain text from all pages of a PDF file using MuPDF.
// Returns errNoText if the PDF has no text on any page (e.g. scanned image-only PDF).
// Returns an error if the file cannot be opened (corrupted, password-protected, etc.).
func extractPDFText(path string) (string, error) {
	doc, err := fitz.New(path)
	if err != nil {
		if errors.Is(err, fitz.ErrNeedsPassword) {
			return "", fmt.Errorf("pdf is password-protected: %w", err)
		}
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer doc.Close()

	var sb strings.Builder
	for n := 0; n < doc.NumPage(); n++ {
		text, err := doc.Text(n)
		if err != nil {
			// Skip unreadable pages rather than aborting the whole document.
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(text)
	}

	if sb.Len() == 0 {
		return "", errNoText
	}
	return sb.String(), nil
}
