package links

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/sanderginn/clubhouse/internal/models"
	"github.com/sanderginn/clubhouse/internal/observability"
)

const (
	tmdbImageBaseURL         = "https://image.tmdb.org/t/p"
	tmdbPosterSize           = "w500"
	tmdbBackdropSize         = "w1280"
	movieMetadataCastMaxSize = 5
	rottenTomatoesBaseURL    = "https://www.rottentomatoes.com"
)

var (
	imdbIDPattern           = regexp.MustCompile(`^tt\d+$`)
	leadingDigitsRegexp     = regexp.MustCompile(`^(\d+)`)
	trailingRTYearPattern   = regexp.MustCompile(`[_-](19\d{2}|20\d{2})$`)
	rtDelimiterStripPattern = regexp.MustCompile(`[_-]+`)
	rtSlugStripPattern      = regexp.MustCompile(`[^a-z0-9]+`)
)

type MovieData = models.MovieData

func ParseMovieMetadata(ctx context.Context, rawURL string, client *TMDBClient, omdbClient *OMDBClient) (*MovieData, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if client == nil {
		return nil, errors.New("tmdb client is required")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil
	}

	host := strings.ToLower(strings.TrimSpace(parsedURL.Hostname()))
	segments := splitURLPath(parsedURL.Path)

	switch {
	case isIMDbHost(host):
		imdbID, ok := parseIMDbID(segments)
		if !ok {
			return nil, nil
		}
		return parseIMDbMovieMetadata(ctx, client, omdbClient, imdbID)
	case isTMDBHost(host):
		mediaType, tmdbID, ok := parseTMDBPath(segments)
		if !ok {
			return nil, nil
		}
		return parseTMDBMovieMetadata(ctx, client, omdbClient, mediaType, tmdbID, false)
	case isLetterboxdHost(host):
		slug, ok := parseLetterboxdSlug(segments)
		if !ok {
			return nil, nil
		}
		return parseLetterboxdMovieMetadata(ctx, client, omdbClient, slug)
	case isRottenTomatoesHost(host):
		mediaType, slug, ok := parseRottenTomatoesPath(segments)
		if !ok {
			return nil, nil
		}
		return parseRottenTomatoesMovieMetadata(ctx, client, omdbClient, mediaType, slug)
	default:
		return nil, nil
	}
}

func isIMDbHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "imdb.com" || strings.HasSuffix(host, ".imdb.com")
}

func isTMDBHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "themoviedb.org" || strings.HasSuffix(host, ".themoviedb.org")
}

func isLetterboxdHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "letterboxd.com" || strings.HasSuffix(host, ".letterboxd.com")
}

func isRottenTomatoesHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "rottentomatoes.com" || strings.HasSuffix(host, ".rottentomatoes.com")
}

func splitURLPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}

	rawSegments := strings.Split(path, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		unescaped, err := url.PathUnescape(segment)
		if err != nil {
			unescaped = segment
		}
		segments = append(segments, unescaped)
	}
	return segments
}

func parseIMDbID(segments []string) (string, bool) {
	if len(segments) < 2 || !strings.EqualFold(segments[0], "title") {
		return "", false
	}

	imdbID := strings.ToLower(strings.TrimSpace(segments[1]))
	if !imdbIDPattern.MatchString(imdbID) {
		return "", false
	}
	return imdbID, true
}

func parseTMDBPath(segments []string) (string, int, bool) {
	if len(segments) < 2 {
		return "", 0, false
	}

	mediaType := strings.ToLower(strings.TrimSpace(segments[0]))
	if mediaType != "movie" && mediaType != "tv" {
		return "", 0, false
	}

	match := leadingDigitsRegexp.FindStringSubmatch(strings.TrimSpace(segments[1]))
	if len(match) != 2 {
		return "", 0, false
	}

	tmdbID, err := strconv.Atoi(match[1])
	if err != nil || tmdbID <= 0 {
		return "", 0, false
	}
	return mediaType, tmdbID, true
}

func parseLetterboxdSlug(segments []string) (string, bool) {
	if len(segments) < 2 || !strings.EqualFold(segments[0], "film") {
		return "", false
	}

	slug := strings.TrimSpace(segments[1])
	if slug == "" {
		return "", false
	}
	return slug, true
}

func parseRottenTomatoesPath(segments []string) (string, string, bool) {
	if len(segments) < 2 {
		return "", "", false
	}

	category := strings.ToLower(strings.TrimSpace(segments[0]))
	var mediaType string
	switch category {
	case "m":
		mediaType = "movie"
	case "tv":
		mediaType = "tv"
	default:
		return "", "", false
	}

	slug := strings.TrimSpace(segments[1])
	if slug == "" {
		return "", "", false
	}

	return mediaType, slug, true
}

func parseIMDbMovieMetadata(ctx context.Context, client *TMDBClient, omdbClient *OMDBClient, imdbID string) (*MovieData, error) {
	result, err := client.FindByIMDBID(ctx, imdbID)
	if err != nil {
		return nil, fmt.Errorf("find tmdb result by imdb id %s: %w", imdbID, err)
	}

	if len(result.MovieResults) > 0 {
		return parseTMDBMovieMetadata(ctx, client, omdbClient, "movie", result.MovieResults[0].ID, false)
	}
	if len(result.TVResults) > 0 {
		return parseTMDBMovieMetadata(ctx, client, omdbClient, "tv", result.TVResults[0].ID, false)
	}

	return nil, nil
}

func parseTMDBMovieMetadata(ctx context.Context, client *TMDBClient, omdbClient *OMDBClient, mediaType string, tmdbID int, verifyIMDBID bool) (*MovieData, error) {
	mediaType = normalizeTMDBMediaType(mediaType)

	switch mediaType {
	case "movie":
		details, err := client.GetMovieDetails(ctx, tmdbID)
		if err != nil {
			return nil, fmt.Errorf("get tmdb movie details for id %d: %w", tmdbID, err)
		}
		movieData := movieDataFromMovieDetails(details, mediaType)
		imdbID := resolveTMDBIMDBID(ctx, client, mediaType, details.ID, verifyIMDBID, details.IMDBID, details.ExternalIDs.IMDBID)
		setMovieExternalLinks(movieData, mediaType, imdbID, "")
		enrichMovieDataWithOMDB(ctx, omdbClient, imdbID, movieData)
		return movieData, nil
	case "tv":
		details, err := client.GetTVDetails(ctx, tmdbID)
		if err != nil {
			return nil, fmt.Errorf("get tmdb tv details for id %d: %w", tmdbID, err)
		}
		movieData := movieDataFromTVDetails(details, mediaType)
		imdbID := resolveTMDBIMDBID(ctx, client, mediaType, details.ID, verifyIMDBID, details.ExternalIDs.IMDBID)
		setMovieExternalLinks(movieData, mediaType, imdbID, "")
		enrichMovieDataWithOMDB(ctx, omdbClient, imdbID, movieData)
		return movieData, nil
	default:
		return nil, nil
	}
}

func parseLetterboxdMovieMetadata(ctx context.Context, client *TMDBClient, omdbClient *OMDBClient, slug string) (*MovieData, error) {
	titleQuery := letterboxdSlugToTitle(slug)
	if titleQuery == "" {
		return nil, nil
	}

	results, err := client.SearchMovie(ctx, titleQuery)
	if err != nil {
		return nil, fmt.Errorf("search tmdb movie for letterboxd slug %s: %w", slug, err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	return parseTMDBMovieMetadata(ctx, client, omdbClient, "movie", results[0].ID, false)
}

func parseRottenTomatoesMovieMetadata(ctx context.Context, client *TMDBClient, omdbClient *OMDBClient, mediaType, slug string) (*MovieData, error) {
	titleQuery, releaseYear := rottenTomatoesSlugToTitle(slug)
	if titleQuery == "" {
		return nil, nil
	}

	switch mediaType {
	case "movie":
		results, err := client.SearchMovie(ctx, titleQuery)
		if err != nil {
			return nil, fmt.Errorf("search tmdb movie for rotten tomatoes slug %s: %w", slug, err)
		}
		match, ok := selectBestMovieSearchResult(titleQuery, releaseYear, results)
		if !ok {
			return nil, nil
		}
		movieData, err := parseTMDBMovieMetadata(ctx, client, omdbClient, "movie", match.ID, true)
		if err != nil || movieData == nil {
			return movieData, err
		}
		setMovieExternalLinks(movieData, "movie", movieData.IMDBID, slug)
		return movieData, nil
	case "tv":
		results, err := client.SearchTV(ctx, titleQuery)
		if err != nil {
			return nil, fmt.Errorf("search tmdb tv for rotten tomatoes slug %s: %w", slug, err)
		}
		match, ok := selectBestTVSearchResult(titleQuery, releaseYear, results)
		if !ok {
			return nil, nil
		}
		movieData, err := parseTMDBMovieMetadata(ctx, client, omdbClient, "tv", match.ID, true)
		if err != nil || movieData == nil {
			return movieData, err
		}
		setMovieExternalLinks(movieData, "tv", movieData.IMDBID, slug)
		return movieData, nil
	default:
		return nil, nil
	}
}

func resolveTMDBIMDBID(ctx context.Context, client *TMDBClient, mediaType string, tmdbID int, verify bool, candidates ...string) string {
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		if !imdbIDPattern.MatchString(normalized) {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}

		if !verify {
			return normalized
		}

		if imdbIDMatchesTMDBEntity(ctx, client, mediaType, tmdbID, normalized) {
			return normalized
		}
	}

	return ""
}

func imdbIDMatchesTMDBEntity(ctx context.Context, client *TMDBClient, mediaType string, tmdbID int, imdbID string) bool {
	if ctx == nil || client == nil || tmdbID <= 0 {
		return false
	}

	result, err := client.FindByIMDBID(ctx, imdbID)
	if err != nil {
		observability.LogWarn(
			ctx,
			"tmdb imdb verification request failed",
			"imdb_id",
			imdbID,
			"tmdb_id",
			strconv.Itoa(tmdbID),
			"media_type",
			mediaType,
			"error",
			err.Error(),
		)
		return false
	}

	switch normalizeTMDBMediaType(mediaType) {
	case "movie":
		for _, movie := range result.MovieResults {
			if movie.ID == tmdbID {
				return true
			}
		}
	case "tv":
		for _, tv := range result.TVResults {
			if tv.ID == tmdbID {
				return true
			}
		}
	}

	observability.LogDebug(
		ctx,
		"tmdb imdb verification mismatch",
		"imdb_id",
		imdbID,
		"tmdb_id",
		strconv.Itoa(tmdbID),
		"media_type",
		mediaType,
	)

	return false
}

func normalizeTMDBMediaType(mediaType string) string {
	normalized := strings.ToLower(strings.TrimSpace(mediaType))
	if normalized == "series" {
		return "tv"
	}
	return normalized
}

func letterboxdSlugToTitle(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	slug = strings.Trim(slug, "/")
	if slug == "" {
		return ""
	}

	title := strings.ReplaceAll(slug, "-", " ")
	return strings.Join(strings.Fields(title), " ")
}

func rottenTomatoesSlugToTitle(slug string) (string, int) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	slug = strings.Trim(slug, "/")
	if slug == "" {
		return "", 0
	}

	year := 0
	if match := trailingRTYearPattern.FindStringSubmatch(slug); len(match) == 2 {
		if parsedYear, err := strconv.Atoi(match[1]); err == nil {
			year = parsedYear
		}
		slug = strings.TrimSuffix(slug, match[0])
	}

	title := rtDelimiterStripPattern.ReplaceAllString(slug, " ")
	title = strings.Join(strings.Fields(title), " ")
	return title, year
}

func selectBestMovieSearchResult(query string, year int, results []MovieSearchResult) (MovieSearchResult, bool) {
	bestIndex := -1
	bestScore := 0
	bestVoteAverage := -1.0

	for i, result := range results {
		score := scoreTMDBTitleMatch(query, result.Title, year, yearFromDate(result.ReleaseDate))
		if score < 40 {
			continue
		}

		if score > bestScore || (score == bestScore && result.VoteAverage > bestVoteAverage) {
			bestIndex = i
			bestScore = score
			bestVoteAverage = result.VoteAverage
		}
	}

	if bestIndex < 0 {
		return MovieSearchResult{}, false
	}
	return results[bestIndex], true
}

func selectBestTVSearchResult(query string, year int, results []TVSearchResult) (TVSearchResult, bool) {
	bestIndex := -1
	bestScore := 0
	bestVoteAverage := -1.0

	for i, result := range results {
		score := scoreTMDBTitleMatch(query, result.Name, year, yearFromDate(result.FirstAirDate))
		if score < 40 {
			continue
		}

		if score > bestScore || (score == bestScore && result.VoteAverage > bestVoteAverage) {
			bestIndex = i
			bestScore = score
			bestVoteAverage = result.VoteAverage
		}
	}

	if bestIndex < 0 {
		return TVSearchResult{}, false
	}
	return results[bestIndex], true
}

func scoreTMDBTitleMatch(query, candidate string, expectedYear, candidateYear int) int {
	queryNormalized := normalizeTMDBMatchValue(query)
	candidateNormalized := normalizeTMDBMatchValue(candidate)
	if queryNormalized == "" || candidateNormalized == "" {
		return 0
	}

	score := 0
	if queryNormalized == candidateNormalized {
		score += 100
	}
	if strings.HasPrefix(candidateNormalized, queryNormalized) || strings.HasPrefix(queryNormalized, candidateNormalized) {
		score += 20
	}
	if strings.Contains(candidateNormalized, queryNormalized) || strings.Contains(queryNormalized, candidateNormalized) {
		score += 15
	}

	queryTokens := strings.Fields(queryNormalized)
	candidateTokens := strings.Fields(candidateNormalized)
	if len(queryTokens) > 0 {
		candidateTokenSet := make(map[string]struct{}, len(candidateTokens))
		for _, token := range candidateTokens {
			candidateTokenSet[token] = struct{}{}
		}

		matches := 0
		for _, token := range queryTokens {
			if _, ok := candidateTokenSet[token]; ok {
				matches++
			}
		}
		score += (matches * 60) / len(queryTokens)
	}

	if expectedYear > 0 && candidateYear > 0 {
		if expectedYear == candidateYear {
			score += 15
		} else {
			score -= 10
		}
	}

	return score
}

func normalizeTMDBMatchValue(value string) string {
	normalized := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r):
			return unicode.ToLower(r)
		case unicode.IsDigit(r):
			return r
		default:
			return ' '
		}
	}, value)
	return strings.Join(strings.Fields(normalized), " ")
}

func yearFromDate(value string) int {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) == 0 || len(parts[0]) != 4 {
		return 0
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	return year
}

func movieDataFromMovieDetails(details *MovieDetails, mediaType string) *MovieData {
	if details == nil {
		return nil
	}

	return &MovieData{
		Title:         strings.TrimSpace(details.Title),
		Overview:      strings.TrimSpace(details.Overview),
		Poster:        tmdbImageURL(details.PosterPath, tmdbPosterSize),
		Backdrop:      tmdbImageURL(details.BackdropPath, tmdbBackdropSize),
		Runtime:       details.Runtime,
		Genres:        tmdbGenreNames(details.Genres),
		ReleaseDate:   strings.TrimSpace(details.ReleaseDate),
		Cast:          tmdbCastMembers(details.Credits.Cast, movieMetadataCastMaxSize),
		Director:      strings.TrimSpace(details.Director),
		TMDBRating:    details.VoteAverage,
		TrailerKey:    tmdbTrailerKey(details.Videos.Results),
		TMDBID:        details.ID,
		TMDBMediaType: strings.TrimSpace(mediaType),
	}
}

func movieDataFromTVDetails(details *TVDetails, mediaType string) *MovieData {
	if details == nil {
		return nil
	}

	return &MovieData{
		Title:         strings.TrimSpace(details.Name),
		Overview:      strings.TrimSpace(details.Overview),
		Poster:        tmdbImageURL(details.PosterPath, tmdbPosterSize),
		Backdrop:      tmdbImageURL(details.BackdropPath, tmdbBackdropSize),
		Runtime:       details.Runtime,
		Genres:        tmdbGenreNames(details.Genres),
		ReleaseDate:   strings.TrimSpace(details.FirstAirDate),
		Cast:          tmdbCastMembers(details.Credits.Cast, movieMetadataCastMaxSize),
		Seasons:       tmdbSeasons(details.Seasons),
		Director:      strings.TrimSpace(details.Director),
		TMDBRating:    details.VoteAverage,
		TrailerKey:    tmdbTrailerKey(details.Videos.Results),
		TMDBID:        details.ID,
		TMDBMediaType: strings.TrimSpace(mediaType),
	}
}

func enrichMovieDataWithOMDB(ctx context.Context, omdbClient *OMDBClient, imdbID string, movieData *MovieData) {
	if ctx == nil || omdbClient == nil || movieData == nil {
		return
	}

	imdbID = strings.ToLower(strings.TrimSpace(imdbID))
	if !imdbIDPattern.MatchString(imdbID) {
		return
	}

	ratings, err := omdbClient.GetRatingsByIMDBID(ctx, imdbID)
	if err != nil {
		observability.LogDebug(ctx, "omdb movie enrichment skipped", "imdb_id", imdbID, "error", err.Error())
		return
	}
	if ratings == nil {
		return
	}

	movieData.RottenTomatoesScore = ratings.RottenTomatoesScore
	movieData.MetacriticScore = ratings.MetacriticScore
}

func setMovieExternalLinks(movieData *MovieData, mediaType, imdbID, rottenTomatoesSlug string) {
	if movieData == nil {
		return
	}

	normalizedIMDBID := strings.ToLower(strings.TrimSpace(imdbID))
	if imdbIDPattern.MatchString(normalizedIMDBID) {
		movieData.IMDBID = normalizedIMDBID
	}

	normalizedSlug := normalizeRottenTomatoesSlug(rottenTomatoesSlug)
	if normalizedSlug == "" {
		normalizedSlug = normalizeRottenTomatoesSlug(movieData.Title)
	}
	if normalizedSlug == "" {
		return
	}

	movieData.RottenTomatoesURL = buildRottenTomatoesURL(mediaType, normalizedSlug)
}

func normalizeRottenTomatoesSlug(raw string) string {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if slug == "" {
		return ""
	}

	slug = strings.Trim(slug, "/")
	if strings.HasPrefix(slug, "m/") || strings.HasPrefix(slug, "tv/") {
		parts := strings.SplitN(slug, "/", 2)
		if len(parts) == 2 {
			slug = parts[1]
		}
	}

	slug = strings.ReplaceAll(slug, "-", "_")
	slug = rtSlugStripPattern.ReplaceAllString(slug, "_")
	slug = strings.Trim(slug, "_")
	slug = strings.Join(strings.FieldsFunc(slug, func(r rune) bool { return r == '_' }), "_")
	return slug
}

func buildRottenTomatoesURL(mediaType, slug string) string {
	if slug == "" {
		return ""
	}

	normalizedMediaType := strings.ToLower(strings.TrimSpace(mediaType))
	if normalizedMediaType == "tv" || normalizedMediaType == "series" {
		return rottenTomatoesBaseURL + "/tv/" + slug
	}
	return rottenTomatoesBaseURL + "/m/" + slug
}

func tmdbImageURL(pathValue, size string) string {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		return ""
	}
	if strings.HasPrefix(pathValue, "https://") || strings.HasPrefix(pathValue, "http://") {
		return pathValue
	}

	if !strings.HasPrefix(pathValue, "/") {
		pathValue = "/" + pathValue
	}

	return fmt.Sprintf("%s/%s%s", tmdbImageBaseURL, size, pathValue)
}

func tmdbGenreNames(genres []TMDBGenre) []string {
	if len(genres) == 0 {
		return nil
	}

	names := make([]string, 0, len(genres))
	for _, genre := range genres {
		name := strings.TrimSpace(genre.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

func tmdbCastMembers(cast []TMDBCastMember, limit int) []models.CastMember {
	if len(cast) == 0 || limit <= 0 {
		return nil
	}

	orderedCast := make([]TMDBCastMember, 0, len(cast))
	for _, member := range cast {
		if strings.TrimSpace(member.Name) == "" {
			continue
		}
		orderedCast = append(orderedCast, member)
	}
	sort.SliceStable(orderedCast, func(i, j int) bool {
		return orderedCast[i].Order < orderedCast[j].Order
	})

	if len(orderedCast) > limit {
		orderedCast = orderedCast[:limit]
	}

	result := make([]models.CastMember, 0, len(orderedCast))
	for _, member := range orderedCast {
		result = append(result, models.CastMember{
			Name:      strings.TrimSpace(member.Name),
			Character: strings.TrimSpace(member.Character),
		})
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func tmdbSeasons(seasons []TMDBSeason) []models.Season {
	if len(seasons) == 0 {
		return nil
	}

	orderedSeasons := make([]TMDBSeason, len(seasons))
	copy(orderedSeasons, seasons)
	sort.SliceStable(orderedSeasons, func(i, j int) bool {
		return orderedSeasons[i].SeasonNumber < orderedSeasons[j].SeasonNumber
	})

	result := make([]models.Season, 0, len(orderedSeasons))
	for _, season := range orderedSeasons {
		result = append(result, models.Season{
			SeasonNumber: season.SeasonNumber,
			EpisodeCount: season.EpisodeCount,
			AirDate:      strings.TrimSpace(season.AirDate),
			Name:         strings.TrimSpace(season.Name),
			Overview:     strings.TrimSpace(season.Overview),
			Poster:       tmdbImageURL(season.PosterPath, tmdbPosterSize),
		})
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func tmdbTrailerKey(videos []TMDBVideo) string {
	if len(videos) == 0 {
		return ""
	}

	type trailerMatcher struct {
		official bool
		kind     string
	}

	matchers := []trailerMatcher{
		{official: true, kind: "trailer"},
		{official: false, kind: "trailer"},
		{official: true, kind: "teaser"},
		{official: false, kind: ""},
	}

	for _, matcher := range matchers {
		for _, video := range videos {
			if !strings.EqualFold(strings.TrimSpace(video.Site), "youtube") {
				continue
			}

			videoType := strings.ToLower(strings.TrimSpace(video.Type))
			if matcher.kind != "" && videoType != matcher.kind {
				continue
			}
			if matcher.official && !video.Official {
				continue
			}

			key := strings.TrimSpace(video.Key)
			if key != "" {
				return key
			}
		}
	}

	return ""
}
