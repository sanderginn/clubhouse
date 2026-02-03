package links

import (
	"context"
	"testing"
)

func TestBandcampExtractorCanExtract(t *testing.T) {
	extractor := BandcampExtractor{}

	if !extractor.CanExtract("https://artist.bandcamp.com/album/test-album") {
		t.Fatalf("expected bandcamp url to be extractable")
	}
	if extractor.CanExtract("https://example.com/album/test") {
		t.Fatalf("expected non-bandcamp url to be ignored")
	}
}

func TestBandcampExtractorExtractFromHTMLAlbum(t *testing.T) {
	extractor := BandcampExtractor{}
	metaTags := map[string]string{
		"bc-page-properties": `{"item_type":"a","item_id":12345}`,
	}

	embed, err := extractor.ExtractFromHTML(
		context.Background(),
		"https://artist.bandcamp.com/album/test-album",
		nil,
		metaTags,
	)
	if err != nil {
		t.Fatalf("expected embed, got error: %v", err)
	}
	if embed.Provider != "bandcamp" {
		t.Fatalf("provider = %v, want bandcamp", embed.Provider)
	}
	if embed.Height != bandcampAlbumHeight {
		t.Fatalf("height = %v, want %d", embed.Height, bandcampAlbumHeight)
	}
	if embed.EmbedURL == "" || embed.Type != "iframe" {
		t.Fatalf("expected iframe embed url to be set")
	}
}

func TestBandcampExtractorExtractFromHTMLEscapedTrack(t *testing.T) {
	extractor := BandcampExtractor{}
	metaTags := map[string]string{
		"bc-page-properties": "{&quot;item_type&quot;:&quot;t&quot;,&quot;item_id&quot;:67890}",
	}

	embed, err := extractor.ExtractFromHTML(
		context.Background(),
		"https://artist.bandcamp.com/track/test-track",
		nil,
		metaTags,
	)
	if err != nil {
		t.Fatalf("expected embed, got error: %v", err)
	}
	if embed.Height != bandcampTrackHeight {
		t.Fatalf("height = %v, want %d", embed.Height, bandcampTrackHeight)
	}
}
