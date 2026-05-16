package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	memgraph "github.com/camggould/memgraph"
	"github.com/camggould/memgraph/store/sqlite"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestServer(t *testing.T) (*sdkmcp.ClientSession, memgraph.GraphID, func()) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	g, err := store.CreateGraph(context.Background(), memgraph.GraphInput{Name: "test"})
	if err != nil {
		_ = store.Close()
		t.Fatalf("create graph: %v", err)
	}

	srv := New(store).build()
	clientT, serverT := sdkmcp.NewInMemoryTransports()

	ctx := context.Background()
	srvSession, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		_ = store.Close()
		t.Fatalf("server connect: %v", err)
	}

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	clientSess, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		_ = srvSession.Close()
		_ = store.Close()
		t.Fatalf("client connect: %v", err)
	}

	cleanup := func() {
		_ = clientSess.Close()
		_ = srvSession.Close()
		_ = store.Close()
	}
	t.Cleanup(cleanup)
	return clientSess, g.ID, cleanup
}

// callTool invokes a tool by name and JSON-unmarshals its structured
// content into out. Returns the raw result for callers that need to inspect
// IsError or the textual content.
func callTool(t *testing.T, sess *sdkmcp.ClientSession, name string, args any, out any) *sdkmcp.CallToolResult {
	t.Helper()
	res, err := sess.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		var msg strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				msg.WriteString(tc.Text)
			}
		}
		t.Fatalf("%s returned IsError: %s", name, msg.String())
	}
	if out != nil {
		b, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatalf("marshal %s structured: %v", name, err)
		}
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("unmarshal %s: %v (raw=%s)", name, err, string(b))
		}
	}
	return res
}

func TestMCP_BuildAndRenderDocument(t *testing.T) {
	sess, graphID, _ := newTestServer(t)

	// Create a document.
	var doc documentOut
	callTool(t, sess, "docs_create_document", map[string]any{
		"graph_id": string(graphID),
		"title":    "Demo",
		"author":   "tester",
	}, &doc)
	if doc.LineageID == "" {
		t.Fatalf("empty document lineage_id")
	}

	// Append a heading and a paragraph.
	var heading nodeOut
	callTool(t, sess, "docs_append_heading", map[string]any{
		"document_id": doc.LineageID,
		"level":       1,
		"text":        "Section",
	}, &heading)

	callTool(t, sess, "docs_append_paragraph", map[string]any{
		"document_id": doc.LineageID,
		"parent_id":   heading.LineageID,
		"text":        "Content under section.",
	}, nil)

	// Render and inspect.
	var rendered renderOut
	callTool(t, sess, "docs_render_markdown", map[string]any{
		"document_id": doc.LineageID,
	}, &rendered)

	if !strings.Contains(rendered.Markdown, "# Section") {
		t.Errorf("render missing heading:\n%s", rendered.Markdown)
	}
	if !strings.Contains(rendered.Markdown, "Content under section.") {
		t.Errorf("render missing paragraph:\n%s", rendered.Markdown)
	}
}

func TestMCP_MediaAndList(t *testing.T) {
	sess, graphID, _ := newTestServer(t)

	var doc documentOut
	callTool(t, sess, "docs_create_document", map[string]any{
		"graph_id": string(graphID),
		"title":    "Media + List",
	}, &doc)

	callTool(t, sess, "docs_append_media", map[string]any{
		"document_id": doc.LineageID,
		"media_kind":  "image",
		"src":         "https://example.com/img.png",
		"alt":         "an example",
	}, nil)

	callTool(t, sess, "docs_append_list", map[string]any{
		"document_id": doc.LineageID,
		"ordered":     true,
		"items":       []string{"one", "two", "three"},
	}, nil)

	var rendered renderOut
	callTool(t, sess, "docs_render_markdown", map[string]any{
		"document_id": doc.LineageID,
	}, &rendered)

	for _, want := range []string{
		"![an example](https://example.com/img.png)",
		"1. one",
		"2. two",
		"3. three",
	} {
		if !strings.Contains(rendered.Markdown, want) {
			t.Errorf("missing %q in:\n%s", want, rendered.Markdown)
		}
	}
}

func TestMCP_ListDocumentsAndOutline(t *testing.T) {
	sess, graphID, _ := newTestServer(t)

	var doc documentOut
	callTool(t, sess, "docs_create_document", map[string]any{
		"graph_id": string(graphID),
		"title":    "Outline Test",
	}, &doc)
	var heading nodeOut
	callTool(t, sess, "docs_append_heading", map[string]any{
		"document_id": doc.LineageID,
		"level":       2,
		"text":        "Subsection",
	}, &heading)

	var docs listDocumentsOut
	callTool(t, sess, "docs_list_documents", map[string]any{
		"graph_id": string(graphID),
	}, &docs)
	if len(docs.Documents) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs.Documents))
	}

	// Outline is wrapped in a top-level "tree" raw JSON to dodge a schema
	// cycle on the recursive Outline type. Unmarshal it ourselves.
	var wrapper struct {
		Tree json.RawMessage `json:"tree"`
	}
	callTool(t, sess, "docs_get_outline", map[string]any{
		"document_id": doc.LineageID,
	}, &wrapper)
	var outline map[string]any
	if err := json.Unmarshal(wrapper.Tree, &outline); err != nil {
		t.Fatalf("unmarshal outline tree: %v", err)
	}
	root, _ := outline["node"].(map[string]any)
	if root["kind"] != "doc.document" {
		t.Errorf("outline root kind = %v, want doc.document", root["kind"])
	}
	children, _ := outline["children"].([]any)
	if len(children) != 1 {
		t.Fatalf("expected 1 child in outline, got %d", len(children))
	}
	first := children[0].(map[string]any)
	firstNode := first["node"].(map[string]any)
	if firstNode["kind"] != "doc.heading" {
		t.Errorf("first child kind = %v, want doc.heading", firstNode["kind"])
	}
	if int(firstNode["level"].(float64)) != 2 {
		t.Errorf("heading level = %v, want 2", firstNode["level"])
	}
}
