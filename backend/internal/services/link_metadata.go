package services

import (
	"context"
	"time"

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
		if linkmeta.IsInternalUploadURL(link.URL) {
			continue
		}
		observability.RecordLinkMetadataFetchAttempt(ctx, 1)
		start := time.Now()
		meta, err := linkmeta.FetchMetadata(ctx, link.URL)
		observability.RecordLinkMetadataFetchDuration(ctx, time.Since(start))
		domain := linkmeta.ExtractDomain(link.URL)
		if err != nil || len(meta) == 0 {
			errorType := linkmeta.ClassifyFetchError(err)
			if err == nil {
				errorType = "empty_metadata"
			}
			observability.RecordLinkMetadataFetchFailure(ctx, 1, domain, errorType)
			if err != nil {
				observability.LogWarn(ctx, "link metadata fetch failed", "link_url", link.URL, "link_domain", domain, "error_type", errorType, "error", err.Error())
			} else {
				observability.LogWarn(ctx, "link metadata fetch empty", "link_url", link.URL, "link_domain", domain, "error_type", errorType)
			}
			continue
		}
		observability.RecordLinkMetadataFetchSuccess(ctx, 1)
		metadata[i] = models.JSONMap(meta)
	}

	return metadata
}
