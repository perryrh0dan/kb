package chunker

import (
	"strings"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

// separators tried in order; fall back to next if chunk still too large
var separators = []string{"\n\n", "\n", ". ", " ", ""}

type Chunker struct {
	ChunkSize    int
	ChunkOverlap int
	enc          *tiktoken.Tiktoken
}

func New(chunkSize, chunkOverlap int) *Chunker {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		// fallback: approximate by words
		enc = nil
	}
	return &Chunker{ChunkSize: chunkSize, ChunkOverlap: chunkOverlap, enc: enc}
}

func (c *Chunker) tokenCount(s string) int {
	if c.enc != nil {
		return len(c.enc.Encode(s, nil, nil))
	}
	// fallback: word count as approximation
	return len(strings.Fields(s))
}

// Split splits text into overlapping chunks using recursive character splitting.
func (c *Chunker) Split(text string) ([]string, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	chunks := c.splitRecursive(text, separators)
	return c.mergeWithOverlap(chunks), nil
}

func (c *Chunker) splitRecursive(text string, seps []string) []string {
	if len(seps) == 0 || c.tokenCount(text) <= c.ChunkSize {
		return []string{text}
	}
	sep := seps[0]
	rest := seps[1:]

	var parts []string
	if sep == "" {
		// character-level split
		runes := []rune(text)
		for i := 0; i < len(runes); i += c.ChunkSize {
			end := i + c.ChunkSize
			if end > len(runes) {
				end = len(runes)
			}
			parts = append(parts, string(runes[i:end]))
		}
		return parts
	}

	segments := strings.Split(text, sep)
	var result []string
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if c.tokenCount(seg) <= c.ChunkSize {
			result = append(result, seg)
		} else {
			result = append(result, c.splitRecursive(seg, rest)...)
		}
	}
	return result
}

// mergeWithOverlap joins small segments into chunks of ChunkSize with ChunkOverlap.
func (c *Chunker) mergeWithOverlap(parts []string) []string {
	if len(parts) == 0 {
		return nil
	}
	var chunks []string
	var current []string
	currentTokens := 0

	flush := func() {
		if len(current) == 0 {
			return
		}
		chunks = append(chunks, strings.Join(current, " "))
	}

	for _, part := range parts {
		pt := c.tokenCount(part)
		if currentTokens+pt > c.ChunkSize && len(current) > 0 {
			flush()
			// keep overlap: drop parts from front until we're within overlap budget
			for len(current) > 0 && currentTokens > c.ChunkOverlap {
				removed := c.tokenCount(current[0])
				current = current[1:]
				currentTokens -= removed
			}
		}
		current = append(current, part)
		currentTokens += pt
	}
	flush()
	return chunks
}
