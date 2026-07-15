package chunker

// Chunker splits text into overlapping chunks.
type Chunker struct {
	ChunkSize    int
	ChunkOverlap int
}

// New creates a Chunker with the given token limits.
func New(chunkSize, chunkOverlap int) *Chunker {
	return &Chunker{ChunkSize: chunkSize, ChunkOverlap: chunkOverlap}
}

// Split splits text into chunks. Implemented in Task 3.
func (c *Chunker) Split(text string) ([]string, error) {
	return []string{text}, nil // placeholder
}
