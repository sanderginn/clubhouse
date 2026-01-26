package services

import (
	"context"

	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	linkmeta "github.com/sanderginn/clubhouse/internal/services/links"
)

func fetchLinkMetadata(ctx context.Context, links []models.LinkRequest) []models.JSONMap {
	if len(links) == 0 {
		return nil
	}

	// Check if link metadata fetching is enabled
	if !GetConfigService().IsLinkMetadataEnabled() {
		return nil
	}

	metadata := make([]models.JSONMap, len(links))
	for i, link := range links {
		observability.RecordLinkMetadataFetchAttempt(ctx, 1)
		meta, err := linkmeta.FetchMetadata(ctx, link.URL)
		if err != nil || len(meta) == 0 {
			observability.RecordLinkMetadataFetchFailure(ctx, 1)
			continue
		}
		observability.RecordLinkMetadataFetchSuccess(ctx, 1)
		metadata[i] = models.JSONMap(meta)
	}

	return metadata
}
