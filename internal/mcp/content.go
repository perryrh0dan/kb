package mcp

import (
	"sort"
	"strings"

	"github.com/user/kb/internal/store"
)

// reconstructContent joins chunk content while removing the token overlap
// introduced by the chunker between adjacent chunks.
func reconstructContent(chunks []store.Chunk) string {
	if len(chunks) == 0 {
		return ""
	}

	ordered := append([]store.Chunk(nil), chunks...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].ChunkIndex < ordered[j].ChunkIndex
	})

	result := strings.Fields(ordered[0].Content)
	for _, chunk := range ordered[1:] {
		words := strings.Fields(chunk.Content)
		overlap := longestOverlap(result, words)
		result = append(result, words[overlap:]...)
	}
	return strings.Join(result, " ")
}

func longestOverlap(previous, current []string) int {
	max := len(previous)
	if len(current) < max {
		max = len(current)
	}
	for size := max; size > 0; size-- {
		start := len(previous) - size
		matches := true
		for i := 0; i < size; i++ {
			if previous[start+i] != current[i] {
				matches = false
				break
			}
		}
		if matches {
			return size
		}
	}
	return 0
}
