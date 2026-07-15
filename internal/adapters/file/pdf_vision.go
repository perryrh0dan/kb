package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/png"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/gen2brain/go-fitz"
	oai "github.com/sashabaranov/go-openai"
)

// minImageWidth and minImageHeight define the minimum pixel dimensions an embedded
// image must have to be considered meaningful. Images smaller than both thresholds
// (e.g. logos, spacers, icons) are ignored so that pages containing only decorative
// elements are not sent to the Vision model.
const (
	minImageWidth  = 100
	minImageHeight = 100
)

const visionPrompt = `Describe the visual elements on this page: diagrams, charts, graphs, illustrations, tables, and embedded photographs or product images.

Include text that is an integral part of visual elements — such as product names, model numbers, brand names, labels on charts or diagrams, table headers, legends, and captions beneath images. These are visual identifiers that cannot be inferred from surrounding body text alone.

Do NOT transcribe running paragraph text or body copy that forms the main written content of the page — that is already captured separately.

Focus on: what products or items are shown and their names/identifiers, what data or relationships are visualised, and any labels, legends or annotations that give meaning to the visuals.`

// svgImageRe matches <image width="W" height="H" ...> elements in MuPDF SVG output.
// MuPDF always emits width before height in its <image> tags.
var svgImageRe = regexp.MustCompile(`<image\b[^>]*width="(\d+)"[^>]*height="(\d+)"`)

// pageHasMeaningfulImages reports whether the SVG output for a page contains at least
// one embedded image whose pixel dimensions meet the minimum thresholds.
// This is used to decide whether to render the whole page and send it to Vision.
func pageHasMeaningfulImages(svg string) bool {
	for _, m := range svgImageRe.FindAllStringSubmatch(svg, -1) {
		w, _ := strconv.Atoi(m[1])
		h, _ := strconv.Atoi(m[2])
		if w >= minImageWidth && h >= minImageHeight {
			return true
		}
	}
	return false
}

// renderPageAsPNG renders a single PDF page to a PNG-encoded byte slice using MuPDF.
// dpi controls the output resolution (150 is a good balance for Vision models).
func renderPageAsPNG(doc *fitz.Document, pageNum int, dpi float64) ([]byte, error) {
	img, err := doc.ImageDPI(pageNum, dpi)
	if err != nil {
		return nil, fmt.Errorf("render page %d: %w", pageNum, err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode page %d as PNG: %w", pageNum, err)
	}
	return buf.Bytes(), nil
}

// describeVisuals sends a single rendered page image to the Vision model and returns
// a plain-text description of the visual content on that page.
func describeVisuals(ctx context.Context, client *oai.Client, model string, pageImage []byte) (string, error) {
	if len(pageImage) == 0 {
		return "", nil
	}

	encoded := base64.StdEncoding.EncodeToString(pageImage)
	dataURL := fmt.Sprintf("data:image/png;base64,%s", encoded)

	parts := []oai.ChatMessagePart{
		{
			Type: oai.ChatMessagePartTypeText,
			Text: visionPrompt,
		},
		{
			Type: oai.ChatMessagePartTypeImageURL,
			ImageURL: &oai.ChatMessageImageURL{
				URL:    dataURL,
				Detail: oai.ImageURLDetailHigh,
			},
		},
	}

	resp, err := client.CreateChatCompletion(ctx, oai.ChatCompletionRequest{
		Model: model,
		Messages: []oai.ChatCompletionMessage{
			{
				Role:         oai.ChatMessageRoleUser,
				MultiContent: parts,
			},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("vision API call: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("vision API returned no choices")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// Ensure slog is used (imported for Warn calls in pdf.go that share this package).
var _ = slog.Default
