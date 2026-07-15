package file

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gen2brain/go-fitz"
)

// errNoContent is returned when a PDF has no text and no visual content was extracted.
var errNoContent = errors.New("pdf contains no extractable content")

// extractPDFContent extracts text from all pages of a PDF.
// If opts.Vision is non-nil and VisionConfig.Enabled is true, pages that contain
// at least one embedded image meeting the minimum size threshold are rendered as a
// full-page PNG and sent to GPT-4o Vision. The visual description is appended to
// the page text before chunking.
// Returns errNoContent if the PDF yields no content at all.
func extractPDFContent(ctx context.Context, path string, opts Options) (string, error) {
	log := slog.Default()

	doc, err := fitz.New(path)
	if err != nil {
		if errors.Is(err, fitz.ErrNeedsPassword) {
			return "", fmt.Errorf("pdf is password-protected: %w", err)
		}
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer doc.Close()

	visionEnabled := opts.Vision != nil && opts.Vision.Config.Enabled && opts.Vision.Client != nil

	var docBuilder strings.Builder

	for n := 0; n < doc.NumPage(); n++ {
		var pageBuilder strings.Builder

		// --- Text extraction ---
		text, err := doc.Text(n)
		if err != nil {
			log.Warn("failed to extract text from pdf page", "path", path, "page", n, "error", err)
		} else {
			text = strings.TrimSpace(text)
			if text != "" {
				pageBuilder.WriteString(text)
			}
		}

		// --- Vision: render page if it contains meaningful images ---
		if visionEnabled {
			svg, err := doc.SVG(n)
			if err != nil {
				log.Warn("failed to get SVG from pdf page", "path", path, "page", n, "error", err)
			} else if pageHasMeaningfulImages(svg) {
				log.Debug("page has meaningful images, rendering for vision", "path", path, "page", n)

				pngBytes, err := renderPageAsPNG(doc, n, opts.Vision.Config.DPI)
				if err != nil {
					log.Warn("failed to render pdf page as PNG", "path", path, "page", n, "error", err)
				} else {
					summary, err := describeVisuals(ctx, opts.Vision.Client, opts.Vision.Config.Model, pngBytes)
					if err != nil {
						log.Warn("vision API failed for pdf page", "path", path, "page", n, "error", err)
					} else if summary != "" {
						if pageBuilder.Len() > 0 {
							pageBuilder.WriteString("\n\n")
						}
						pageBuilder.WriteString("[Visual content: ")
						pageBuilder.WriteString(summary)
						pageBuilder.WriteString("]")
						log.Debug("appended vision summary to pdf page", "path", path, "page", n)
					}
				}
			}
		}

		// Append page content to document
		if pageBuilder.Len() > 0 {
			if docBuilder.Len() > 0 {
				docBuilder.WriteString("\n\n")
			}
			docBuilder.WriteString(pageBuilder.String())
		}
	}

	if docBuilder.Len() == 0 {
		return "", errNoContent
	}
	return docBuilder.String(), nil
}
