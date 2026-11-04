// Package mangadex provides a small, read-only MangaDex API client for Komanook.
package mangadex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	apiBase          = "https://api.mangadex.org"
	apiResponseLimit = 4 << 20
	feedPageSize     = 50
	maximumPageCount = 2000
)

var (
	ErrNotFound     = errors.New("mangadex resource not found")
	ErrRateLimited  = errors.New("mangadex rate limit reached")
	ErrUnavailable  = errors.New("mangadex unavailable")
	ErrInvalidImage = errors.New("invalid mangadex image response")
	resourceID      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	chapterHash     = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
)

// Client makes bounded, read-only requests to the public MangaDex API. It does
// not download or retain chapter images; it only keeps short-lived @Home
// delivery metadata so readers do not repeatedly request the same token.
type Client struct {
	http *http.Client

	apiGate    requestGate
	atHomeGate requestGate

	cacheMu sync.Mutex
	home    map[string]homeCacheEntry
}

type requestGate struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func (g *requestGate) wait(ctx context.Context) error {
	g.mu.Lock()
	now := time.Now()
	if g.next.Before(now) {
		g.next = now
	}
	wait := g.next.Sub(now)
	g.next = g.next.Add(g.interval)
	g.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type homeCacheEntry struct {
	data      homeResponse
	expiresAt time.Time
}

// NewClient creates a client that stays below MangaDex's public API request
// limits for a single application process. A custom HTTP client can be passed
// for deterministic tests; nil uses a bounded production client.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 8 * time.Second}
	}
	return &Client{
		http:       httpClient,
		apiGate:    requestGate{interval: 250 * time.Millisecond},
		atHomeGate: requestGate{interval: 2 * time.Second},
		home:       make(map[string]homeCacheEntry),
	}
}

type Manga struct {
	ID            string
	Title         string
	Description   string
	Status        string
	Year          int
	ContentRating string
}

type SearchResult struct {
	Items  []Manga
	Total  int
	Offset int
	Limit  int
}

type Group struct {
	ID   string
	Name string
}

type Chapter struct {
	ID                 string
	MangaID            string
	MangaTitle         string
	Number             string
	Volume             string
	Title              string
	TranslatedLanguage string
	PublishedAt        time.Time
	Pages              int
	Groups             []Group
}

type FeedResult struct {
	Items  []Chapter
	Total  int
	Offset int
	Limit  int
}

type Page struct {
	URL string
}

type envelope struct {
	Data     json.RawMessage `json:"data"`
	Included []entity        `json:"included"`
	Total    int             `json:"total"`
	Limit    int             `json:"limit"`
	Offset   int             `json:"offset"`
}

type entity struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Attributes    json.RawMessage `json:"attributes"`
	Relationships []relationship  `json:"relationships"`
}

type relationship struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

type mangaAttributes struct {
	Title         map[string]string   `json:"title"`
	AltTitles     []map[string]string `json:"altTitles"`
	Description   map[string]string   `json:"description"`
	Status        string              `json:"status"`
	Year          int                 `json:"year"`
	ContentRating string              `json:"contentRating"`
}

type chapterAttributes struct {
	Chapter            string    `json:"chapter"`
	Volume             string    `json:"volume"`
	Title              string    `json:"title"`
	TranslatedLanguage string    `json:"translatedLanguage"`
	PublishAt          time.Time `json:"publishAt"`
	Pages              int       `json:"pages"`
}

type groupAttributes struct {
	Name string `json:"name"`
}

type homeResponse struct {
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
}

func (c *Client) Search(ctx context.Context, title string, offset int) (SearchResult, error) {
	if strings.TrimSpace(title) == "" || len(title) > 100 || offset < 0 || offset > 1000 || offset%10 != 0 {
		return SearchResult{}, ErrUnavailable
	}
	values := url.Values{}
	values.Set("title", title)
	values.Set("limit", "10")
	values.Set("offset", fmt.Sprint(offset))
	var response envelope
	if err := c.get(ctx, "/manga", values, &response, false); err != nil {
		return SearchResult{}, err
	}
	var entities []entity
	if err := json.Unmarshal(response.Data, &entities); err != nil {
		return SearchResult{}, ErrUnavailable
	}
	result := SearchResult{Total: response.Total, Offset: response.Offset, Limit: response.Limit, Items: make([]Manga, 0, len(entities))}
	for _, item := range entities {
		if !resourceID.MatchString(item.ID) {
			continue
		}
		var attributes mangaAttributes
		if json.Unmarshal(item.Attributes, &attributes) != nil {
			continue
		}
		result.Items = append(result.Items, toManga(item.ID, attributes))
	}
	return result, nil
}

func (c *Client) Manga(ctx context.Context, id string) (Manga, error) {
	if !resourceID.MatchString(id) {
		return Manga{}, ErrNotFound
	}
	var response struct {
		Data entity `json:"data"`
	}
	if err := c.get(ctx, "/manga/"+id, nil, &response, false); err != nil {
		return Manga{}, err
	}
	if response.Data.ID != id {
		return Manga{}, ErrUnavailable
	}
	var attributes mangaAttributes
	if err := json.Unmarshal(response.Data.Attributes, &attributes); err != nil {
		return Manga{}, ErrUnavailable
	}
	return toManga(response.Data.ID, attributes), nil
}

func toManga(id string, attributes mangaAttributes) Manga {
	return Manga{
		ID:            id,
		Title:         preferredText(attributes.Title, attributes.AltTitles),
		Description:   preferredText(attributes.Description, nil),
		Status:        attributes.Status,
		Year:          attributes.Year,
		ContentRating: attributes.ContentRating,
	}
}

func preferredText(primary map[string]string, alternatives []map[string]string) string {
	for _, preferred := range []string{"en", "ja-ro", "ja"} {
		if value := strings.TrimSpace(primary[preferred]); value != "" {
			return value
		}
	}
	if len(alternatives) > 0 {
		for _, preferred := range []string{"en", "ja-ro", "ja"} {
			for _, alternative := range alternatives {
				if value := strings.TrimSpace(alternative[preferred]); value != "" {
					return value
				}
			}
		}
	}
	keys := make([]string, 0, len(primary))
	for key := range primary {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value := strings.TrimSpace(primary[key]); value != "" {
			return value
		}
	}
	return "Untitled MangaDex series"
}

func (c *Client) Feed(ctx context.Context, mangaID string, offset int) (FeedResult, error) {
	if !resourceID.MatchString(mangaID) || offset < 0 || offset > 10000 || offset%feedPageSize != 0 {
		return FeedResult{}, ErrNotFound
	}
	values := url.Values{}
	values.Add("translatedLanguage[]", "en")
	values.Add("includes[]", "scanlation_group")
	values.Set("order[chapter]", "asc")
	values.Set("limit", fmt.Sprint(feedPageSize))
	values.Set("offset", fmt.Sprint(offset))
	var response envelope
	if err := c.get(ctx, "/manga/"+mangaID+"/feed", values, &response, false); err != nil {
		return FeedResult{}, err
	}
	var entities []entity
	if err := json.Unmarshal(response.Data, &entities); err != nil {
		return FeedResult{}, ErrUnavailable
	}
	groupNames := includedGroupNames(response.Included)
	result := FeedResult{Total: response.Total, Offset: response.Offset, Limit: response.Limit, Items: make([]Chapter, 0, len(entities))}
	for _, item := range entities {
		chapter, err := toChapter(item, groupNames)
		if err == nil {
			if chapter.MangaID == "" {
				chapter.MangaID = mangaID
			}
			result.Items = append(result.Items, chapter)
		}
	}
	return result, nil
}

func (c *Client) Chapter(ctx context.Context, id string) (Chapter, error) {
	if !resourceID.MatchString(id) {
		return Chapter{}, ErrNotFound
	}
	values := url.Values{}
	values.Add("includes[]", "manga")
	values.Add("includes[]", "scanlation_group")
	var response struct {
		Data     entity   `json:"data"`
		Included []entity `json:"included"`
	}
	if err := c.get(ctx, "/chapter/"+id, values, &response, false); err != nil {
		return Chapter{}, err
	}
	chapter, err := toChapter(response.Data, includedGroupNames(response.Included))
	if err != nil {
		return Chapter{}, err
	}
	for _, included := range response.Included {
		if included.Type != "manga" || included.ID != chapter.MangaID {
			continue
		}
		var attributes mangaAttributes
		if json.Unmarshal(included.Attributes, &attributes) == nil {
			chapter.MangaTitle = preferredText(attributes.Title, attributes.AltTitles)
		}
	}
	return chapter, nil
}

func toChapter(item entity, groupNames map[string]string) (Chapter, error) {
	if !resourceID.MatchString(item.ID) {
		return Chapter{}, ErrNotFound
	}
	var attributes chapterAttributes
	if err := json.Unmarshal(item.Attributes, &attributes); err != nil {
		return Chapter{}, ErrUnavailable
	}
	chapter := Chapter{
		ID:                 item.ID,
		Number:             attributes.Chapter,
		Volume:             attributes.Volume,
		Title:              attributes.Title,
		TranslatedLanguage: attributes.TranslatedLanguage,
		PublishedAt:        attributes.PublishAt,
		Pages:              attributes.Pages,
	}
	for _, relation := range item.Relationships {
		switch relation.Type {
		case "manga":
			if resourceID.MatchString(relation.ID) {
				chapter.MangaID = relation.ID
			}
		case "scanlation_group":
			if !resourceID.MatchString(relation.ID) {
				continue
			}
			name := groupNames[relation.ID]
			if len(relation.Attributes) > 0 {
				var attr groupAttributes
				if json.Unmarshal(relation.Attributes, &attr) == nil && strings.TrimSpace(attr.Name) != "" {
					name = attr.Name
				}
			}
			if strings.TrimSpace(name) != "" {
				chapter.Groups = append(chapter.Groups, Group{ID: relation.ID, Name: strings.TrimSpace(name)})
			}
		}
	}
	sort.Slice(chapter.Groups, func(i, j int) bool { return chapter.Groups[i].Name < chapter.Groups[j].Name })
	return chapter, nil
}

func includedGroupNames(included []entity) map[string]string {
	groups := make(map[string]string)
	for _, item := range included {
		if item.Type != "scanlation_group" {
			continue
		}
		var attributes groupAttributes
		if json.Unmarshal(item.Attributes, &attributes) == nil && strings.TrimSpace(attributes.Name) != "" {
			groups[item.ID] = strings.TrimSpace(attributes.Name)
		}
	}
	return groups
}

func (c *Client) Pages(ctx context.Context, chapterID string) ([]Page, error) {
	if !resourceID.MatchString(chapterID) {
		return nil, ErrNotFound
	}
	response, err := c.atHome(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if len(response.Chapter.DataSaver) == 0 || len(response.Chapter.DataSaver) > maximumPageCount {
		return nil, ErrInvalidImage
	}
	base, err := url.Parse(response.BaseURL)
	if err != nil || !allowedImageHost(base) || !chapterHash.MatchString(response.Chapter.Hash) {
		return nil, ErrInvalidImage
	}
	pages := make([]Page, 0, len(response.Chapter.DataSaver))
	for _, filename := range response.Chapter.DataSaver {
		if filename == "" || strings.ContainsAny(filename, "/\\") || filename == "." || filename == ".." {
			return nil, ErrInvalidImage
		}
		pageURL := *base
		pageURL.Path = path.Join(base.Path, "data-saver", response.Chapter.Hash, filename)
		pageURL.RawPath = ""
		pages = append(pages, Page{URL: pageURL.String()})
	}
	return pages, nil
}

func allowedImageHost(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "uploads.mangadex.org" || strings.HasSuffix(host, ".mangadex.network")
}

func (c *Client) atHome(ctx context.Context, chapterID string) (homeResponse, error) {
	now := time.Now()
	c.cacheMu.Lock()
	if cached, ok := c.home[chapterID]; ok && now.Before(cached.expiresAt) {
		c.cacheMu.Unlock()
		return cached.data, nil
	}
	c.cacheMu.Unlock()
	if err := c.atHomeGate.wait(ctx); err != nil {
		return homeResponse{}, err
	}
	// A concurrent request may have populated the cache while this request waited.
	now = time.Now()
	c.cacheMu.Lock()
	if cached, ok := c.home[chapterID]; ok && now.Before(cached.expiresAt) {
		c.cacheMu.Unlock()
		return cached.data, nil
	}
	c.cacheMu.Unlock()
	var response homeResponse
	if err := c.get(ctx, "/at-home/server/"+chapterID, nil, &response, true); err != nil {
		return homeResponse{}, err
	}
	if _, err := url.ParseRequestURI(response.BaseURL); err != nil {
		return homeResponse{}, ErrInvalidImage
	}
	c.cacheMu.Lock()
	c.home[chapterID] = homeCacheEntry{data: response, expiresAt: time.Now().Add(10 * time.Minute)}
	if len(c.home) > 500 {
		for id, entry := range c.home {
			if time.Now().After(entry.expiresAt) {
				delete(c.home, id)
			}
		}
	}
	c.cacheMu.Unlock()
	return response, nil
}

func (c *Client) get(ctx context.Context, route string, values url.Values, dest any, atHome bool) error {
	if err := c.apiGate.wait(ctx); err != nil {
		return err
	}
	endpoint, err := url.Parse(apiBase + route)
	if err != nil {
		return ErrUnavailable
	}
	endpoint.RawQuery = values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Komanook/1.0")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("%w: request failed", ErrUnavailable)
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusOK:
	default:
		return ErrUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, apiResponseLimit+1))
	if err != nil || len(body) > apiResponseLimit {
		return ErrUnavailable
	}
	if err = json.Unmarshal(body, dest); err != nil {
		return ErrUnavailable
	}
	return nil
}
