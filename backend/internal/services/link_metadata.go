package services

import (
	"context"
	"time"

	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
	linkmeta "github.com/sanderginn/clubhouse/internal/services/links"
)

func fetchLinkMetadata(ctx context.Context, links []models.LinkRequest, sectionType string) []models.JSONMap {
	if len(links) == 0 {
		return nil
	}

	// Check if link metadata fetching is enabled
	if !GetConfigService().IsLinkMetadataEnabled() {
		return nil
	}

	metadata := make([]models.JSONMap, len(links))
	metadataCtx := linkmeta.WithMetadataSectionType(ctx, sectionType)
	for i, link := range links {
		if linkmeta.IsInternalUploadURL(link.URL) {
			continue
		}
		embed, _ := linkmeta.ExtractEmbed(metadataCtx, link.URL)
		observability.RecordLinkMetadataFetchAttempt(metadataCtx, 1)
		start := time.Now()
		meta, err := linkmeta.FetchMetadata(metadataCtx, link.URL)
		observability.RecordLinkMetadataFetchDuration(metadataCtx, time.Since(start))
		domain := linkmeta.ExtractDomain(link.URL)
		if err != nil || len(meta) == 0 {
			errorType := linkmeta.ClassifyFetchError(err)
			if err == nil {
				errorType = "empty_metadata"
			}
			observability.RecordLinkMetadataFetchFailure(metadataCtx, 1, domain, errorType)
			if err != nil {
				observability.LogWarn(metadataCtx, "link metadata fetch failed", "link_url", link.URL, "link_domain", domain, "error_type", errorType, "error", err.Error())
			} else {
				observability.LogWarn(metadataCtx, "link metadata fetch empty", "link_url", link.URL, "link_domain", domain, "error_type", errorType)
			}
			if embed == nil {
				continue
			}
			meta = map[string]interface{}{}
		} else {
			observability.RecordLinkMetadataFetchSuccess(metadataCtx, 1)
		}
		meta = linkmeta.ApplyEmbedMetadata(meta, embed)
		metadata[i] = models.JSONMap(meta)
	}

	return metadata
}
