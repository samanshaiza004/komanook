package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/justinas/nosurf"
	"komanook/internal/database/store"
)

func (a *App) readerLibrary(w http.ResponseWriter, r *http.Request) {
	u, err := a.currentUser(r)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(search) > 100 {
		http.Error(w, "Search is too long.", http.StatusBadRequest)
		return
	}
	a.renderLibrary(w, r, u.ID, search, r.URL.Query().Get("saved"), r.URL.Query().Get("removed"), "", http.StatusOK)
}

func (a *App) saveLibraryEntry(w http.ResponseWriter, r *http.Request) {
	u, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Library form is too large or invalid.", http.StatusBadRequest)
		return
	}
	workID, err := strconv.ParseInt(r.PathValue("work"), 10, 64)
	if err != nil || workID < 1 {
		http.NotFound(w, r)
		return
	}
	if _, err = a.Q.CatalogWorkByID(r.Context(), workID); errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		a.failure(w, r, err)
		return
	}
	search := strings.TrimSpace(r.PostForm.Get("q"))
	if len(search) > 100 {
		http.Error(w, "Search is too long.", http.StatusBadRequest)
		return
	}
	if r.PostForm.Get("remove") == "1" {
		if err = a.Q.DeleteLibraryEntry(r.Context(), store.DeleteLibraryEntryParams{UserID: u.ID, WorkID: workID}); err != nil {
			a.failure(w, r, err)
			return
		}
		query := url.Values{"removed": {strconv.FormatInt(workID, 10)}}
		if search != "" {
			query.Set("q", search)
		}
		http.Redirect(w, r, "/account/library?"+query.Encode(), http.StatusSeeOther)
		return
	}
	status := r.PostForm.Get("status")
	if !validLibraryStatus(status) {
		a.renderLibrary(w, r, u.ID, search, "", "", "Choose a reading status before saving.", http.StatusUnprocessableEntity)
		return
	}

	rating := pgtype.Int2{}
	if value := strings.TrimSpace(r.PostForm.Get("rating")); value != "" {
		score, parseErr := strconv.ParseInt(value, 10, 16)
		if parseErr != nil || score < 1 || score > 10 {
			a.renderLibrary(w, r, u.ID, search, "", "", "Choose a rating from 1 to 10, or leave the work unrated.", http.StatusUnprocessableEntity)
			return
		}
		rating = pgtype.Int2{Int16: int16(score), Valid: true}
	}
	if err = a.Q.SaveLibraryEntry(r.Context(), store.SaveLibraryEntryParams{
		UserID: u.ID, WorkID: workID, Status: status, Rating: rating,
	}); err != nil {
		a.failure(w, r, err)
		return
	}
	query := url.Values{"saved": {strconv.FormatInt(workID, 10)}}
	if search != "" {
		query.Set("q", search)
	}
	http.Redirect(w, r, "/account/library?"+query.Encode(), http.StatusSeeOther)
}

func validLibraryStatus(status string) bool {
	switch status {
	case "reading", "plan_to_read", "completed", "on_hold", "dropped":
		return true
	default:
		return false
	}
}

func libraryStatusLabel(status string) string {
	switch status {
	case "reading":
		return "Currently reading"
	case "plan_to_read":
		return "Plan to read"
	case "completed":
		return "Completed"
	case "on_hold":
		return "On hold"
	case "dropped":
		return "Dropped"
	default:
		return "Not in your library"
	}
}

func (a *App) renderLibrary(w http.ResponseWriter, r *http.Request, userID int64, search, saved, removed, message string, status int) {
	works, err := a.Q.LibraryWorksByUserID(r.Context(), store.LibraryWorksByUserIDParams{UserID: userID, Search: search})
	if err != nil {
		a.failure(w, r, err)
		return
	}
	counts, err := a.Q.LibraryCountsByUserID(r.Context(), userID)
	if err != nil {
		a.failure(w, r, err)
		return
	}
	page := libraryView(libraryPageData{
		Token: nosurf.Token(r), Search: search, Works: works,
		Tracked: counts.Tracked, Reading: counts.Reading, Completed: counts.Completed,
		SavedWork: saved, RemovedWork: removed, Error: message,
	})
	if status == http.StatusOK {
		a.render(w, r, page)
		return
	}
	a.renderStatus(w, r, status, page)
}
