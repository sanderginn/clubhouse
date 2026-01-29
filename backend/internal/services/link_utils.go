package services

import "github.com/sanderginn/clubhouse/internal/models"

func linkRequestsMatchURLs(existing []string, links []models.LinkRequest) bool {
	if len(existing) != len(links) {
		return false
	}
	for i, link := range links {
		if existing[i] != link.URL {
			return false
		}
	}
	return true
}
