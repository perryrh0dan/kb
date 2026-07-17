CREATE TABLE IF NOT EXISTS documents (
    id           TEXT PRIMARY KEY,
    title        TEXT NOT NULL,
    source_type  TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    metadata     TEXT NOT NULL DEFAULT '{}',
    ingested_at  DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
    id          TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    embedding   F32_BLOB(3072)
);

CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON chunks(document_id);

CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vectors USING vec0(
    chunk_id TEXT PRIMARY KEY,
    source_type TEXT PARTITION KEY,
    embedding FLOAT[3072] distance_metric=cosine
);
