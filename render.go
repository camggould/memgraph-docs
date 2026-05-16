package docs

import (
	"context"
	"fmt"
	"strings"

	memgraph "github.com/camggould/memgraph"
)

// Outline summarizes the structural skeleton of a document — headings and
// container nodes, without paragraph or code bodies. Used by docs_get_outline.
type Outline struct {
	Node     OutlineNode   `json:"node"`
	Children []Outline     `json:"children,omitempty"`
}

// OutlineNode is the minimal per-node payload returned in an outline.
type OutlineNode struct {
	LineageID memgraph.LineageID `json:"lineage_id"`
	Kind      string             `json:"kind"`
	Title     string             `json:"title,omitempty"`
	Level     int                `json:"level,omitempty"`
}

// GetOutline returns the document's structural tree. maxDepth limits how
// deep the traversal goes; 0 means unlimited.
func (b *Builder) GetOutline(ctx context.Context, docID memgraph.LineageID, maxDepth int) (Outline, error) {
	root, err := b.store.GetNodeByLineage(ctx, docID, memgraph.ReadOpts{})
	if err != nil {
		return Outline{}, err
	}
	if root.Kind != KindDocument {
		return Outline{}, fmt.Errorf("docs: node %q is not a document (kind=%q)", docID, root.Kind)
	}
	return b.outlineNode(ctx, root, 1, maxDepth)
}

func (b *Builder) outlineNode(ctx context.Context, n memgraph.Node, depth, maxDepth int) (Outline, error) {
	o := Outline{Node: outlineNodeFromNode(n)}
	if maxDepth > 0 && depth > maxDepth {
		return o, nil
	}
	edges, err := b.outgoingContainsSorted(ctx, n.LineageID)
	if err != nil {
		return o, err
	}
	for _, e := range edges {
		child, err := b.store.GetNodeByLineage(ctx, e.ToLineage, memgraph.ReadOpts{})
		if err != nil {
			return o, err
		}
		sub, err := b.outlineNode(ctx, child, depth+1, maxDepth)
		if err != nil {
			return o, err
		}
		o.Children = append(o.Children, sub)
	}
	return o, nil
}

func outlineNodeFromNode(n memgraph.Node) OutlineNode {
	on := OutlineNode{
		LineageID: n.LineageID,
		Kind:      n.Kind,
		Title:     n.Summary,
	}
	if on.Title == "" {
		on.Title = n.Content
	}
	if lvl, ok := n.Metadata["level"]; ok {
		switch v := lvl.(type) {
		case int:
			on.Level = v
		case float64:
			on.Level = int(v)
		}
	}
	return on
}

// RenderMarkdown walks the document tree and emits a CommonMark string.
// Unknown kinds render as plain paragraphs.
func (b *Builder) RenderMarkdown(ctx context.Context, docID memgraph.LineageID) (string, error) {
	root, err := b.store.GetNodeByLineage(ctx, docID, memgraph.ReadOpts{})
	if err != nil {
		return "", err
	}
	if root.Kind != KindDocument {
		return "", fmt.Errorf("docs: node %q is not a document (kind=%q)", docID, root.Kind)
	}
	var sb strings.Builder
	if err := b.renderDocument(ctx, &sb, root); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func (b *Builder) renderDocument(ctx context.Context, sb *strings.Builder, root memgraph.Node) error {
	// Title rendered as H1 only if the agent didn't already supply a top-
	// level heading. Heuristic: peek at the first child; if it's a heading
	// at level 1, skip the auto title.
	edges, err := b.outgoingContainsSorted(ctx, root.LineageID)
	if err != nil {
		return err
	}

	if !leadsWithH1(ctx, b, edges) && strings.TrimSpace(root.Content) != "" {
		sb.WriteString("# ")
		sb.WriteString(root.Content)
		sb.WriteString("\n\n")
	}
	if abs, ok := root.Metadata["abstract"].(string); ok && abs != "" {
		sb.WriteString("> ")
		sb.WriteString(abs)
		sb.WriteString("\n\n")
	}
	for _, e := range edges {
		if err := b.renderEdge(ctx, sb, e, 0); err != nil {
			return err
		}
	}
	return nil
}

// leadsWithH1 is best-effort and tolerates store errors (returns false).
func leadsWithH1(ctx context.Context, b *Builder, edges []memgraph.Edge) bool {
	if len(edges) == 0 {
		return false
	}
	first, err := b.store.GetNodeByLineage(ctx, edges[0].ToLineage, memgraph.ReadOpts{})
	if err != nil {
		return false
	}
	if first.Kind != KindHeading {
		return false
	}
	if lvl, ok := first.Metadata["level"]; ok {
		switch v := lvl.(type) {
		case int:
			return v == 1
		case float64:
			return int(v) == 1
		}
	}
	return false
}

// renderEdge renders the child node pointed at by edge, then recurses into
// that node's own contains-children. `listDepth` is the nesting level when
// rendering list items (used for indenting nested lists).
func (b *Builder) renderEdge(ctx context.Context, sb *strings.Builder, e memgraph.Edge, listDepth int) error {
	n, err := b.store.GetNodeByLineage(ctx, e.ToLineage, memgraph.ReadOpts{})
	if err != nil {
		return err
	}
	return b.renderNode(ctx, sb, n, listDepth)
}

func (b *Builder) renderNode(ctx context.Context, sb *strings.Builder, n memgraph.Node, listDepth int) error {
	switch n.Kind {
	case KindHeading:
		level := 1
		if lvl, ok := n.Metadata["level"]; ok {
			switch v := lvl.(type) {
			case int:
				level = v
			case float64:
				level = int(v)
			}
		}
		sb.WriteString(strings.Repeat("#", clampHeadingLevel(level)))
		sb.WriteString(" ")
		sb.WriteString(n.Content)
		sb.WriteString("\n\n")
		return b.renderChildren(ctx, sb, n, listDepth)

	case KindParagraph:
		sb.WriteString(n.Content)
		sb.WriteString("\n\n")
		return b.renderChildren(ctx, sb, n, listDepth)

	case KindCodeBlock:
		lang := ""
		if s, ok := n.Metadata["language"].(string); ok {
			lang = s
		}
		sb.WriteString("```")
		sb.WriteString(lang)
		sb.WriteString("\n")
		sb.WriteString(n.Content)
		if !strings.HasSuffix(n.Content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n\n")
		return nil

	case KindQuote:
		for _, line := range strings.Split(n.Content, "\n") {
			sb.WriteString("> ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		return b.renderChildren(ctx, sb, n, listDepth)

	case KindMedia:
		src, _ := n.Metadata["src"].(string)
		alt, _ := n.Metadata["alt"].(string)
		mk, _ := n.Metadata["kind"].(string)
		switch mk {
		case MediaImage, "":
			sb.WriteString("![")
			sb.WriteString(alt)
			sb.WriteString("](")
			sb.WriteString(src)
			sb.WriteString(")\n")
		default:
			label := alt
			if label == "" {
				label = mk
			}
			sb.WriteString("[")
			sb.WriteString(label)
			sb.WriteString("](")
			sb.WriteString(src)
			sb.WriteString(")\n")
		}
		if strings.TrimSpace(n.Content) != "" {
			sb.WriteString("*")
			sb.WriteString(n.Content)
			sb.WriteString("*\n")
		}
		sb.WriteString("\n")
		return nil

	case KindDivider:
		sb.WriteString("---\n\n")
		return nil

	case KindList:
		ordered := false
		if v, ok := n.Metadata["ordered"].(bool); ok {
			ordered = v
		}
		return b.renderList(ctx, sb, n, listDepth, ordered)

	case KindListItem:
		// Standalone list_item shouldn't normally render outside a list, but
		// be defensive and emit it as a bullet.
		return b.renderListItem(ctx, sb, n, listDepth, false, 0)

	default:
		// Unknown kind — emit content as a paragraph if non-empty.
		if strings.TrimSpace(n.Content) != "" {
			sb.WriteString(n.Content)
			sb.WriteString("\n\n")
		}
		return b.renderChildren(ctx, sb, n, listDepth)
	}
}

func clampHeadingLevel(l int) int {
	if l < 1 {
		return 1
	}
	if l > MaxHeadingLevel {
		return MaxHeadingLevel
	}
	return l
}

func (b *Builder) renderChildren(ctx context.Context, sb *strings.Builder, parent memgraph.Node, listDepth int) error {
	edges, err := b.outgoingContainsSorted(ctx, parent.LineageID)
	if err != nil {
		return err
	}
	for _, e := range edges {
		if err := b.renderEdge(ctx, sb, e, listDepth); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) renderList(ctx context.Context, sb *strings.Builder, list memgraph.Node, depth int, ordered bool) error {
	edges, err := b.outgoingContainsSorted(ctx, list.LineageID)
	if err != nil {
		return err
	}
	for i, e := range edges {
		child, err := b.store.GetNodeByLineage(ctx, e.ToLineage, memgraph.ReadOpts{})
		if err != nil {
			return err
		}
		if err := b.renderListItem(ctx, sb, child, depth, ordered, i+1); err != nil {
			return err
		}
	}
	if depth == 0 {
		sb.WriteString("\n")
	}
	return nil
}

func (b *Builder) renderListItem(ctx context.Context, sb *strings.Builder, item memgraph.Node, depth int, ordered bool, index int) error {
	indent := strings.Repeat("  ", depth)
	sb.WriteString(indent)
	if ordered {
		sb.WriteString(fmt.Sprintf("%d. ", index))
	} else {
		sb.WriteString("- ")
	}
	sb.WriteString(item.Content)
	sb.WriteString("\n")

	// Nested content: render children with indentation. For nested lists we
	// pass depth+1 so renderList indents bullets one more level.
	edges, err := b.outgoingContainsSorted(ctx, item.LineageID)
	if err != nil {
		return err
	}
	for _, e := range edges {
		child, err := b.store.GetNodeByLineage(ctx, e.ToLineage, memgraph.ReadOpts{})
		if err != nil {
			return err
		}
		if child.Kind == KindList {
			ord := false
			if v, ok := child.Metadata["ordered"].(bool); ok {
				ord = v
			}
			if err := b.renderList(ctx, sb, child, depth+1, ord); err != nil {
				return err
			}
		} else {
			// Indent non-list children under the bullet.
			var inner strings.Builder
			if err := b.renderNode(ctx, &inner, child, depth+1); err != nil {
				return err
			}
			for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
				sb.WriteString(indent)
				sb.WriteString("  ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
	}
	return nil
}
