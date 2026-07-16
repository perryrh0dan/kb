package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/user/kb/internal/embedder"
	"github.com/user/kb/internal/store"
)

// Server is the MCP server wrapping the knowledge base.
type Server struct {
	store    store.Store
	embedder embedder.Embedder
}

// New creates a new MCP Server.
func New(st store.Store, emb embedder.Embedder) *Server {
	return &Server{store: st, embedder: emb}
}

// Run starts the MCP server on stdio and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	log := slog.Default()
	log.Info("MCP server starting", "transport", "stdio")

	srv := mcp.NewServer(&mcp.Implementation{Name: "kb", Version: "1.0.0"}, nil)

	// Tool: search_knowledge_base
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_knowledge_base",
		Description: "Search the private knowledge base using semantic similarity.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query":     {"type": "string", "description": "Search query"},
				"limit":     {"type": "integer", "description": "Max results (default 10)", "default": 10},
				"min_score": {"type": "number", "description": "Minimum similarity score 0-1 (default 0)"},
				"source":    {"type": "string", "description": "Filter by source: file|confluence"}
			},
			"required": ["query"]
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args searchArgs) (*mcp.CallToolResult, any, error) {
		if args.Limit == 0 {
			args.Limit = 10
		}
		log.Debug("tool called: search_knowledge_base", "query", args.Query, "limit", args.Limit)
		vecs, err := s.embedder.Embed(ctx, []string{args.Query})
		if err != nil {
			log.Warn("embed failed in search_knowledge_base", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("embed error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		results, err := s.store.Search(ctx, vecs[0], args.Limit, args.MinScore, args.Source)
		if err != nil {
			log.Warn("store search failed", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		b, err := json.Marshal(results)
		if err != nil {
			log.Error("json.Marshal failed for search results", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("marshal error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		log.Debug("search_knowledge_base returned results", "count", len(results))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})

	// Tool: list_sources
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_sources",
		Description: "List all ingested sources with document and chunk counts.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		log.Debug("tool called: list_sources")
		stats, err := s.store.Stats(ctx)
		if err != nil {
			log.Warn("store stats failed", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, nil, nil
		}
		b, err := json.Marshal(stats)
		if err != nil {
			log.Error("json.Marshal failed for stats", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("marshal error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})

	// Tool: get_document
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_document",
		Description: "Retrieve the full content and metadata of a document by its ID.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"document_id": {"type": "string", "description": "Document ID (source URI)"}
			},
			"required": ["document_id"]
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args getDocArgs) (*mcp.CallToolResult, any, error) {
		log.Debug("tool called: get_document", "document_id", args.DocumentID)
		doc, err := s.store.GetDocument(ctx, args.DocumentID)
		if err != nil {
			log.Warn("store GetDocument failed", "document_id", args.DocumentID, "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, nil, nil
		}
		if doc == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "document not found"}},
				IsError: true,
			}, nil, nil
		}
		b, err := json.Marshal(doc)
		if err != nil {
			log.Error("json.Marshal failed for document", "document_id", args.DocumentID, "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("marshal error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})

	// Tool: list_documents
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_documents",
		Description: "List documents in the knowledge base. " +
			"Filter by source_type ('file' or 'confluence') and source " +
			"(folder path for files, space key for Confluence). " +
			"Omit both to list everything.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"source_type": {"type": "string", "description": "Filter by source type: file|confluence"},
				"source":      {"type": "string", "description": "Space key (e.g. PublicCloud) or folder path (e.g. /docs)"}
			}
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest, args listDocsArgs) (*mcp.CallToolResult, any, error) {
		log.Debug("tool called: list_documents", "source_type", args.SourceType, "source", args.Source)

		prefix := buildDocPrefix(args.SourceType, args.Source)

		docs, err := s.store.ListDocuments(ctx, prefix)
		if err != nil {
			log.Warn("store ListDocuments failed", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("list error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		if len(docs) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "[]"}},
			}, nil, nil
		}
		b, err := json.Marshal(docs)
		if err != nil {
			log.Error("json.Marshal failed for document list", "error", err)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("marshal error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		log.Debug("list_documents returned results", "count", len(docs))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})

	return srv.Run(ctx, &mcp.StdioTransport{})
}

type searchArgs struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit"`
	MinScore float64 `json:"min_score"`
	Source   string  `json:"source"`
}

type getDocArgs struct {
	DocumentID string `json:"document_id"`
}

type listDocsArgs struct {
	SourceType string `json:"source_type"`
	Source     string `json:"source"`
}

// buildDocPrefix converts source_type + source into a document ID prefix
// suitable for ListDocuments. Examples:
//
//	("confluence", "PublicCloud") → "confluence://PublicCloud/"
//	("file", "/root/workspace/kb/docs") → "file:///root/workspace/kb/docs/"
//	("file", "") → "file://"
//	("", "") → ""
func buildDocPrefix(sourceType, source string) string {
	switch sourceType {
	case "confluence":
		if source == "" {
			return "confluence://"
		}
		return "confluence://" + source + "/"
	case "file":
		if source == "" {
			return "file://"
		}
		// Ensure trailing slash so we don't match /docs2 when filtering /docs
		p := source
		if len(p) > 0 && p[len(p)-1] != '/' {
			p += "/"
		}
		return "file://" + p
	default:
		return ""
	}
}
