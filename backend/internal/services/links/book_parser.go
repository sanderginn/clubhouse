package links

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/sanderginn/clubhouse/internal/models"
)

var (
	goodreadsBookIDPattern   = regexp.MustCompile(`^(\d+)`)
	openLibraryWorkIDPattern = regexp.MustCompile(`(?i)^ol[0-9a-z]+w$`)
	openLibraryBookIDPattern = regexp.MustCompile(`(?i)^ol[0-9a-z]+m$`)
	isbnTokenPattern         = regexp.MustCompile(`(?i)(97[89][0-9-]{10,16}|[0-9][0-9-]{8,14}[0-9x])`)

	fetchBookPageTitleFunc = fetchBookPageTitle
)

type BookData = models.BookData

func ParseBookMetadata(ctx context.Context, rawURL string, client *OpenLibraryClient) (*BookData, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if client == nil {
		return nil, errors.New("open library client is required")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil
	}

	host := strings.ToLower(strings.TrimSpace(parsedURL.Hostname()))
	segments := splitURLPath(parsedURL.Path)

	switch {
	case isGoodreadsHost(host):
		goodreadsID, ok := parseGoodreadsBookID(segments)
		if !ok {
			return nil, nil
		}
		return parseGoodreadsBookMetadata(ctx, rawURL, client, goodreadsID)
	case isAmazonHost(host):
		asin, ok := parseAmazonASIN(segments)
		if !ok {
			return nil, nil
		}
		return parseAmazonBookMetadata(ctx, rawURL, client, asin)
	case isOpenLibraryHost(host):
		if workKey, ok := parseOpenLibraryWorkKey(segments); ok {
			return parseOpenLibraryWorkMetadata(ctx, client, workKey)
		}
		if editionKey, ok := parseOpenLibraryEditionKey(segments); ok {
			return parseOpenLibraryEditionMetadata(ctx, client, editionKey)
		}
		if isbn, ok := extractISBNFromSegments(segments); ok {
			return parseISBNBookMetadata(ctx, client, isbn)
		}
		return nil, nil
	default:
		if isbn, ok := extractISBNFromSegments(segments); ok {
			return parseISBNBookMetadata(ctx, client, isbn)
		}
		return nil, nil
	}
}

func parseGoodreadsBookMetadata(ctx context.Context, rawURL string, client *OpenLibraryClient, goodreadsID string) (*BookData, error) {
	title, err := fetchBookPageTitleFunc(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch goodreads title for id %s: %w", goodreadsID, err)
	}
	if title == "" {
		return nil, nil
	}

	metadata, err := searchOpenLibraryByTitle(ctx, client, title)
	if err != nil {
		return nil, fmt.Errorf("search open library for goodreads id %s: %w", goodreadsID, err)
	}
	if metadata == nil {
		return nil, nil
	}

	metadata.GoodreadsURL = strings.TrimSpace(rawURL)
	return metadata, nil
}

func parseAmazonBookMetadata(ctx context.Context, rawURL string, client *OpenLibraryClient, asin string) (*BookData, error) {
	asin = strings.TrimSpace(asin)
	if asin == "" {
		return nil, nil
	}

	var lookupErr error
	if metadata, err := parseISBNBookMetadata(ctx, client, asin); err != nil {
		lookupErr = fmt.Errorf("lookup amazon asin %s: %w", asin, err)
	} else if metadata != nil {
		return metadata, nil
	}

	title, err := fetchBookPageTitleFunc(ctx, rawURL)
	if err != nil {
		if lookupErr != nil {
			return nil, lookupErr
		}
		return nil, fmt.Errorf("fetch amazon title for asin %s: %w", asin, err)
	}
	if title == "" {
		return nil, lookupErr
	}

	metadata, err := searchOpenLibraryByTitle(ctx, client, title)
	if err != nil {
		if lookupErr != nil {
			return nil, lookupErr
		}
		return nil, fmt.Errorf("search open library for amazon asin %s: %w", asin, err)
	}
	if metadata != nil {
		return metadata, nil
	}

	return nil, lookupErr
}

func searchOpenLibraryByTitle(ctx context.Context, client *OpenLibraryClient, title string) (*BookData, error) {
	title = normalizeBookPageTitle(title)
	if title == "" {
		return nil, nil
	}

	results, err := client.SearchBooks(ctx, title)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	result := results[0]
	metadata := bookDataFromSearchResult(client, result)
	if metadata == nil {
		return nil, nil
	}

	workKey := strings.TrimSpace(result.Key)
	if workKey == "" {
		return metadata, nil
	}

	work, err := client.GetWork(ctx, workKey)
	if err != nil {
		if errors.Is(err, ErrOpenLibraryNotFound) {
			return metadata, nil
		}
		return nil, err
	}

	metadata = mergeBookData(metadata, bookDataFromWork(client, work))
	if metadata.OpenLibraryKey == "" {
		metadata.OpenLibraryKey = normalizeOpenLibraryKey(workKey, "works")
	}

	return metadata, nil
}

func parseOpenLibraryWorkMetadata(ctx context.Context, client *OpenLibraryClient, workKey string) (*BookData, error) {
	work, err := client.GetWork(ctx, workKey)
	if err != nil {
		if errors.Is(err, ErrOpenLibraryNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get open library work %s: %w", workKey, err)
	}

	metadata := bookDataFromWork(client, work)
	if metadata == nil {
		return nil, nil
	}
	metadata.OpenLibraryKey = normalizeOpenLibraryKey(workKey, "works")
	return metadata, nil
}

func parseOpenLibraryEditionMetadata(ctx context.Context, client *OpenLibraryClient, editionKey string) (*BookData, error) {
	edition, err := client.GetEdition(ctx, editionKey)
	if err != nil {
		if errors.Is(err, ErrOpenLibraryNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get open library edition %s: %w", editionKey, err)
	}

	metadata := bookDataFromEdition(client, edition)
	if metadata == nil {
		return nil, nil
	}
	metadata.OpenLibraryKey = normalizeOpenLibraryKey(editionKey, "books")
	return metadata, nil
}

func parseISBNBookMetadata(ctx context.Context, client *OpenLibraryClient, identifier string) (*BookData, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, nil
	}

	edition, err := client.GetByISBN(ctx, identifier)
	if err != nil {
		if errors.Is(err, ErrOpenLibraryNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get open library isbn %s: %w", identifier, err)
	}

	metadata := bookDataFromEdition(client, edition)
	if metadata == nil {
		return nil, nil
	}
	if metadata.ISBN == "" {
		metadata.ISBN = identifier
	}
	return metadata, nil
}

func bookDataFromSearchResult(client *OpenLibraryClient, result OLSearchResult) *BookData {
	metadata := &BookData{
		Title:          strings.TrimSpace(result.Title),
		Authors:        cleanStringSlice(result.AuthorName),
		CoverURL:       client.CoverURL(result.CoverID, "L"),
		OpenLibraryKey: normalizeOpenLibraryKey(result.Key, "works"),
	}
	if result.FirstPublishYear > 0 {
		metadata.PublishDate = strconv.Itoa(result.FirstPublishYear)
	}
	return metadata
}

func bookDataFromWork(client *OpenLibraryClient, work *OLWork) *BookData {
	if work == nil {
		return nil
	}

	metadata := &BookData{
		Title:       strings.TrimSpace(work.Title),
		Description: strings.TrimSpace(work.Description),
		Genres:      cleanStringSlice(work.Subjects),
	}
	if coverID := firstCoverID(work.Covers); coverID > 0 {
		metadata.CoverURL = client.CoverURL(coverID, "L")
	}
	return metadata
}

func bookDataFromEdition(client *OpenLibraryClient, edition *OLEdition) *BookData {
	if edition == nil {
		return nil
	}

	metadata := &BookData{
		Title:       strings.TrimSpace(edition.Title),
		PageCount:   edition.NumberOfPages,
		PublishDate: strings.TrimSpace(edition.PublishDate),
	}
	if coverID := firstCoverID(edition.Covers); coverID > 0 {
		metadata.CoverURL = client.CoverURL(coverID, "L")
	}
	metadata.ISBN = firstNonEmpty(firstNonEmptySlice(edition.ISBN13), firstNonEmptySlice(edition.ISBN10))

	return metadata
}

func mergeBookData(base *BookData, enrichment *BookData) *BookData {
	if base == nil {
		return enrichment
	}
	if enrichment == nil {
		return base
	}

	merged := *base
	if merged.Title == "" {
		merged.Title = enrichment.Title
	}
	if len(merged.Authors) == 0 {
		merged.Authors = enrichment.Authors
	}
	if merged.Description == "" {
		merged.Description = enrichment.Description
	}
	if merged.CoverURL == "" {
		merged.CoverURL = enrichment.CoverURL
	}
	if merged.PageCount == 0 {
		merged.PageCount = enrichment.PageCount
	}
	if len(merged.Genres) == 0 {
		merged.Genres = enrichment.Genres
	}
	if merged.PublishDate == "" {
		merged.PublishDate = enrichment.PublishDate
	}
	if merged.ISBN == "" {
		merged.ISBN = enrichment.ISBN
	}
	if merged.OpenLibraryKey == "" {
		merged.OpenLibraryKey = enrichment.OpenLibraryKey
	}
	if merged.GoodreadsURL == "" {
		merged.GoodreadsURL = enrichment.GoodreadsURL
	}

	return &merged
}

func isGoodreadsHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "goodreads.com" || strings.HasSuffix(host, ".goodreads.com")
}

func isAmazonHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "amazon.com" || strings.HasSuffix(host, ".amazon.com")
}

func isOpenLibraryHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "openlibrary.org" || strings.HasSuffix(host, ".openlibrary.org")
}

func parseGoodreadsBookID(segments []string) (string, bool) {
	if len(segments) < 3 || !strings.EqualFold(segments[0], "book") || !strings.EqualFold(segments[1], "show") {
		return "", false
	}

	match := goodreadsBookIDPattern.FindStringSubmatch(strings.TrimSpace(segments[2]))
	if len(match) != 2 {
		return "", false
	}

	return strings.TrimSpace(match[1]), true
}

func parseAmazonASIN(segments []string) (string, bool) {
	if len(segments) < 3 {
		return "", false
	}

	for i := 0; i < len(segments)-1; i++ {
		if !strings.EqualFold(segments[i], "dp") {
			continue
		}

		asin := strings.TrimSpace(segments[i+1])
		asin = strings.TrimSuffix(asin, ".html")
		asin = strings.TrimSuffix(asin, "/")
		if asin == "" {
			continue
		}

		valid := true
		for _, r := range asin {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				continue
			}
			valid = false
			break
		}
		if valid {
			return asin, true
		}
	}

	return "", false
}

func parseOpenLibraryWorkKey(segments []string) (string, bool) {
	if len(segments) < 2 || !strings.EqualFold(segments[0], "works") {
		return "", false
	}

	rawID := strings.TrimSpace(strings.TrimSuffix(segments[1], ".json"))
	if !openLibraryWorkIDPattern.MatchString(rawID) {
		return "", false
	}

	return "/works/" + rawID, true
}

func parseOpenLibraryEditionKey(segments []string) (string, bool) {
	if len(segments) < 2 || !strings.EqualFold(segments[0], "books") {
		return "", false
	}

	rawID := strings.TrimSpace(strings.TrimSuffix(segments[1], ".json"))
	if !openLibraryBookIDPattern.MatchString(rawID) {
		return "", false
	}

	return "/books/" + rawID, true
}

func extractISBNFromSegments(segments []string) (string, bool) {
	for _, segment := range segments {
		token := strings.TrimSpace(segment)
		if token == "" {
			continue
		}

		matches := isbnTokenPattern.FindAllString(token, -1)
		for _, match := range matches {
			isbn := normalizeISBN(match)
			if isValidISBN10(isbn) || isValidISBN13(isbn) {
				return isbn, true
			}
		}
	}

	return "", false
}

func normalizeISBN(raw string) string {
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == 'x' || r == 'X':
			builder.WriteByte('X')
		}
	}

	return builder.String()
}

func isValidISBN10(isbn string) bool {
	if len(isbn) != 10 {
		return false
	}

	sum := 0
	for i := 0; i < 10; i++ {
		var value int
		switch {
		case i == 9 && isbn[i] == 'X':
			value = 10
		case isbn[i] >= '0' && isbn[i] <= '9':
			value = int(isbn[i] - '0')
		default:
			return false
		}
		sum += value * (10 - i)
	}

	return sum%11 == 0
}

func isValidISBN13(isbn string) bool {
	if len(isbn) != 13 {
		return false
	}

	sum := 0
	for i := 0; i < 13; i++ {
		if isbn[i] < '0' || isbn[i] > '9' {
			return false
		}
		digit := int(isbn[i] - '0')
		if i%2 == 0 {
			sum += digit
		} else {
			sum += 3 * digit
		}
	}

	return sum%10 == 0
}

func fetchBookPageTitle(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	metaTags, title := extractHTMLMeta(body)
	title = firstNonEmpty(metaTags["og:title"], metaTags["twitter:title"], title)
	return normalizeBookPageTitle(title), nil
}

func normalizeBookPageTitle(raw string) string {
	title := strings.TrimSpace(html.UnescapeString(raw))
	if title == "" {
		return ""
	}

	suffixes := []string{
		"| Goodreads",
		"- Goodreads",
		"| Amazon",
		": Amazon.com",
		": Amazon.com: Books",
	}

	for _, suffix := range suffixes {
		if idx := strings.Index(strings.ToLower(title), strings.ToLower(suffix)); idx > 0 {
			title = strings.TrimSpace(title[:idx])
		}
	}

	// Goodreads titles often follow "<title> by <author> | Goodreads".
	if idx := strings.Index(strings.ToLower(title), " by "); idx > 0 {
		title = strings.TrimSpace(title[:idx])
	}

	return strings.TrimSpace(title)
}

func normalizeOpenLibraryKey(rawKey, resource string) string {
	identifier, err := normalizeOpenLibraryIdentifier(rawKey, resource)
	if err != nil {
		return strings.TrimSpace(strings.TrimSuffix(rawKey, ".json"))
	}

	unescaped, err := url.PathUnescape(identifier)
	if err != nil {
		unescaped = identifier
	}

	return "/" + resource + "/" + strings.TrimSpace(unescaped)
}

func firstCoverID(covers []int) int {
	for _, coverID := range covers {
		if coverID > 0 {
			return coverID
		}
	}
	return 0
}

func cleanStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}

	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func firstNonEmptySlice(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
