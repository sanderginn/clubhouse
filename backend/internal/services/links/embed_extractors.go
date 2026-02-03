package links

import "context"

var defaultEmbedExtractors = []EmbedExtractor{
	SpotifyExtractor{},
}

// ExtractEmbed returns the first matching embed payload for a URL.
func ExtractEmbed(ctx context.Context, rawURL string) (*EmbedData, error) {
	for _, extractor := range defaultEmbedExtractors {
		if extractor.CanExtract(rawURL) {
			return extractor.Extract(ctx, rawURL)
		}
	}
	return nil, nil
}

// ApplyEmbedMetadata merges embed details into the metadata map.
func ApplyEmbedMetadata(metadata map[string]interface{}, embed *EmbedData) map[string]interface{} {
	if embed == nil {
		return metadata
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["embed"] = embed
	metadata["embed_url"] = embed.EmbedURL
	metadata["embed_provider"] = embed.Provider
	if embed.Height > 0 {
		metadata["embed_height"] = embed.Height
	}
	if embed.Width > 0 {
		metadata["embed_width"] = embed.Width
	}
	return metadata
}
