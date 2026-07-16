package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	oai "github.com/sashabaranov/go-openai"
)

const maxImagesPerPage = 10

const visionPrompt = `Describe ONLY the visual elements on this image: diagrams, charts, graphs, illustrations, tables with visual structure, and embedded photographs. Do NOT transcribe or repeat any text that appears — the text content is already captured separately. Focus exclusively on what the visual communicates that plain text cannot: spatial relationships, data trends, color coding, layout structure, and visual patterns.`

// imgTagRe matches <img src="data:image/TYPE;base64,DATA"> tags in HTML.
var imgTagRe = regexp.MustCompile(`<img\s[^>]*src="data:image/([a-zA-Z+]+);base64,([^"]+)"`)

// pathTagRe matches <path ...> elements in SVG (both self-closing and non-self-closing).
var pathTagRe = regexp.MustCompile(`<path\b[^>]*/?>`) 

// fontIDRe matches path ids that are font glyphs (id="font_...").
var fontIDRe = regexp.MustCompile(`\bid="font_`)

var (
	rsvgOnce      sync.Once
	rsvgAvailFlag bool
)

// rsvgAvailable returns true if rsvg-convert is found in PATH. Result is cached.
func rsvgAvailable() bool {
	rsvgOnce.Do(func() {
		_, err := exec.LookPath("rsvg-convert")
		rsvgAvailFlag = err == nil
	})
	return rsvgAvailFlag
}

// extractRasterImages parses HTML output from go-fitz and returns decoded image bytes
// for every <img src="data:image/TYPE;base64,..."> tag found.
func extractRasterImages(html string) [][]byte {
	matches := imgTagRe.FindAllStringSubmatch(html, -1)
	var images [][]byte
	for _, m := range matches {
		// m[2] is the base64 payload; try multiple encodings for robustness
		data := m[2]
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			// Some base64 in HTML may use URL-safe encoding
			decoded, err = base64.URLEncoding.DecodeString(data)
		}
		if err != nil {
			// Try raw (no padding) variants — some encoders omit or misalign padding
			stripped := strings.TrimRight(data, "=")
			decoded, err = base64.RawStdEncoding.DecodeString(stripped)
			if err != nil {
				decoded, err = base64.RawURLEncoding.DecodeString(stripped)
			}
		}
		if err != nil {
			slog.Default().Warn("failed to decode base64 image from PDF HTML", "error", err)
			continue
		}
		images = append(images, decoded)
	}
	return images
}

// hasMeaningfulPaths returns true if the SVG contains <path> elements
// that are NOT font glyphs (i.e. id does not start with "font_").
// This distinguishes real vector graphics from MuPDF's text-glyph paths.
func hasMeaningfulPaths(svg string) bool {
	matches := pathTagRe.FindAllString(svg, -1)
	for _, m := range matches {
		if !fontIDRe.MatchString(m) {
			return true
		}
	}
	return false
}

// renderSVGWithBinary renders an SVG string to PNG bytes using the given binary path.
// Returns (nil, nil) if the binary does not exist or is not executable.
// Returns (nil, err) only on unexpected execution errors.
func renderSVGWithBinary(binary, svg string, dpi float64) ([]byte, error) {
	_, err := exec.LookPath(binary)
	if err != nil {
		return nil, nil // binary not available — skip silently
	}
	cmd := exec.Command(binary,
		"--format", "png",
		"--dpi-x", fmt.Sprintf("%.0f", dpi),
		"--dpi-y", fmt.Sprintf("%.0f", dpi),
		"-",
	)
	cmd.Stdin = strings.NewReader(svg)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rsvg-convert: %w (stderr: %s)", err, stderr.String())
	}
	return out.Bytes(), nil
}

// renderSVG renders an SVG string to PNG using rsvg-convert.
// Returns (nil, nil) if rsvg-convert is not available.
func renderSVG(svg string, dpi float64) ([]byte, error) {
	if !rsvgAvailable() {
		return nil, nil
	}
	return renderSVGWithBinary("rsvg-convert", svg, dpi)
}

// describeVisuals sends up to maxImagesPerPage PNG/JPEG images to GPT-4o Vision
// and returns a plain-text description of the visual content.
// Images must be raw PNG or JPEG bytes.
func describeVisuals(ctx context.Context, client *oai.Client, model string, images [][]byte) (string, error) {
	if len(images) == 0 {
		return "", nil
	}
	// Cap at maxImagesPerPage to control cost
	if len(images) > maxImagesPerPage {
		slog.Default().Warn("truncating images sent to vision model", "total", len(images), "limit", maxImagesPerPage)
		images = images[:maxImagesPerPage]
	}

	// Build multi-part message: one image_url part per image
	parts := []oai.ChatMessagePart{
		{
			Type: oai.ChatMessagePartTypeText,
			Text: visionPrompt,
		},
	}
	for _, imgBytes := range images {
		// Detect MIME type from magic bytes
		mime := "image/png"
		if len(imgBytes) >= 2 && imgBytes[0] == 0xFF && imgBytes[1] == 0xD8 {
			mime = "image/jpeg"
		}
		encoded := base64.StdEncoding.EncodeToString(imgBytes)
		dataURL := fmt.Sprintf("data:%s;base64,%s", mime, encoded)
		parts = append(parts, oai.ChatMessagePart{
			Type: oai.ChatMessagePartTypeImageURL,
			ImageURL: &oai.ChatMessageImageURL{
				URL:    dataURL,
				Detail: oai.ImageURLDetailHigh,
			},
		})
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
