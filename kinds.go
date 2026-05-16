package docs

// Node kinds used by memgraph-docs. All are namespaced with `doc.` so they
// don't collide with kinds from other clients sharing the same memgraph
// deployment.
const (
	KindDocument  = "doc.document"
	KindHeading   = "doc.heading"
	KindParagraph = "doc.paragraph"
	KindList      = "doc.list"
	KindListItem  = "doc.list_item"
	KindCodeBlock = "doc.code_block"
	KindQuote     = "doc.quote"
	KindMedia     = "doc.media"
	KindDivider   = "doc.divider"
)

// Edge kind connecting parents to children in a document tree.
const EdgeContains = "doc.contains"

// Media kinds used by docs.media nodes' metadata.kind field.
const (
	MediaImage = "image"
	MediaVideo = "video"
	MediaAudio = "audio"
	MediaFile  = "file"
)

// MaxHeadingLevel is the deepest CommonMark heading level we accept.
const MaxHeadingLevel = 6
