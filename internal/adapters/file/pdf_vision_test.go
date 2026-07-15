package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gen2brain/go-fitz"
	"github.com/user/kb/config"
	oai "github.com/sashabaranov/go-openai"
)

// --- pageHasMeaningfulImages ---

func TestPageHasMeaningfulImages_WithLargeImage(t *testing.T) {
	// 267x181 — both dimensions above threshold
	svg := `<svg><image id="img_1" width="267" height="181" xlink:href="data:image/png;base64,abc"/></svg>`
	if !pageHasMeaningfulImages(svg) {
		t.Error("expected true for 267x181 image, got false")
	}
}

func TestPageHasMeaningfulImages_WidthTooSmall(t *testing.T) {
	// 72x181 — width below threshold (72 < 100)
	svg := `<svg><image id="img_1" width="72" height="181" xlink:href="data:image/png;base64,abc"/></svg>`
	if pageHasMeaningfulImages(svg) {
		t.Error("expected false for 72x181 image (width below threshold), got true")
	}
}

func TestPageHasMeaningfulImages_HeightTooSmall(t *testing.T) {
	// 299x9 — height below threshold (9 < 100) — typical decorative spacer
	svg := `<svg><image id="img_1" width="299" height="9" xlink:href="data:image/png;base64,abc"/></svg>`
	if pageHasMeaningfulImages(svg) {
		t.Error("expected false for 299x9 image (height below threshold), got true")
	}
}

func TestPageHasMeaningfulImages_BothBelowThreshold(t *testing.T) {
	// 50x50 — both below threshold
	svg := `<svg><image id="img_1" width="50" height="50" xlink:href="data:image/png;base64,abc"/></svg>`
	if pageHasMeaningfulImages(svg) {
		t.Error("expected false for 50x50 image, got true")
	}
}

func TestPageHasMeaningfulImages_NoImages(t *testing.T) {
	svg := `<svg><path d="M0 0 H100 V100 Z" fill="red"/><text>hello</text></svg>`
	if pageHasMeaningfulImages(svg) {
		t.Error("expected false for SVG with no images, got true")
	}
}

func TestPageHasMeaningfulImages_MixedSizes(t *testing.T) {
	// One small image + one large image — should return true
	svg := `<svg>
		<image id="img_1" width="20" height="9" xlink:href="data:image/png;base64,abc"/>
		<image id="img_2" width="570" height="152" xlink:href="data:image/png;base64,def"/>
	</svg>`
	if !pageHasMeaningfulImages(svg) {
		t.Error("expected true when at least one image meets threshold, got false")
	}
}

func TestPageHasMeaningfulImages_ExactThreshold(t *testing.T) {
	// Exactly at threshold — should be included
	svg := `<svg><image id="img_1" width="100" height="100" xlink:href="data:image/png;base64,abc"/></svg>`
	if !pageHasMeaningfulImages(svg) {
		t.Error("expected true for image exactly at threshold (100x100), got false")
	}
}

// --- renderPageAsPNG ---

func TestRenderPageAsPNG(t *testing.T) {
	doc, err := fitz.New("testdata/sample.pdf")
	if err != nil {
		t.Fatalf("open test PDF: %v", err)
	}
	defer doc.Close()

	pngBytes, err := renderPageAsPNG(doc, 0, 72)
	if err != nil {
		t.Fatalf("renderPageAsPNG: %v", err)
	}
	if len(pngBytes) == 0 {
		t.Fatal("expected non-empty PNG bytes")
	}

	// Verify it decodes as a valid PNG with non-zero dimensions
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("result is not valid PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		t.Errorf("PNG has zero dimensions: %v", bounds)
	}
}

// --- describeVisuals ---

func TestDescribeVisuals_SendsSingleImage(t *testing.T) {
	var capturedRequest oai.ChatCompletionRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedRequest); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		resp := oai.ChatCompletionResponse{
			Choices: []oai.ChatCompletionChoice{
				{Message: oai.ChatCompletionMessage{Content: "A bar chart showing revenue by quarter."}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := oai.DefaultConfig("sk-test")
	cfg.BaseURL = srv.URL + "/v1"
	client := oai.NewClientWithConfig(cfg)

	// Create a minimal valid PNG (1x1 white pixel)
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	png.Encode(&buf, img)
	pngBytes := buf.Bytes()

	visionCfg := config.VisionConfig{
		Enabled: true,
		Model:   "gpt-4o",
		DPI:     150,
	}

	result, err := describeVisuals(context.Background(), client, visionCfg.Model, pngBytes)
	if err != nil {
		t.Fatalf("describeVisuals: %v", err)
	}
	if result != "A bar chart showing revenue by quarter." {
		t.Errorf("unexpected result: %q", result)
	}

	// Verify the request structure
	if len(capturedRequest.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(capturedRequest.Messages))
	}
	parts := capturedRequest.Messages[0].MultiContent
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (text + image), got %d", len(parts))
	}
	if parts[0].Type != oai.ChatMessagePartTypeText {
		t.Errorf("part[0] should be text, got %q", parts[0].Type)
	}
	if parts[1].Type != oai.ChatMessagePartTypeImageURL {
		t.Errorf("part[1] should be image_url, got %q", parts[1].Type)
	}
	if parts[1].ImageURL == nil {
		t.Fatal("part[1].ImageURL is nil")
	}
	expectedPrefix := "data:image/png;base64,"
	if len(parts[1].ImageURL.URL) < len(expectedPrefix) ||
		parts[1].ImageURL.URL[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("image URL should start with %q, got %q",
			expectedPrefix, parts[1].ImageURL.URL[:min(40, len(parts[1].ImageURL.URL))])
	}

	// Verify the image content round-trips correctly
	b64data := parts[1].ImageURL.URL[len(expectedPrefix):]
	decoded, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		t.Fatalf("image base64 decode: %v", err)
	}
	if !bytes.Equal(decoded, pngBytes) {
		t.Error("image bytes were not preserved correctly in Vision request")
	}
}

func TestDescribeVisuals_EmptyImage(t *testing.T) {
	result, err := describeVisuals(context.Background(), nil, "gpt-4o", nil)
	if err != nil {
		t.Fatalf("expected nil error for empty image, got: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result for nil image, got: %q", result)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
