package services

import (
	"context"

	"github.com/sanderginn/clubhouse/internal/models"
	linkmeta "github.com/sanderginn/clubhouse/internal/services/links"
)

func fetchLinkMetadata(ctx context.Context, links []models.LinkRequest) []models.JSONMap {
	if len(links) == 0 {
		return nil
	}

	metadata := make([]models.JSONMap, len(links))
	for i, link := range links {
		meta, err := linkmeta.FetchMetadata(ctx, link.URL)
		if err != nil || len(meta) == 0 {
			continue
		}
		metadata[i] = models.JSONMap(meta)
	}

	return metadata
}
