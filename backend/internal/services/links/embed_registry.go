package links

import "context"

var embedExtractors = []EmbedExtractor{
	BandcampExtractor{},
	NewSoundCloudExtractor(nil),
}

func extractEmbed(
	ctx context.Context,
	rawURL string,
	body []byte,
	metaTags map[string]string,
) *EmbedData {
	for _, extractor := range embedExtractors {
		if !extractor.CanExtract(rawURL) {
			continue
		}

		if htmlExtractor, ok := extractor.(HTMLEmbedExtractor); ok {
			embed, err := htmlExtractor.ExtractFromHTML(ctx, rawURL, body, metaTags)
			if err == nil && embed != nil {
				return embed
			}
		}

		embed, err := extractor.Extract(ctx, rawURL)
		if err == nil && embed != nil {
			return embed
		}
	}

	return nil
}
