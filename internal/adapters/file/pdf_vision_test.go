package file

import (
	"testing"
)

func TestExtractRasterImages_JPEG(t *testing.T) {
	// Minimal valid base64-encoded 1x1 white JPEG
	jpeg1x1 := "/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAFgABAQEAAAAAAAAAAAAAAAAABgUEA/8QAIRAAAQMEAgMAAAAAAAAAAAAAAQIDBAAFERIhMUH/xAAUAQEAAAAAAAAAAAAAAAAAAAAA/8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAwDAQACEQMRAD8Amw2fa5VyuzMaJFU6+6cJSgZJ/KUpSlKUpSlKUpT/2Q=="
	html := `<div><img style="position:absolute" src="data:image/jpeg;base64,` + jpeg1x1 + `"><p>some text</p></div>`

	images := extractRasterImages(html)
	if len(images) != 1 {
		t.Fatalf("got %d images, want 1", len(images))
	}
	if len(images[0]) == 0 {
		t.Error("decoded image bytes should not be empty")
	}
}

func TestExtractRasterImages_None(t *testing.T) {
	html := `<div><p>just text, no images</p></div>`
	images := extractRasterImages(html)
	if len(images) != 0 {
		t.Errorf("got %d images, want 0", len(images))
	}
}

func TestExtractRasterImages_MultipleFormats(t *testing.T) {
	html := `<div>
		<img src="data:image/jpeg;base64,/9j/abc123">
		<img src="data:image/png;base64,iVBORw0KGgo=">
	</div>`
	images := extractRasterImages(html)
	if len(images) != 2 {
		t.Errorf("got %d images, want 2", len(images))
	}
}

func TestHasMeaningfulPaths_FontOnly(t *testing.T) {
	// SVG with only font glyph paths (id starts with "font_")
	svg := `<svg><defs><path id="font_1_23" d="M 0 0 L 10 10"/></defs><use xlink:href="#font_1_23"/></svg>`
	if hasMeaningfulPaths(svg) {
		t.Error("SVG with only font glyphs should return false")
	}
}

func TestHasMeaningfulPaths_WithShapes(t *testing.T) {
	// SVG with a real shape path (no font_ id)
	svg := `<svg><path d="M 0 0 H 100 V 100 Z" fill="#ff0000"/></svg>`
	if !hasMeaningfulPaths(svg) {
		t.Error("SVG with shape paths should return true")
	}
}

func TestHasMeaningfulPaths_Empty(t *testing.T) {
	svg := `<svg><text>hello</text></svg>`
	if hasMeaningfulPaths(svg) {
		t.Error("SVG with no paths should return false")
	}
}

func TestRenderSVG_NoRsvg(t *testing.T) {
	// Force rsvg-convert unavailable by calling renderSVGWithBinary with a bad path
	svg := `<svg width="10" height="10"><rect width="10" height="10" fill="red"/></svg>`
	bytes, err := renderSVGWithBinary("nonexistent-rsvg-convert-binary", svg, 96)
	if err != nil {
		t.Fatalf("expected nil error when binary missing, got: %v", err)
	}
	if bytes != nil {
		t.Error("expected nil bytes when binary missing")
	}
}

