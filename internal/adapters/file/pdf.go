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
// If opts.Vision is non-nil and VisionConfig.Enabled is true, it also extracts
// visual elements per page (embedded images + SVG vector graphics) and appends
// a GPT-4o Vision description to each page's text.
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

		// --- Visual extraction + Vision ---
		if visionEnabled {
			var pageImages [][]byte

			// 1. Raster images from HTML
			html, err := doc.HTML(n, false)
			if err != nil {
				log.Warn("failed to get HTML from pdf page", "path", path, "page", n, "error", err)
			} else {
				raster := extractRasterImages(html)
				pageImages = append(pageImages, raster...)
				log.Debug("extracted raster images from pdf page", "path", path, "page", n, "count", len(raster))
			}

			// 2. SVG vector graphics
			svg, err := doc.SVG(n)
			if err != nil {
				log.Warn("failed to get SVG from pdf page", "path", path, "page", n, "error", err)
			} else if hasMeaningfulPaths(svg) {
				pngBytes, err := renderSVG(svg, opts.Vision.Config.DPI)
				if err != nil {
					log.Warn("failed to render SVG via rsvg-convert", "path", path, "page", n, "error", err)
				} else if pngBytes != nil {
					pageImages = append(pageImages, pngBytes)
					log.Debug("rendered SVG vector graphics from pdf page", "path", path, "page", n)
				} else {
					log.Debug("rsvg-convert not available, skipping SVG for page", "path", path, "page", n)
				}
			}

			// 3. Vision API call
			if len(pageImages) > 0 {
				summary, err := describeVisuals(ctx, opts.Vision.Client, opts.Vision.Config.Model, pageImages)
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
