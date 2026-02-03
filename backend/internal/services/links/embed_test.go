package links

import (
	"encoding/json"
	"testing"
)

func TestEmbedDataJSON(t *testing.T) {
	embed := EmbedData{
		Type:     "iframe",
		Provider: "youtube",
		EmbedURL: "https://www.youtube.com/embed/abc123",
		Width:    560,
		Height:   315,
	}

	payload, err := json.Marshal(embed)
	if err != nil {
		t.Fatalf("marshal embed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal embed: %v", err)
	}

	if decoded["type"] != "iframe" {
		t.Fatalf("type = %v, want iframe", decoded["type"])
	}
	if decoded["provider"] != "youtube" {
		t.Fatalf("provider = %v, want youtube", decoded["provider"])
	}
	if decoded["embed_url"] != "https://www.youtube.com/embed/abc123" {
		t.Fatalf("embed_url = %v, want embed url", decoded["embed_url"])
	}
	if decoded["width"] != float64(560) {
		t.Fatalf("width = %v, want 560", decoded["width"])
	}
	if decoded["height"] != float64(315) {
		t.Fatalf("height = %v, want 315", decoded["height"])
	}
}

func TestLinkMetadataEmbedOmitEmpty(t *testing.T) {
	metadata := LinkMetadata{
		Title:        "Title",
		Description:  "Description",
		Image:        "https://example.com/image.png",
		CanonicalURL: "https://example.com/page",
	}

	payload, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	if _, ok := decoded["embed"]; ok {
		t.Fatalf("expected embed to be omitted when nil")
	}
}

func TestLinkMetadataEmbedPresent(t *testing.T) {
	metadata := LinkMetadata{
		Title:        "Title",
		Description:  "Description",
		Image:        "https://example.com/image.png",
		CanonicalURL: "https://example.com/page",
		Embed: &EmbedData{
			Type:     "oembed",
			Provider: "spotify",
			EmbedURL: "https://open.spotify.com/embed/track/xyz",
		},
	}

	payload, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	embed, ok := decoded["embed"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected embed object to be present")
	}
	if embed["type"] != "oembed" {
		t.Fatalf("embed.type = %v, want oembed", embed["type"])
	}
	if embed["provider"] != "spotify" {
		t.Fatalf("embed.provider = %v, want spotify", embed["provider"])
	}
}
