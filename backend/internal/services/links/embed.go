package links

import "context"

// EmbedData represents interactive embed details for a link.
type EmbedData struct {
	Type     string `json:"type"` // "iframe", "oembed", "native"
	Provider string `json:"provider"`
	EmbedURL string `json:"embed_url"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// LinkMetadata represents structured metadata for a link.
type LinkMetadata struct {
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Image        string     `json:"image"`
	CanonicalURL string     `json:"canonical_url"`
	Embed        *EmbedData `json:"embed,omitempty"`
}

// EmbedExtractor defines a provider-specific embed extractor.
type EmbedExtractor interface {
	CanExtract(url string) bool
	Extract(ctx context.Context, url string) (*EmbedData, error)
}
