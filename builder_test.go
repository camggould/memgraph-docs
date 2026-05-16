package docs_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	memgraph "github.com/camggould/memgraph"
	"github.com/camggould/memgraph/store/sqlite"

	docs "github.com/camggould/memgraph-docs"
)

func newTestStore(t *testing.T) (memgraph.Store, memgraph.GraphID) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	g, err := store.CreateGraph(context.Background(), memgraph.GraphInput{
		Name: "test",
	})
	if err != nil {
		t.Fatalf("create graph: %v", err)
	}
	return store, g.ID
}

func TestCreateAndListDocuments(t *testing.T) {
	store, graphID := newTestStore(t)
	b := docs.NewBuilder(store)
	ctx := context.Background()

	doc, err := b.CreateDocument(ctx, docs.CreateDocumentInput{
		GraphID:  graphID,
		Title:    "Test Doc",
		Author:   "alice",
		Abstract: "a short summary",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	if doc.Title != "Test Doc" {
		t.Errorf("title = %q, want %q", doc.Title, "Test Doc")
	}

	list, err := b.ListDocuments(ctx, graphID)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d documents, want 1", len(list))
	}
	if list[0].LineageID != doc.LineageID {
		t.Errorf("lineage id mismatch: %q vs %q", list[0].LineageID, doc.LineageID)
	}
	if list[0].Author != "alice" || list[0].Abstract != "a short summary" {
		t.Errorf("author/abstract not preserved: %+v", list[0])
	}
}

func TestAppendAndRenderBasic(t *testing.T) {
	store, graphID := newTestStore(t)
	b := docs.NewBuilder(store)
	ctx := context.Background()

	doc, err := b.CreateDocument(ctx, docs.CreateDocumentInput{
		GraphID: graphID,
		Title:   "Greetings",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if _, err := b.AppendHeading(ctx, docs.AppendHeadingInput{
		DocumentID: doc.LineageID,
		Level:      1,
		Text:       "Hello",
	}); err != nil {
		t.Fatalf("append h1: %v", err)
	}
	if _, err := b.AppendParagraph(ctx, docs.AppendParagraphInput{
		DocumentID: doc.LineageID,
		Text:       "World goes here.",
	}); err != nil {
		t.Fatalf("append paragraph: %v", err)
	}

	md, err := b.RenderMarkdown(ctx, doc.LineageID)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := "# Hello\n\nWorld goes here.\n\n"
	if md != want {
		t.Errorf("render mismatch:\n--got--\n%q\n--want--\n%q", md, want)
	}
}

func TestHierarchicalSection(t *testing.T) {
	store, graphID := newTestStore(t)
	b := docs.NewBuilder(store)
	ctx := context.Background()

	doc, _ := b.CreateDocument(ctx, docs.CreateDocumentInput{GraphID: graphID, Title: "T"})

	intro, err := b.AppendHeading(ctx, docs.AppendHeadingInput{
		DocumentID: doc.LineageID,
		Level:      1,
		Text:       "Intro",
	})
	if err != nil {
		t.Fatalf("append intro: %v", err)
	}
	if _, err := b.AppendParagraph(ctx, docs.AppendParagraphInput{
		DocumentID: doc.LineageID,
		ParentID:   intro.LineageID,
		Text:       "First paragraph under intro.",
	}); err != nil {
		t.Fatalf("append nested paragraph: %v", err)
	}
	if _, err := b.AppendHeading(ctx, docs.AppendHeadingInput{
		DocumentID: doc.LineageID,
		Level:      1,
		Text:       "Body",
	}); err != nil {
		t.Fatalf("append body: %v", err)
	}

	md, _ := b.RenderMarkdown(ctx, doc.LineageID)
	wantParts := []string{
		"# Intro\n\n",
		"First paragraph under intro.\n\n",
		"# Body\n\n",
	}
	for _, p := range wantParts {
		if !strings.Contains(md, p) {
			t.Errorf("rendered markdown missing %q\ngot:\n%s", p, md)
		}
	}
	if !strings.HasPrefix(md, "# Intro") {
		t.Errorf("expected to start with H1 (skipping auto-title), got prefix: %q", md[:min(50, len(md))])
	}
}

func TestAppendList(t *testing.T) {
	store, graphID := newTestStore(t)
	b := docs.NewBuilder(store)
	ctx := context.Background()

	doc, _ := b.CreateDocument(ctx, docs.CreateDocumentInput{GraphID: graphID, Title: "Lists"})
	if _, err := b.AppendList(ctx, docs.AppendListInput{
		DocumentID: doc.LineageID,
		Ordered:    false,
		Items:      []string{"alpha", "beta", "gamma"},
	}); err != nil {
		t.Fatalf("append list: %v", err)
	}

	md, _ := b.RenderMarkdown(ctx, doc.LineageID)
	for _, want := range []string{"- alpha\n", "- beta\n", "- gamma\n"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in rendered output:\n%s", want, md)
		}
	}

	// Ordered too.
	doc2, _ := b.CreateDocument(ctx, docs.CreateDocumentInput{GraphID: graphID, Title: "Lists2"})
	if _, err := b.AppendList(ctx, docs.AppendListInput{
		DocumentID: doc2.LineageID,
		Ordered:    true,
		Items:      []string{"first", "second"},
	}); err != nil {
		t.Fatalf("append ordered list: %v", err)
	}
	md2, _ := b.RenderMarkdown(ctx, doc2.LineageID)
	if !strings.Contains(md2, "1. first") || !strings.Contains(md2, "2. second") {
		t.Errorf("ordered list missing items:\n%s", md2)
	}
}

func TestAppendMedia(t *testing.T) {
	store, graphID := newTestStore(t)
	b := docs.NewBuilder(store)
	ctx := context.Background()

	doc, _ := b.CreateDocument(ctx, docs.CreateDocumentInput{GraphID: graphID, Title: "Pic"})
	if _, err := b.AppendMedia(ctx, docs.AppendMediaInput{
		DocumentID: doc.LineageID,
		MediaKind:  docs.MediaImage,
		Src:        "https://example.com/foo.png",
		Alt:        "a foo",
		Caption:    "the foo",
	}); err != nil {
		t.Fatalf("append media: %v", err)
	}
	md, _ := b.RenderMarkdown(ctx, doc.LineageID)
	if !strings.Contains(md, "![a foo](https://example.com/foo.png)") {
		t.Errorf("missing image markup:\n%s", md)
	}
	if !strings.Contains(md, "*the foo*") {
		t.Errorf("missing caption:\n%s", md)
	}
}

func TestUpdateTextCreatesNewVersion(t *testing.T) {
	store, graphID := newTestStore(t)
	b := docs.NewBuilder(store)
	ctx := context.Background()

	doc, _ := b.CreateDocument(ctx, docs.CreateDocumentInput{GraphID: graphID, Title: "U"})
	p, err := b.AppendParagraph(ctx, docs.AppendParagraphInput{
		DocumentID: doc.LineageID,
		Text:       "original",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.UpdateText(ctx, docs.UpdateTextInput{
		NodeID: p.LineageID,
		Text:   "edited",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	md, _ := b.RenderMarkdown(ctx, doc.LineageID)
	if !strings.Contains(md, "edited") {
		t.Errorf("expected edited content, got:\n%s", md)
	}
	if strings.Contains(md, "original") {
		t.Errorf("old version should be superseded, got:\n%s", md)
	}

	// History should have both versions.
	hist, err := store.History(ctx, p.LineageID)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 {
		t.Errorf("expected 2 versions, got %d", len(hist))
	}
}

func TestMoveReordersAndReparents(t *testing.T) {
	store, graphID := newTestStore(t)
	b := docs.NewBuilder(store)
	ctx := context.Background()

	doc, _ := b.CreateDocument(ctx, docs.CreateDocumentInput{GraphID: graphID, Title: "Move"})
	a, _ := b.AppendParagraph(ctx, docs.AppendParagraphInput{DocumentID: doc.LineageID, Text: "alpha"})
	bb, _ := b.AppendParagraph(ctx, docs.AppendParagraphInput{DocumentID: doc.LineageID, Text: "beta"})
	c, _ := b.AppendParagraph(ctx, docs.AppendParagraphInput{DocumentID: doc.LineageID, Text: "gamma"})

	// Move gamma to position 0 (start). New order: gamma, alpha, beta.
	pos := 0
	if err := b.Move(ctx, docs.MoveInput{
		NodeID:      c.LineageID,
		NewParentID: doc.LineageID,
		Position:    &pos,
	}); err != nil {
		t.Fatalf("move: %v", err)
	}
	md, _ := b.RenderMarkdown(ctx, doc.LineageID)
	// Indices of the three words; gamma must come before alpha and beta.
	gi := strings.Index(md, "gamma")
	ai := strings.Index(md, "alpha")
	bi := strings.Index(md, "beta")
	if !(gi >= 0 && gi < ai && ai < bi) {
		t.Errorf("expected order gamma < alpha < beta, got md:\n%s", md)
	}

	// Reparent beta under alpha as a child.
	if err := b.Move(ctx, docs.MoveInput{
		NodeID:      bb.LineageID,
		NewParentID: a.LineageID,
	}); err != nil {
		t.Fatalf("reparent: %v", err)
	}
	md2, _ := b.RenderMarkdown(ctx, doc.LineageID)
	// beta should still be present.
	if !strings.Contains(md2, "beta") {
		t.Errorf("expected beta to still render, got:\n%s", md2)
	}
}

func TestGetOutlineDepth(t *testing.T) {
	store, graphID := newTestStore(t)
	b := docs.NewBuilder(store)
	ctx := context.Background()

	doc, _ := b.CreateDocument(ctx, docs.CreateDocumentInput{GraphID: graphID, Title: "Outline"})
	h1, _ := b.AppendHeading(ctx, docs.AppendHeadingInput{DocumentID: doc.LineageID, Level: 1, Text: "Top"})
	if _, err := b.AppendParagraph(ctx, docs.AppendParagraphInput{
		DocumentID: doc.LineageID,
		ParentID:   h1.LineageID,
		Text:       "body",
	}); err != nil {
		t.Fatal(err)
	}

	o, err := b.GetOutline(ctx, doc.LineageID, 0)
	if err != nil {
		t.Fatalf("outline: %v", err)
	}
	if o.Node.Kind != docs.KindDocument {
		t.Errorf("root kind = %q, want %q", o.Node.Kind, docs.KindDocument)
	}
	if len(o.Children) != 1 || o.Children[0].Node.Kind != docs.KindHeading {
		t.Errorf("expected one heading child, got %+v", o.Children)
	}
	if o.Children[0].Node.Level != 1 {
		t.Errorf("heading level = %d, want 1", o.Children[0].Node.Level)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
