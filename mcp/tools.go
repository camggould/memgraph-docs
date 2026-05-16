package mcp

import (
	"context"
	"time"

	memgraph "github.com/camggould/memgraph"
	docs "github.com/camggould/memgraph-docs"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- DTOs ---

type documentOut struct {
	LineageID string    `json:"lineage_id"`
	GraphID   string    `json:"graph_id"`
	Title     string    `json:"title"`
	Author    string    `json:"author,omitempty"`
	Abstract  string    `json:"abstract,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func toDocumentOut(d docs.DocumentRef) documentOut {
	return documentOut{
		LineageID: string(d.LineageID),
		GraphID:   string(d.GraphID),
		Title:     d.Title,
		Author:    d.Author,
		Abstract:  d.Abstract,
		CreatedAt: d.CreatedAt,
	}
}

type nodeOut struct {
	LineageID string `json:"lineage_id"`
	GraphID   string `json:"graph_id"`
	Kind      string `json:"kind"`
	Ordinal   int    `json:"ordinal,omitempty"`
}

func toNodeOut(n docs.NodeRef) nodeOut {
	return nodeOut{
		LineageID: string(n.LineageID),
		GraphID:   string(n.GraphID),
		Kind:      n.Kind,
		Ordinal:   n.Ordinal,
	}
}

// --- Inputs ---

type createDocumentIn struct {
	GraphID   string `json:"graph_id" jsonschema:"the graph to create the document in"`
	Title     string `json:"title" jsonschema:"document title"`
	Author    string `json:"author,omitempty"`
	Abstract  string `json:"abstract,omitempty"`
	CreatedBy string `json:"created_by,omitempty"`
}

type listDocumentsIn struct {
	GraphID string `json:"graph_id"`
}

type listDocumentsOut struct {
	Documents []documentOut `json:"documents"`
}

type appendHeadingIn struct {
	DocumentID string `json:"document_id"`
	ParentID   string `json:"parent_id,omitempty"`
	Level      int    `json:"level" jsonschema:"heading level 1-6"`
	Text       string `json:"text"`
	CreatedBy  string `json:"created_by,omitempty"`
}

type appendParagraphIn struct {
	DocumentID string `json:"document_id"`
	ParentID   string `json:"parent_id,omitempty"`
	Text       string `json:"text"`
	CreatedBy  string `json:"created_by,omitempty"`
}

type appendListIn struct {
	DocumentID string   `json:"document_id"`
	ParentID   string   `json:"parent_id,omitempty"`
	Ordered    bool     `json:"ordered"`
	Items      []string `json:"items"`
	CreatedBy  string   `json:"created_by,omitempty"`
}

type appendCodeIn struct {
	DocumentID string `json:"document_id"`
	ParentID   string `json:"parent_id,omitempty"`
	Code       string `json:"code"`
	Language   string `json:"language,omitempty"`
	CreatedBy  string `json:"created_by,omitempty"`
}

type appendQuoteIn struct {
	DocumentID string `json:"document_id"`
	ParentID   string `json:"parent_id,omitempty"`
	Text       string `json:"text"`
	CreatedBy  string `json:"created_by,omitempty"`
}

type appendMediaIn struct {
	DocumentID string `json:"document_id"`
	ParentID   string `json:"parent_id,omitempty"`
	MediaKind  string `json:"media_kind,omitempty" jsonschema:"one of image, video, audio, file (default image)"`
	Src        string `json:"src"`
	Alt        string `json:"alt,omitempty"`
	Caption    string `json:"caption,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	CreatedBy  string `json:"created_by,omitempty"`
}

type appendDividerIn struct {
	DocumentID string `json:"document_id"`
	ParentID   string `json:"parent_id,omitempty"`
	CreatedBy  string `json:"created_by,omitempty"`
}

type updateTextIn struct {
	NodeID    string `json:"node_id"`
	Text      string `json:"text"`
	CreatedBy string `json:"created_by,omitempty"`
}

type moveIn struct {
	NodeID      string `json:"node_id"`
	NewParentID string `json:"new_parent_id"`
	Position    *int   `json:"position,omitempty" jsonschema:"0-indexed insertion point; omit to append"`
	CreatedBy   string `json:"created_by,omitempty"`
}

type renderIn struct {
	DocumentID string `json:"document_id"`
}

type renderOut struct {
	Markdown string `json:"markdown"`
}

type outlineIn struct {
	DocumentID string `json:"document_id"`
	MaxDepth   int    `json:"max_depth,omitempty" jsonschema:"0 = unlimited"`
}

// outlineOut wraps the outline tree as `any` so the SDK's schema generator
// doesn't recurse into the cyclic docs.Outline type. Clients still see a
// normal nested JSON object under "tree".
type outlineOut struct {
	Tree any `json:"tree"`
}

type okOut struct {
	Ok bool `json:"ok"`
}

// --- Registration ---

func (s *Server) registerTools(srv *sdkmcp.Server) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_create_document",
		Description: "Create a new document in a graph.",
	}, s.handleCreateDocument)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_list_documents",
		Description: "List all documents in a graph.",
	}, s.handleListDocuments)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_get_outline",
		Description: "Return the structural tree of a document. Set max_depth to limit nesting.",
	}, s.handleGetOutline)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_render_markdown",
		Description: "Render the document as a CommonMark string.",
	}, s.handleRenderMarkdown)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_append_heading",
		Description: "Append a heading (level 1-6) under a parent (default: document root).",
	}, s.handleAppendHeading)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_append_paragraph",
		Description: "Append a paragraph under a parent (default: document root).",
	}, s.handleAppendParagraph)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_append_list",
		Description: "Append an ordered or unordered list with the given items.",
	}, s.handleAppendList)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_append_code",
		Description: "Append a fenced code block.",
	}, s.handleAppendCode)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_append_quote",
		Description: "Append a blockquote.",
	}, s.handleAppendQuote)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_append_media",
		Description: "Append a media reference (image, video, audio, file) by URL.",
	}, s.handleAppendMedia)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_append_divider",
		Description: "Append a horizontal rule.",
	}, s.handleAppendDivider)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_update_text",
		Description: "Create a new version of an existing node with updated text.",
	}, s.handleUpdateText)

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "docs_move",
		Description: "Reparent and/or reorder a node. Position is 0-indexed; omit to append.",
	}, s.handleMove)
}

// --- Handlers ---

func (s *Server) handleCreateDocument(ctx context.Context, _ *sdkmcp.CallToolRequest, in createDocumentIn) (*sdkmcp.CallToolResult, documentOut, error) {
	d, err := s.builder.CreateDocument(ctx, docs.CreateDocumentInput{
		GraphID:   memgraph.GraphID(in.GraphID),
		Title:     in.Title,
		Author:    in.Author,
		Abstract:  in.Abstract,
		CreatedBy: in.CreatedBy,
	})
	if err != nil {
		return nil, documentOut{}, err
	}
	return nil, toDocumentOut(d), nil
}

func (s *Server) handleListDocuments(ctx context.Context, _ *sdkmcp.CallToolRequest, in listDocumentsIn) (*sdkmcp.CallToolResult, listDocumentsOut, error) {
	list, err := s.builder.ListDocuments(ctx, memgraph.GraphID(in.GraphID))
	if err != nil {
		return nil, listDocumentsOut{}, err
	}
	out := listDocumentsOut{Documents: make([]documentOut, 0, len(list))}
	for _, d := range list {
		out.Documents = append(out.Documents, toDocumentOut(d))
	}
	return nil, out, nil
}

func (s *Server) handleGetOutline(ctx context.Context, _ *sdkmcp.CallToolRequest, in outlineIn) (*sdkmcp.CallToolResult, outlineOut, error) {
	o, err := s.builder.GetOutline(ctx, memgraph.LineageID(in.DocumentID), in.MaxDepth)
	if err != nil {
		return nil, outlineOut{}, err
	}
	return nil, outlineOut{Tree: o}, nil
}

func (s *Server) handleRenderMarkdown(ctx context.Context, _ *sdkmcp.CallToolRequest, in renderIn) (*sdkmcp.CallToolResult, renderOut, error) {
	md, err := s.builder.RenderMarkdown(ctx, memgraph.LineageID(in.DocumentID))
	if err != nil {
		return nil, renderOut{}, err
	}
	return nil, renderOut{Markdown: md}, nil
}

func (s *Server) handleAppendHeading(ctx context.Context, _ *sdkmcp.CallToolRequest, in appendHeadingIn) (*sdkmcp.CallToolResult, nodeOut, error) {
	n, err := s.builder.AppendHeading(ctx, docs.AppendHeadingInput{
		DocumentID: memgraph.LineageID(in.DocumentID),
		ParentID:   memgraph.LineageID(in.ParentID),
		Level:      in.Level,
		Text:       in.Text,
		CreatedBy:  in.CreatedBy,
	})
	if err != nil {
		return nil, nodeOut{}, err
	}
	return nil, toNodeOut(n), nil
}

func (s *Server) handleAppendParagraph(ctx context.Context, _ *sdkmcp.CallToolRequest, in appendParagraphIn) (*sdkmcp.CallToolResult, nodeOut, error) {
	n, err := s.builder.AppendParagraph(ctx, docs.AppendParagraphInput{
		DocumentID: memgraph.LineageID(in.DocumentID),
		ParentID:   memgraph.LineageID(in.ParentID),
		Text:       in.Text,
		CreatedBy:  in.CreatedBy,
	})
	if err != nil {
		return nil, nodeOut{}, err
	}
	return nil, toNodeOut(n), nil
}

func (s *Server) handleAppendList(ctx context.Context, _ *sdkmcp.CallToolRequest, in appendListIn) (*sdkmcp.CallToolResult, nodeOut, error) {
	n, err := s.builder.AppendList(ctx, docs.AppendListInput{
		DocumentID: memgraph.LineageID(in.DocumentID),
		ParentID:   memgraph.LineageID(in.ParentID),
		Ordered:    in.Ordered,
		Items:      in.Items,
		CreatedBy:  in.CreatedBy,
	})
	if err != nil {
		return nil, nodeOut{}, err
	}
	return nil, toNodeOut(n), nil
}

func (s *Server) handleAppendCode(ctx context.Context, _ *sdkmcp.CallToolRequest, in appendCodeIn) (*sdkmcp.CallToolResult, nodeOut, error) {
	n, err := s.builder.AppendCode(ctx, docs.AppendCodeInput{
		DocumentID: memgraph.LineageID(in.DocumentID),
		ParentID:   memgraph.LineageID(in.ParentID),
		Code:       in.Code,
		Language:   in.Language,
		CreatedBy:  in.CreatedBy,
	})
	if err != nil {
		return nil, nodeOut{}, err
	}
	return nil, toNodeOut(n), nil
}

func (s *Server) handleAppendQuote(ctx context.Context, _ *sdkmcp.CallToolRequest, in appendQuoteIn) (*sdkmcp.CallToolResult, nodeOut, error) {
	n, err := s.builder.AppendQuote(ctx, docs.AppendQuoteInput{
		DocumentID: memgraph.LineageID(in.DocumentID),
		ParentID:   memgraph.LineageID(in.ParentID),
		Text:       in.Text,
		CreatedBy:  in.CreatedBy,
	})
	if err != nil {
		return nil, nodeOut{}, err
	}
	return nil, toNodeOut(n), nil
}

func (s *Server) handleAppendMedia(ctx context.Context, _ *sdkmcp.CallToolRequest, in appendMediaIn) (*sdkmcp.CallToolResult, nodeOut, error) {
	n, err := s.builder.AppendMedia(ctx, docs.AppendMediaInput{
		DocumentID: memgraph.LineageID(in.DocumentID),
		ParentID:   memgraph.LineageID(in.ParentID),
		MediaKind:  in.MediaKind,
		Src:        in.Src,
		Alt:        in.Alt,
		Caption:    in.Caption,
		Width:      in.Width,
		Height:     in.Height,
		CreatedBy:  in.CreatedBy,
	})
	if err != nil {
		return nil, nodeOut{}, err
	}
	return nil, toNodeOut(n), nil
}

func (s *Server) handleAppendDivider(ctx context.Context, _ *sdkmcp.CallToolRequest, in appendDividerIn) (*sdkmcp.CallToolResult, nodeOut, error) {
	n, err := s.builder.AppendDivider(ctx, docs.AppendDividerInput{
		DocumentID: memgraph.LineageID(in.DocumentID),
		ParentID:   memgraph.LineageID(in.ParentID),
		CreatedBy:  in.CreatedBy,
	})
	if err != nil {
		return nil, nodeOut{}, err
	}
	return nil, toNodeOut(n), nil
}

func (s *Server) handleUpdateText(ctx context.Context, _ *sdkmcp.CallToolRequest, in updateTextIn) (*sdkmcp.CallToolResult, nodeOut, error) {
	n, err := s.builder.UpdateText(ctx, docs.UpdateTextInput{
		NodeID:    memgraph.LineageID(in.NodeID),
		Text:      in.Text,
		CreatedBy: in.CreatedBy,
	})
	if err != nil {
		return nil, nodeOut{}, err
	}
	return nil, toNodeOut(n), nil
}

func (s *Server) handleMove(ctx context.Context, _ *sdkmcp.CallToolRequest, in moveIn) (*sdkmcp.CallToolResult, okOut, error) {
	if err := s.builder.Move(ctx, docs.MoveInput{
		NodeID:      memgraph.LineageID(in.NodeID),
		NewParentID: memgraph.LineageID(in.NewParentID),
		Position:    in.Position,
		CreatedBy:   in.CreatedBy,
	}); err != nil {
		return nil, okOut{}, err
	}
	return nil, okOut{Ok: true}, nil
}
