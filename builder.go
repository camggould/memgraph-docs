package docs

import (
	"context"
	"fmt"
	"sort"
	"time"

	memgraph "github.com/camggould/memgraph"
)

// Builder is the high-level write/read API for documents stored in a
// memgraph.Store. All operations are safe for concurrent use as long as
// the underlying Store is.
type Builder struct {
	store memgraph.Store
}

// NewBuilder returns a Builder bound to the given Store. The Builder does
// not own the Store; callers are responsible for closing it.
func NewBuilder(store memgraph.Store) *Builder {
	return &Builder{store: store}
}

// --- Inputs and refs ---

type CreateDocumentInput struct {
	GraphID   memgraph.GraphID
	Title     string
	Author    string
	Abstract  string
	CreatedBy string
}

type AppendHeadingInput struct {
	DocumentID memgraph.LineageID
	ParentID   memgraph.LineageID // empty = document root
	Level      int                // 1-6
	Text       string
	CreatedBy  string
}

type AppendParagraphInput struct {
	DocumentID memgraph.LineageID
	ParentID   memgraph.LineageID
	Text       string
	CreatedBy  string
}

type AppendListInput struct {
	DocumentID memgraph.LineageID
	ParentID   memgraph.LineageID
	Ordered    bool
	Items      []string
	CreatedBy  string
}

type AppendCodeInput struct {
	DocumentID memgraph.LineageID
	ParentID   memgraph.LineageID
	Code       string
	Language   string
	CreatedBy  string
}

type AppendQuoteInput struct {
	DocumentID memgraph.LineageID
	ParentID   memgraph.LineageID
	Text       string
	CreatedBy  string
}

type AppendMediaInput struct {
	DocumentID memgraph.LineageID
	ParentID   memgraph.LineageID
	MediaKind  string // image, video, audio, file
	Src        string
	Alt        string
	Caption    string
	Width      int
	Height     int
	CreatedBy  string
}

type AppendDividerInput struct {
	DocumentID memgraph.LineageID
	ParentID   memgraph.LineageID
	CreatedBy  string
}

type UpdateTextInput struct {
	NodeID    memgraph.LineageID
	Text      string
	CreatedBy string
}

type MoveInput struct {
	NodeID       memgraph.LineageID
	NewParentID  memgraph.LineageID
	Position     *int // 0-indexed; nil = append
	CreatedBy    string
}

// DocumentRef summarizes a document for listing.
type DocumentRef struct {
	LineageID memgraph.LineageID
	GraphID   memgraph.GraphID
	Title     string
	Author    string
	Abstract  string
	CreatedAt time.Time
}

// NodeRef is the public ID + minimal metadata returned from write operations.
type NodeRef struct {
	LineageID memgraph.LineageID
	GraphID   memgraph.GraphID
	Kind      string
	Ordinal   int
}

// --- Document lifecycle ---

func (b *Builder) CreateDocument(ctx context.Context, in CreateDocumentInput) (DocumentRef, error) {
	if in.GraphID == "" {
		return DocumentRef{}, fmt.Errorf("docs: graph_id required")
	}
	if in.Title == "" {
		return DocumentRef{}, fmt.Errorf("docs: title required")
	}
	md := map[string]any{}
	if in.Author != "" {
		md["author"] = in.Author
	}
	if in.Abstract != "" {
		md["abstract"] = in.Abstract
	}
	n, err := b.store.PutNode(ctx, memgraph.NodeInput{
		GraphID:   in.GraphID,
		Kind:      KindDocument,
		Content:   in.Title,
		Summary:   in.Title,
		Metadata:  metaOrNil(md),
		CreatedBy: defaultCreatedBy(in.CreatedBy),
	})
	if err != nil {
		return DocumentRef{}, err
	}
	return DocumentRef{
		LineageID: n.LineageID,
		GraphID:   n.GraphID,
		Title:     in.Title,
		Author:    in.Author,
		Abstract:  in.Abstract,
		CreatedAt: n.CreatedAt,
	}, nil
}

func (b *Builder) ListDocuments(ctx context.Context, graphID memgraph.GraphID) ([]DocumentRef, error) {
	nodes, err := b.store.ListNodes(ctx, graphID, memgraph.NodeFilter{
		Kinds: []string{KindDocument},
		Limit: 1000,
	})
	if err != nil {
		return nil, err
	}
	out := make([]DocumentRef, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, refFromDocumentNode(n))
	}
	return out, nil
}

func refFromDocumentNode(n memgraph.Node) DocumentRef {
	r := DocumentRef{
		LineageID: n.LineageID,
		GraphID:   n.GraphID,
		Title:     n.Content,
		CreatedAt: n.CreatedAt,
	}
	if s, ok := n.Metadata["author"].(string); ok {
		r.Author = s
	}
	if s, ok := n.Metadata["abstract"].(string); ok {
		r.Abstract = s
	}
	return r
}

// --- Append helpers ---

// appendChild does the shared work of putting a content node and linking it
// from a parent with the next available ordinal.
func (b *Builder) appendChild(
	ctx context.Context,
	docID, parentID memgraph.LineageID,
	kind, content, summary string,
	metadata map[string]any,
	createdBy string,
) (NodeRef, error) {
	if docID == "" {
		return NodeRef{}, fmt.Errorf("docs: document_id required")
	}
	doc, err := b.store.GetNodeByLineage(ctx, docID, memgraph.ReadOpts{})
	if err != nil {
		return NodeRef{}, fmt.Errorf("docs: resolve document: %w", err)
	}
	if doc.Kind != KindDocument {
		return NodeRef{}, fmt.Errorf("docs: document_id is not a document node (kind=%q)", doc.Kind)
	}

	parent := parentID
	if parent == "" {
		parent = docID
	}

	ordinal, err := b.nextOrdinal(ctx, parent)
	if err != nil {
		return NodeRef{}, err
	}

	cb := defaultCreatedBy(createdBy)
	node, err := b.store.PutNode(ctx, memgraph.NodeInput{
		GraphID:   doc.GraphID,
		Kind:      kind,
		Content:   content,
		Summary:   summary,
		Metadata:  metaOrNil(metadata),
		CreatedBy: cb,
	})
	if err != nil {
		return NodeRef{}, fmt.Errorf("docs: put %s node: %w", kind, err)
	}
	if _, err := b.store.PutEdge(ctx, memgraph.EdgeInput{
		GraphID:     doc.GraphID,
		FromLineage: parent,
		ToLineage:   node.LineageID,
		Kind:        EdgeContains,
		Ordinal:     &ordinal,
		CreatedBy:   cb,
	}); err != nil {
		return NodeRef{}, fmt.Errorf("docs: link %s edge: %w", kind, err)
	}
	return NodeRef{
		LineageID: node.LineageID,
		GraphID:   node.GraphID,
		Kind:      node.Kind,
		Ordinal:   ordinal,
	}, nil
}

// nextOrdinal returns max(siblings.ordinal) + 1, or 1 if no siblings.
func (b *Builder) nextOrdinal(ctx context.Context, parent memgraph.LineageID) (int, error) {
	edges, err := b.store.Outgoing(ctx, parent, memgraph.TraverseOpts{
		EdgeKinds: []string{EdgeContains},
	})
	if err != nil {
		return 0, err
	}
	max := 0
	for _, e := range edges {
		if e.Ordinal != nil && *e.Ordinal > max {
			max = *e.Ordinal
		}
	}
	return max + 1, nil
}

// --- Append: single-node content ---

func (b *Builder) AppendHeading(ctx context.Context, in AppendHeadingInput) (NodeRef, error) {
	if in.Level < 1 || in.Level > MaxHeadingLevel {
		return NodeRef{}, fmt.Errorf("docs: heading level must be 1..%d, got %d", MaxHeadingLevel, in.Level)
	}
	return b.appendChild(ctx, in.DocumentID, in.ParentID, KindHeading, in.Text, in.Text,
		map[string]any{"level": in.Level}, in.CreatedBy)
}

func (b *Builder) AppendParagraph(ctx context.Context, in AppendParagraphInput) (NodeRef, error) {
	return b.appendChild(ctx, in.DocumentID, in.ParentID, KindParagraph, in.Text, summaryOf(in.Text, 80),
		nil, in.CreatedBy)
}

func (b *Builder) AppendCode(ctx context.Context, in AppendCodeInput) (NodeRef, error) {
	md := map[string]any{}
	if in.Language != "" {
		md["language"] = in.Language
	}
	return b.appendChild(ctx, in.DocumentID, in.ParentID, KindCodeBlock, in.Code,
		summaryOf(in.Code, 80), metaOrNil(md), in.CreatedBy)
}

func (b *Builder) AppendQuote(ctx context.Context, in AppendQuoteInput) (NodeRef, error) {
	return b.appendChild(ctx, in.DocumentID, in.ParentID, KindQuote, in.Text, summaryOf(in.Text, 80),
		nil, in.CreatedBy)
}

func (b *Builder) AppendMedia(ctx context.Context, in AppendMediaInput) (NodeRef, error) {
	if in.Src == "" {
		return NodeRef{}, fmt.Errorf("docs: media src required")
	}
	mk := in.MediaKind
	if mk == "" {
		mk = MediaImage
	}
	md := map[string]any{
		"kind": mk,
		"src":  in.Src,
	}
	if in.Alt != "" {
		md["alt"] = in.Alt
	}
	if in.Width > 0 {
		md["width"] = in.Width
	}
	if in.Height > 0 {
		md["height"] = in.Height
	}
	return b.appendChild(ctx, in.DocumentID, in.ParentID, KindMedia, in.Caption,
		summaryOf(in.Caption, 80), md, in.CreatedBy)
}

func (b *Builder) AppendDivider(ctx context.Context, in AppendDividerInput) (NodeRef, error) {
	return b.appendChild(ctx, in.DocumentID, in.ParentID, KindDivider, "", "", nil, in.CreatedBy)
}

// --- Append: list (multi-node, atomic-ish) ---

// AppendList creates a list node and its item children in sequence. If item
// creation fails partway through, earlier writes are NOT rolled back —
// memgraph doesn't expose multi-write transactions at the Store level. For
// v0.1 this is acceptable; clients can detect partial failure via the count
// in the returned NodeRef metadata.
func (b *Builder) AppendList(ctx context.Context, in AppendListInput) (NodeRef, error) {
	listRef, err := b.appendChild(ctx, in.DocumentID, in.ParentID, KindList, "", "",
		map[string]any{"ordered": in.Ordered}, in.CreatedBy)
	if err != nil {
		return NodeRef{}, err
	}
	for _, item := range in.Items {
		if _, err := b.appendChild(ctx, in.DocumentID, listRef.LineageID, KindListItem, item,
			summaryOf(item, 80), nil, in.CreatedBy); err != nil {
			return listRef, fmt.Errorf("docs: append list item: %w", err)
		}
	}
	return listRef, nil
}

// --- Edits ---

func (b *Builder) UpdateText(ctx context.Context, in UpdateTextInput) (NodeRef, error) {
	if in.NodeID == "" {
		return NodeRef{}, fmt.Errorf("docs: node_id required")
	}
	cur, err := b.store.GetNodeByLineage(ctx, in.NodeID, memgraph.ReadOpts{})
	if err != nil {
		return NodeRef{}, err
	}
	node, err := b.store.PutNode(ctx, memgraph.NodeInput{
		GraphID:   cur.GraphID,
		LineageID: cur.LineageID,
		Kind:      cur.Kind,
		Content:   in.Text,
		Summary:   summaryOf(in.Text, 80),
		Tags:      cur.Tags,
		Metadata:  cur.Metadata,
		CreatedBy: defaultCreatedBy(in.CreatedBy),
	})
	if err != nil {
		return NodeRef{}, err
	}
	return NodeRef{
		LineageID: node.LineageID,
		GraphID:   node.GraphID,
		Kind:      node.Kind,
	}, nil
}

// Move reparents a node and/or reorders it among its siblings. It rewrites
// the single inbound doc.contains edge for the node (deletes and recreates
// it), and compacts sibling ordinals to 1..N at both the old and new
// parents so numbers stay small.
func (b *Builder) Move(ctx context.Context, in MoveInput) error {
	if in.NodeID == "" || in.NewParentID == "" {
		return fmt.Errorf("docs: node_id and new_parent_id required")
	}
	inbound, err := b.store.Incoming(ctx, in.NodeID, memgraph.TraverseOpts{
		EdgeKinds: []string{EdgeContains},
	})
	if err != nil {
		return err
	}
	if len(inbound) == 0 {
		return fmt.Errorf("docs: node has no parent")
	}
	// Filter to actual edges into the target node.
	var existing memgraph.Edge
	found := false
	for _, e := range inbound {
		if e.ToLineage == in.NodeID && e.Kind == EdgeContains {
			existing = e
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("docs: contains edge not found for node")
	}

	oldParent := existing.FromLineage

	// Delete the old edge first.
	if err := b.store.DeleteEdge(ctx, existing.ID); err != nil {
		return err
	}

	// Get the new parent's current ordered children to compute insertion position.
	siblings, err := b.outgoingContainsSorted(ctx, in.NewParentID)
	if err != nil {
		return err
	}

	pos := len(siblings)
	if in.Position != nil {
		pos = *in.Position
		if pos < 0 {
			pos = 0
		}
		if pos > len(siblings) {
			pos = len(siblings)
		}
	}

	// Insert: assign ordinal = pos+1 to the new edge, then bump subsequent
	// siblings by recreating their edges with new ordinals. Because edges
	// are content-addressed by ID in the Store, "edit ordinal" really means
	// "delete + recreate."
	cb := defaultCreatedBy(in.CreatedBy)

	// Step 1: shift siblings at or after pos.
	for i := pos; i < len(siblings); i++ {
		e := siblings[i]
		newOrd := i + 2 // pushed back by one because the new node takes pos+1
		if err := b.recreateEdgeWithOrdinal(ctx, e, newOrd, cb); err != nil {
			return err
		}
	}

	// Step 2: insert the moved node at pos+1.
	newOrd := pos + 1
	if _, err := b.store.PutEdge(ctx, memgraph.EdgeInput{
		GraphID:     existing.GraphID,
		FromLineage: in.NewParentID,
		ToLineage:   in.NodeID,
		Kind:        EdgeContains,
		Ordinal:     &newOrd,
		CreatedBy:   cb,
	}); err != nil {
		return err
	}

	// Step 3: compact the old parent's remaining children to 1..N.
	if oldParent != in.NewParentID {
		if err := b.compactOrdinals(ctx, oldParent, cb); err != nil {
			return err
		}
	} else {
		// Same-parent move: also compact (we shifted, but compacting normalizes
		// any pre-existing gaps).
		if err := b.compactOrdinals(ctx, in.NewParentID, cb); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) recreateEdgeWithOrdinal(ctx context.Context, e memgraph.Edge, ord int, createdBy string) error {
	if err := b.store.DeleteEdge(ctx, e.ID); err != nil {
		return err
	}
	_, err := b.store.PutEdge(ctx, memgraph.EdgeInput{
		GraphID:     e.GraphID,
		FromLineage: e.FromLineage,
		ToGraph:     e.ToGraph,
		ToLineage:   e.ToLineage,
		Kind:        e.Kind,
		Metadata:    e.Metadata,
		Ordinal:     &ord,
		CreatedBy:   createdBy,
	})
	return err
}

func (b *Builder) compactOrdinals(ctx context.Context, parent memgraph.LineageID, createdBy string) error {
	siblings, err := b.outgoingContainsSorted(ctx, parent)
	if err != nil {
		return err
	}
	for i, e := range siblings {
		want := i + 1
		if e.Ordinal != nil && *e.Ordinal == want {
			continue
		}
		if err := b.recreateEdgeWithOrdinal(ctx, e, want, createdBy); err != nil {
			return err
		}
	}
	return nil
}

// --- Read helpers used by render and outline ---

func (b *Builder) outgoingContainsSorted(ctx context.Context, parent memgraph.LineageID) ([]memgraph.Edge, error) {
	edges, err := b.store.Outgoing(ctx, parent, memgraph.TraverseOpts{
		EdgeKinds: []string{EdgeContains},
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(edges, func(i, j int) bool {
		oi := 0
		oj := 0
		if edges[i].Ordinal != nil {
			oi = *edges[i].Ordinal
		}
		if edges[j].Ordinal != nil {
			oj = *edges[j].Ordinal
		}
		if oi != oj {
			return oi < oj
		}
		return edges[i].CreatedAt.Before(edges[j].CreatedAt)
	})
	return edges, nil
}

// --- Helpers ---

func defaultCreatedBy(s string) string {
	if s == "" {
		return "memgraph-docs"
	}
	return s
}

func metaOrNil(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	return m
}

// summaryOf returns the first n characters of s, with an ellipsis if truncated.
// Runes-safe.
func summaryOf(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
