package web

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/justinas/nosurf"
	"komanook/internal/database/store"
)

const profileFavoriteSlots = 6

var profileHandlePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,23}$`)

type profileForm struct {
	Handle           string
	DisplayName      string
	Tagline          string
	Bio              string
	Visibility       string
	Theme            string
	ShowReadingStats bool
	ShowcaseOrder    string
	Favorites        []string
}

type profileEditorData struct {
	Token         string
	Form          profileForm
	Works         []store.CatalogWorksRow
	Error         string
	Saved         bool
	HasProfile    bool
	PreviewNeeded bool
}

func defaultProfileForm() profileForm {
	return profileForm{
		Visibility:    "private",
		Theme:         "shelf",
		ShowcaseOrder: "favorites-first",
		Favorites:     make([]string, profileFavoriteSlots),
	}
}

func (a *App) profileSettings(w http.ResponseWriter, r *http.Request) {
	u, err := a.currentUser(r)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}

	form := defaultProfileForm()
	profile, err := a.Q.ProfileByUserID(r.Context(), u.ID)
	if err == nil {
		form = profileFormFromRecord(profile)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		a.failure(w, r, err)
		return
	}
	if err == nil {
		favorites, favoriteErr := a.Q.ProfileFavoritesByUserID(r.Context(), u.ID)
		if favoriteErr != nil {
			a.failure(w, r, favoriteErr)
			return
		}
		form.Favorites = favoriteIDs(favorites)
	}
	works, err := a.Q.CatalogWorks(r.Context())
	if err != nil {
		a.failure(w, r, err)
		return
	}
	a.render(w, r, profileEditorView(profileEditorData{
		Token:         nosurf.Token(r),
		Form:          form,
		Works:         works,
		Saved:         r.URL.Query().Get("saved") == "1",
		HasProfile:    profile.UserID != 0,
		PreviewNeeded: r.URL.Query().Get("preview") == "save",
	}))
}

func (a *App) profilePreview(w http.ResponseWriter, r *http.Request) {
	u, err := a.currentUser(r)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}
	profile, err := a.Q.ProfileByUserID(r.Context(), u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Redirect(w, r, "/account/profile?preview=save", http.StatusSeeOther)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}
	favorites, err := a.Q.ProfileFavoritesByUserID(r.Context(), u.ID)
	if err != nil {
		a.failure(w, r, err)
		return
	}
	stats := store.ProfileReadingStatsRow{}
	if profile.ShowReadingStats {
		stats, err = a.Q.ProfileReadingStats(r.Context(), u.ID)
		if err != nil {
			a.failure(w, r, err)
			return
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	a.render(w, r, publicProfileView(profile, favorites, stats, true))
}

func (a *App) saveProfile(w http.ResponseWriter, r *http.Request) {
	u, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Profile form is too large or invalid.", http.StatusBadRequest)
		return
	}

	form := profileFormFromRequest(r)
	works, err := a.Q.CatalogWorks(r.Context())
	if err != nil {
		a.failure(w, r, err)
		return
	}
	if message := validateProfileForm(form); message != "" {
		a.renderStatus(w, r, http.StatusUnprocessableEntity, profileEditorView(profileEditorData{Token: nosurf.Token(r), Form: form, Works: works, Error: message}))
		return
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		a.failure(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.Q.WithTx(tx)
	_, err = q.UpsertReaderProfile(r.Context(), store.UpsertReaderProfileParams{
		UserID:           u.ID,
		Handle:           form.Handle,
		DisplayName:      form.DisplayName,
		Tagline:          form.Tagline,
		Bio:              form.Bio,
		Visibility:       form.Visibility,
		Theme:            form.Theme,
		ShowReadingStats: form.ShowReadingStats,
		ShowcaseOrder:    form.ShowcaseOrder,
	})
	if err != nil {
		if uniqueViolation(err) {
			a.renderStatus(w, r, http.StatusUnprocessableEntity, profileEditorView(profileEditorData{Token: nosurf.Token(r), Form: form, Works: works, Error: "That profile address is already in use. Choose another handle."}))
			return
		}
		a.failure(w, r, err)
		return
	}
	if err = q.DeleteProfileFavorites(r.Context(), u.ID); err != nil {
		a.failure(w, r, err)
		return
	}
	position := int16(0)
	for _, value := range form.Favorites {
		if value == "" {
			continue
		}
		workID, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
			a.failure(w, r, errors.New("validated profile favorite became invalid"))
			return
		}
		if _, err = q.CatalogWorkByID(r.Context(), workID); errors.Is(err, pgx.ErrNoRows) {
			a.renderStatus(w, r, http.StatusUnprocessableEntity, profileEditorView(profileEditorData{Token: nosurf.Token(r), Form: form, Works: works, Error: "Choose manga from the available catalog."}))
			return
		} else if err != nil {
			a.failure(w, r, err)
			return
		}
		position++
		if err = q.AddProfileFavorite(r.Context(), store.AddProfileFavoriteParams{UserID: u.ID, WorkID: workID, Position: position}); err != nil {
			a.failure(w, r, err)
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		a.failure(w, r, err)
		return
	}
	http.Redirect(w, r, "/account/profile?saved=1", http.StatusSeeOther)
}

func (a *App) publicProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := a.Q.PublicProfileByHandle(r.Context(), r.PathValue("handle"))
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.failure(w, r, err)
		return
	}
	favorites, err := a.Q.ProfileFavoritesByUserID(r.Context(), profile.UserID)
	if err != nil {
		a.failure(w, r, err)
		return
	}
	stats := store.ProfileReadingStatsRow{}
	if profile.ShowReadingStats {
		stats, err = a.Q.ProfileReadingStats(r.Context(), profile.UserID)
		if err != nil {
			a.failure(w, r, err)
			return
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	a.render(w, r, publicProfileView(profile, favorites, stats, false))
}

func hasProfileCover(favorites []store.ProfileFavoritesByUserIDRow) bool {
	for _, favorite := range favorites {
		if favorite.Cover != "" {
			return true
		}
	}
	return false
}

func profileFormFromRecord(profile store.ReaderProfile) profileForm {
	return profileForm{
		Handle:           profile.Handle,
		DisplayName:      profile.DisplayName,
		Tagline:          profile.Tagline,
		Bio:              profile.Bio,
		Visibility:       profile.Visibility,
		Theme:            profile.Theme,
		ShowReadingStats: profile.ShowReadingStats,
		ShowcaseOrder:    profile.ShowcaseOrder,
		Favorites:        make([]string, profileFavoriteSlots),
	}
}

func profileFormFromRequest(r *http.Request) profileForm {
	form := defaultProfileForm()
	form.Handle = strings.ToLower(strings.TrimSpace(r.PostForm.Get("handle")))
	form.DisplayName = strings.TrimSpace(r.PostForm.Get("display_name"))
	form.Tagline = strings.TrimSpace(r.PostForm.Get("tagline"))
	form.Bio = strings.TrimSpace(r.PostForm.Get("bio"))
	form.Visibility = r.PostForm.Get("visibility")
	form.Theme = r.PostForm.Get("theme")
	form.ShowReadingStats = r.PostForm.Get("show_reading_stats") == "on"
	form.ShowcaseOrder = r.PostForm.Get("showcase_order")
	for i := range form.Favorites {
		form.Favorites[i] = strings.TrimSpace(r.PostForm.Get("favorite_" + strconv.Itoa(i+1)))
	}
	return form
}

func favoriteIDs(rows []store.ProfileFavoritesByUserIDRow) []string {
	ids := make([]string, profileFavoriteSlots)
	for _, row := range rows {
		if row.Position >= 1 && row.Position <= profileFavoriteSlots {
			ids[row.Position-1] = strconv.FormatInt(row.WorkID, 10)
		}
	}
	return ids
}

func validateProfileForm(form profileForm) string {
	if !profileHandlePattern.MatchString(form.Handle) {
		return "Choose a handle from 3 to 24 characters using lowercase letters, numbers, hyphens, or underscores."
	}
	if n := utf8.RuneCountInString(form.DisplayName); n < 1 || n > 60 {
		return "Your display name must be between 1 and 60 characters."
	}
	if utf8.RuneCountInString(form.Tagline) > 100 {
		return "Keep your profile note to 100 characters or fewer."
	}
	if utf8.RuneCountInString(form.Bio) > 320 {
		return "Keep your profile introduction to 320 characters or fewer."
	}
	if form.Visibility != "private" && form.Visibility != "public" {
		return "Choose whether your profile is public or private."
	}
	if form.Theme != "shelf" && form.Theme != "night" && form.Theme != "coral" {
		return "Choose one of the available profile themes."
	}
	if form.ShowcaseOrder != "favorites-first" && form.ShowcaseOrder != "stats-first" {
		return "Choose a profile showcase order."
	}
	seen := make(map[string]struct{}, profileFavoriteSlots)
	for _, value := range form.Favorites {
		if value == "" {
			continue
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id < 1 {
			return "Choose manga from the available catalog."
		}
		if _, duplicate := seen[value]; duplicate {
			return "Choose each manga once, or leave a slot open."
		}
		seen[value] = struct{}{}
	}
	return ""
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func profileDescription(profile store.ReaderProfile) string {
	if profile.Tagline != "" {
		return profile.DisplayName + "'s manga shelf: " + profile.Tagline
	}
	return "A personal manga shelf curated by " + profile.DisplayName + "."
}
