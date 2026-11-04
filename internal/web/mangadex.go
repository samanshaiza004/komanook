package web

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"komanook/internal/database/store"
	"komanook/internal/mangadex"
)

var mangaDexIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (a *App) mangaDexSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 100 {
		http.Error(w, "Search is too long.", http.StatusBadRequest)
		return
	}
	offset, ok := parseOffset(r.URL.Query().Get("offset"), 1000, 10)
	if !ok {
		http.Error(w, "Invalid search page.", http.StatusBadRequest)
		return
	}
	result := mangadex.SearchResult{Offset: offset, Limit: 10, Items: []mangadex.Manga{}}
	if query != "" {
		var err error
		result, err = a.MangaDex.Search(r.Context(), query, offset)
		if err != nil {
			a.mangaDexFailure(w, err)
			return
		}
	}
	a.render(w, r, mangaDexSearchPage(query, result, a.Config.MangaDexRemovalContact))
}

func (a *App) mangaDexManga(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(r.PathValue("id"))
	offset, ok := parseOffset(r.URL.Query().Get("offset"), 10000, 50)
	if !ok {
		http.Error(w, "Invalid chapter page.", http.StatusBadRequest)
		return
	}
	manga, err := a.MangaDex.Manga(r.Context(), id)
	if err != nil {
		a.mangaDexFailure(w, err)
		return
	}
	feed, err := a.MangaDex.Feed(r.Context(), id, offset)
	if err != nil {
		a.mangaDexFailure(w, err)
		return
	}
	blocked, err := a.blockedMangaDexGroups(r)
	if err != nil {
		a.failure(w, r, err)
		return
	}
	chapters := make([]mangadex.Chapter, 0, len(feed.Items))
	for _, chapter := range feed.Items {
		if mangaDexChapterAvailable(chapter, blocked) {
			chapters = append(chapters, chapter)
		}
	}
	a.render(w, r, mangaDexMangaPage(manga, chapters, feed, a.Config.MangaDexRemovalContact))
}

func (a *App) mangaDexRead(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(r.PathValue("id"))
	chapter, err := a.MangaDex.Chapter(r.Context(), id)
	if err != nil {
		a.mangaDexFailure(w, err)
		return
	}
	blocked, err := a.blockedMangaDexGroups(r)
	if err != nil {
		a.failure(w, r, err)
		return
	}
	if chapter.MangaID == "" || !mangaDexChapterAvailable(chapter, blocked) {
		http.NotFound(w, r)
		return
	}
	if chapter.MangaTitle == "" {
		manga, mangaErr := a.MangaDex.Manga(r.Context(), chapter.MangaID)
		if mangaErr != nil {
			a.mangaDexFailure(w, mangaErr)
			return
		}
		chapter.MangaTitle = manga.Title
	}
	pages, err := a.MangaDex.Pages(r.Context(), id)
	if err != nil {
		a.mangaDexFailure(w, err)
		return
	}
	w.Header().Set("Content-Security-Policy", mangaDexContentSecurityPolicy())
	a.render(w, r, mangaDexReaderView(chapter, pages, a.Config.MangaDexRemovalContact))
}

func (a *App) addMangaDexGroupRemoval(w http.ResponseWriter, r *http.Request) {
	u, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if u.Role != "admin" {
		http.Error(w, "Administrator access required.", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid removal record.", http.StatusBadRequest)
		return
	}
	groupID := strings.ToLower(strings.TrimSpace(r.PostForm.Get("group_id")))
	groupName := strings.TrimSpace(r.PostForm.Get("group_name"))
	reference := strings.TrimSpace(r.PostForm.Get("request_reference"))
	verified := r.PostForm.Get("leader_verified") == "on"
	if !mangaDexIDPattern.MatchString(groupID) || utf8.RuneCountInString(groupName) < 1 || utf8.RuneCountInString(groupName) > 120 || len(reference) < 1 || len(reference) > 500 || !verified {
		http.Error(w, "Verify the active group leader's written request and provide the group details and reference.", http.StatusBadRequest)
		return
	}
	if err := a.Q.BlockMangaDexGroup(r.Context(), store.BlockMangaDexGroupParams{
		GroupID:          groupID,
		GroupName:        groupName,
		RequestReference: reference,
		CreatedBy:        u.ID,
	}); err != nil {
		a.failure(w, r, err)
		return
	}
	http.Redirect(w, r, "/account#scanlation-removals", http.StatusSeeOther)
}

func (a *App) blockedMangaDexGroups(r *http.Request) (map[string]struct{}, error) {
	ids, err := a.Q.MangaDexBlockedGroupIDs(r.Context())
	if err != nil {
		return nil, err
	}
	blocked := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		blocked[strings.ToLower(id)] = struct{}{}
	}
	return blocked, nil
}

func mangaDexChapterAvailable(chapter mangadex.Chapter, blocked map[string]struct{}) bool {
	if len(chapter.Groups) == 0 {
		return false
	}
	for _, group := range chapter.Groups {
		if _, removed := blocked[strings.ToLower(group.ID)]; removed {
			return false
		}
	}
	return true
}

func parseOffset(value string, maximum, pageSize int) (int, bool) {
	if value == "" {
		return 0, true
	}
	offset, err := strconv.Atoi(value)
	if err != nil || offset < 0 || offset > maximum || offset%pageSize != 0 {
		return 0, false
	}
	return offset, true
}

func (a *App) mangaDexFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mangadex.ErrNotFound):
		http.Error(w, "Not found.", http.StatusNotFound)
	case errors.Is(err, mangadex.ErrRateLimited):
		w.Header().Set("Retry-After", "60")
		http.Error(w, "MangaDex is rate limiting requests. Try again shortly.", http.StatusTooManyRequests)
	case errors.Is(err, mangadex.ErrInvalidImage):
		http.Error(w, "MangaDex did not provide readable page images.", http.StatusBadGateway)
	default:
		http.Error(w, "MangaDex is temporarily unavailable. Try again shortly.", http.StatusBadGateway)
	}
}

func mangaDexContentSecurityPolicy() string {
	return "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' https://uploads.mangadex.org:* https://*.mangadex.network:*; font-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'; object-src 'none'"
}

func mangaDexSearchURL(query string, offset int) string {
	values := url.Values{}
	values.Set("q", query)
	if offset > 0 {
		values.Set("offset", strconv.Itoa(offset))
	}
	return "/mangadex?" + values.Encode()
}

func mangaDexMangaURL(id string, offset int) string {
	if offset == 0 {
		return "/mangadex/manga/" + id
	}
	return "/mangadex/manga/" + id + "?offset=" + strconv.Itoa(offset)
}
