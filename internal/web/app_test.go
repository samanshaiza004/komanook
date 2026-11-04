package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"komanook/internal/auth"
	"komanook/internal/config"
	"komanook/internal/database/store"
	"komanook/internal/mangadex"
	"komanook/internal/media"
	"komanook/internal/testutil"
)

type client struct {
	t    *testing.T
	http *http.Client
	base string
}
type result struct {
	code   int
	header http.Header
	body   string
}

func stringID(id int64) string { return strconv.FormatInt(id, 10) }
func hashKey(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}
func (c client) request(method, path, body, contentType string, headers map[string]string) result {
	c.t.Helper()
	r, err := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	if method != "GET" && method != "HEAD" {
		r.Header.Set("Origin", c.base)
	}
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	resp, err := c.http.Do(r)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return result{resp.StatusCode, resp.Header, string(b)}
}
func (c client) get(path string) result { return c.request("GET", path, "", "", nil) }

var csrfPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

func (c client) token() string {
	c.t.Helper()
	r := c.get("/login")
	m := csrfPattern.FindStringSubmatch(r.body)
	if len(m) != 2 {
		c.t.Fatalf("missing token: status=%d", r.code)
	}
	return m[1]
}
func (c client) signIn(email, password string) result {
	c.t.Helper()
	return c.request("POST", "/login", url.Values{"email": {email}, "password": {password}, "csrf_token": {c.token()}}.Encode(), "application/x-www-form-urlencoded", nil)
}
func freshClient(t *testing.T, base string) client {
	j, _ := cookiejar.New(nil)
	return client{t, &http.Client{Jar: j, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, base}
}
func checkStatus(t *testing.T, r result, want int) {
	t.Helper()
	if r.code != want {
		t.Fatalf("status=%d want=%d body=%s", r.code, want, r.body)
	}
}

type mangaDexRoundTripper func(*http.Request) (*http.Response, error)

func (f mangaDexRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func mangaDexFixtureResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestHTTPFoundation(t *testing.T) {
	p := testutil.Database(t)
	ctx := context.Background()
	q := store.New(p)
	hash, err := auth.Hash([]byte("a quiet place to read"))
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []struct{ email, role string }{{"reader@example.test", "reader"}, {"other@example.test", "reader"}, {"operator@example.test", "admin"}} {
		if _, err = q.ProvisionUser(ctx, store.ProvisionUserParams{Email: account.email, PasswordHash: hash, Role: account.role}); err != nil {
			t.Fatal(err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m, err := media.New(p, t.TempDir(), logger)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewUnstartedServer(nil)
	a := New(config.Config{Env: "test", BaseURL: "http://" + s.Listener.Addr().String(), MangaDexRemovalContact: "mailto:operator@example.test"}, p, m, logger)
	const mangaID = "11111111-1111-4111-8111-111111111111"
	const externalChapterID = "22222222-2222-4222-8222-222222222222"
	const groupID = "33333333-3333-4333-8333-333333333333"
	a.MangaDex = mangadex.NewClient(&http.Client{Transport: mangaDexRoundTripper(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/manga":
			return mangaDexFixtureResponse(http.StatusOK, `{"limit":10,"offset":0,"total":1,"data":[{"id":"`+mangaID+`","type":"manga","attributes":{"title":{"en":"Community Story"},"description":{"en":"A test series."},"status":"completed","contentRating":"safe"}}]}`), nil
		case "/manga/" + mangaID:
			return mangaDexFixtureResponse(http.StatusOK, `{"data":{"id":"`+mangaID+`","type":"manga","attributes":{"title":{"en":"Community Story"},"description":{"en":"A test series."},"status":"completed","contentRating":"safe"}}}`), nil
		case "/manga/" + mangaID + "/feed":
			return mangaDexFixtureResponse(http.StatusOK, `{"limit":50,"offset":0,"total":1,"data":[{"id":"`+externalChapterID+`","type":"chapter","attributes":{"chapter":"1","title":"First Chapter","translatedLanguage":"en","pages":1},"relationships":[{"id":"`+mangaID+`","type":"manga"},{"id":"`+groupID+`","type":"scanlation_group"}]}],"included":[{"id":"`+groupID+`","type":"scanlation_group","attributes":{"name":"Trusted Group"}}]}`), nil
		case "/chapter/" + externalChapterID:
			return mangaDexFixtureResponse(http.StatusOK, `{"data":{"id":"`+externalChapterID+`","type":"chapter","attributes":{"chapter":"1","title":"First Chapter","translatedLanguage":"en","pages":1},"relationships":[{"id":"`+mangaID+`","type":"manga"},{"id":"`+groupID+`","type":"scanlation_group"}]},"included":[{"id":"`+mangaID+`","type":"manga","attributes":{"title":{"en":"Community Story"}}},{"id":"`+groupID+`","type":"scanlation_group","attributes":{"name":"Trusted Group"}}]}`), nil
		case "/at-home/server/" + externalChapterID:
			return mangaDexFixtureResponse(http.StatusOK, `{"baseUrl":"https://uploads.mangadex.org","chapter":{"hash":"0123456789abcdef0123456789abcdef","dataSaver":["1-page.jpg"]}}`), nil
		default:
			return nil, errors.New("unexpected MangaDex fixture request")
		}
	})})
	s.Config.Handler = a.Handler()
	s.Start()
	defer s.Close()
	c := freshClient(t, s.URL)
	var chapter int64
	if err = p.QueryRow(ctx, "SELECT id FROM chapters ORDER BY id LIMIT 1").Scan(&chapter); err != nil {
		t.Fatal(err)
	}
	chapterPath := "/api/progress/" + stringID(chapter)

	t.Run("public cache has no cookies or private content", func(t *testing.T) {
		r := c.get("/catalog")
		checkStatus(t, r, 200)
		if r.header.Get("Set-Cookie") != "" || !strings.Contains(r.header.Get("Cache-Control"), "public") || strings.Contains(r.body, "csrf_token") {
			t.Fatal("public caching invariant")
		}
		checkStatus(t, c.get("/series/missing"), 404)
		patch := c.request("GET", "/catalog?genre=Adventure", "", "", map[string]string{"Datastar-Request": "true"})
		if strings.Contains(patch.body, "<!doctype") || !strings.Contains(patch.body, `id="catalog-results"`) || strings.Contains(patch.body, "The Last Signal") {
			t.Fatal("incorrect bounded patch")
		}
		if !strings.Contains(strings.Join(patch.header.Values("Vary"), ","), "Datastar-Request") {
			t.Fatal("patch cache variant missing")
		}
	})
	t.Run("MangaDex credits chapters and honors recorded group removals", func(t *testing.T) {
		search := c.get("/mangadex?q=Community+Story")
		checkStatus(t, search, 200)
		if search.header.Get("Set-Cookie") != "" || !strings.Contains(search.header.Get("Cache-Control"), "no-store") || !strings.Contains(search.body, "Community Story") || !strings.Contains(search.body, `href="mailto:operator@example.test"`) {
			t.Fatal("MangaDex search is missing attribution, removal route, or private caching headers")
		}
		series := c.get("/mangadex/manga/" + mangaID)
		checkStatus(t, series, 200)
		if !strings.Contains(series.body, "Trusted Group") || !strings.Contains(series.body, "Read →") || series.header.Get("Set-Cookie") != "" {
			t.Fatal("MangaDex chapter list must display group credits without a session")
		}
		reader := c.get("/mangadex/read/" + externalChapterID)
		checkStatus(t, reader, 200)
		if !strings.Contains(reader.body, "https://uploads.mangadex.org/data-saver/") || !strings.Contains(reader.body, "Trusted Group") || !strings.Contains(reader.header.Get("Content-Security-Policy"), "uploads.mangadex.org") || reader.header.Get("Set-Cookie") != "" {
			t.Fatal("MangaDex reader must use remote pages, show group credit, and avoid session cookies")
		}

		operator := freshClient(t, s.URL)
		checkStatus(t, operator.signIn("operator@example.test", "a quiet place to read"), 303)
		form := url.Values{
			"csrf_token":        {operator.token()},
			"group_id":          {groupID},
			"group_name":        {"Trusted Group"},
			"request_reference": {"verified request 42"},
			"leader_verified":   {"on"},
		}
		checkStatus(t, operator.request("POST", "/admin/mangadex/group-removals", form.Encode(), "application/x-www-form-urlencoded", nil), 303)
		series = c.get("/mangadex/manga/" + mangaID)
		checkStatus(t, series, 200)
		if strings.Contains(series.body, "Trusted Group") || !strings.Contains(series.body, "No credited chapters") {
			t.Fatal("recorded group removal did not hide the group's chapter")
		}
		checkStatus(t, c.get("/mangadex/read/"+externalChapterID), 404)
	})
	t.Run("escaping and database failure", func(t *testing.T) {
		payload := `<img src=x onerror=alert(1)>`
		if _, err = p.Exec(ctx, "UPDATE series SET title=$1 WHERE slug='last-signal'", payload); err != nil {
			t.Fatal(err)
		}
		r := c.get("/series/last-signal")
		if strings.Contains(r.body, payload) || !strings.Contains(r.body, "&lt;img") {
			t.Fatal("title was not escaped")
		}
		_, _ = p.Exec(ctx, "UPDATE series SET title='The Last Signal' WHERE slug='last-signal'")
		checkStatus(t, c.get("/catalog?q="+url.QueryEscape(strings.Repeat("x", 101))), 400)
	})
	t.Run("authentication and CSRF boundaries", func(t *testing.T) {
		checkStatus(t, c.get(chapterPath), 401)
		checkStatus(t, c.request("POST", "/login", "email=reader@example.test&password=irrelevant", "application/x-www-form-urlencoded", nil), 403)
		bad := c.signIn("reader@example.test", "wrong password")
		checkStatus(t, bad, 422)
		if !strings.Contains(bad.header.Get("Content-Type"), "text/html") {
			t.Error("failed login must remain HTML")
		}
		checkStatus(t, c.signIn("reader@example.test", "a quiet place to read"), 303)
		checkStatus(t, c.get("/account"), 200)
		checkStatus(t, c.request("POST", "/logout", "", "", nil), 403)
		checkStatus(t, c.request("POST", chapterPath, `{"page":2}`, "application/json", nil), 403)
		checkStatus(t, c.request("POST", "/api/progress/import", `{"items":[{"chapter":1,"page":2}]}`, "application/json", nil), 403)
		headers := map[string]string{"X-CSRF-Token": c.token(), "Origin": "https://attacker.example"}
		checkStatus(t, c.request("POST", chapterPath, `{"page":2}`, "application/json", headers), 403)
		checkStatus(t, c.get("/admin/uploads/anything"), 403)
		checkStatus(t, c.request("POST", "/admin/uploads", "", "", map[string]string{"X-CSRF-Token": c.token()}), 403)
		for _, path := range []string{"/login", "/account", "/read/" + stringID(chapter), chapterPath} {
			if !strings.Contains(c.get(path).header.Get("Cache-Control"), "no-store") {
				t.Fatal("private page cache", path)
			}
		}
		public := c.get("/catalog")
		if strings.Contains(public.body, "reader@example.test") || public.header.Get("Set-Cookie") != "" {
			t.Fatal("session leaked into public page")
		}
	})
	t.Run("reader profiles are opt-in, Work-based, and session-scoped", func(t *testing.T) {
		checkStatus(t, c.get("/u/paperghost"), 404)
		settings := c.get("/account/profile")
		checkStatus(t, settings, 200)
		if !strings.Contains(settings.header.Get("Cache-Control"), "no-store") || !strings.Contains(settings.body, "Who can see your profile?") {
			t.Fatal("profile editor must be private and offer an explicit visibility choice")
		}
		checkStatus(t, c.request("POST", "/account/profile", "", "application/x-www-form-urlencoded", nil), 403)

		works, err := q.PublishedWorks(ctx)
		if err != nil || len(works) < 2 {
			t.Fatalf("seeded Work choices missing: %v", err)
		}
		owner, err := q.UserByEmail(ctx, "reader@example.test")
		if err != nil {
			t.Fatal(err)
		}
		if err = q.SaveProgress(ctx, store.SaveProgressParams{UserID: owner.ID, ChapterID: chapter, Page: 4}); err != nil {
			t.Fatal(err)
		}
		initial := url.Values{
			"csrf_token": {c.token()}, "handle": {"paperghost"}, "display_name": {"Paper Ghost"},
			"visibility": {"private"}, "theme": {"shelf"}, "showcase_order": {"favorites-first"},
		}
		checkStatus(t, c.request("POST", "/account/profile", initial.Encode(), "application/x-www-form-urlencoded", nil), 303)
		checkStatus(t, c.get("/u/paperghost"), 404)
		privateProfile, err := q.ProfileByUserID(ctx, owner.ID)
		if err != nil || privateProfile.Visibility != "private" || privateProfile.ShowReadingStats {
			t.Fatal("new profiles should be private and keep reading milestones off")
		}
		form := url.Values{
			"csrf_token":         {c.token()},
			"handle":             {"paperghost"},
			"display_name":       {"Paper Ghost"},
			"tagline":            {"Drawn to stories that stay with me."},
			"bio":                {`<script>alert(1)</script>`},
			"visibility":         {"public"},
			"theme":              {"coral"},
			"show_reading_stats": {"on"},
			"showcase_order":     {"stats-first"},
			"favorite_1":         {stringID(works[0].WorkID)},
			"favorite_2":         {stringID(works[1].WorkID)},
			"favorite_3":         {""},
			"favorite_4":         {""},
			"favorite_5":         {""},
			"favorite_6":         {""},
		}
		checkStatus(t, c.request("POST", "/account/profile", form.Encode(), "application/x-www-form-urlencoded", nil), 303)

		public := c.get("/u/paperghost")
		checkStatus(t, public, 200)
		if public.header.Get("Set-Cookie") != "" || !strings.Contains(public.header.Get("Cache-Control"), "public") {
			t.Fatal("public profile must be cacheable without setting identity cookies")
		}
		if strings.Contains(public.body, "reader@example.test") || strings.Contains(public.body, `<script>alert(1)</script>`) || !strings.Contains(public.body, "&lt;script&gt;") {
			t.Fatal("public profile exposed account data or unescaped biography")
		}
		if first, second, heading := strings.Index(public.body, "Saved milestones"), strings.Index(public.body, "Manga that made me"), strings.Index(public.body, works[0].WorkTitle); first < 0 || second < 0 || first > second || heading < 0 {
			t.Fatal("profile showcase order or Work favorites did not render")
		}
		if !strings.Contains(public.body, "Chapters with a saved place") || !strings.Contains(public.body, "Saved places at a chapter’s last page") {
			t.Fatal("opt-in reading milestones are missing")
		}

		other := freshClient(t, s.URL)
		checkStatus(t, other.signIn("other@example.test", "a quiet place to read"), 303)
		conflict := url.Values{
			"csrf_token": {other.token()}, "handle": {"paperghost"}, "display_name": {"Other Reader"},
			"visibility": {"public"}, "theme": {"shelf"}, "showcase_order": {"favorites-first"},
			"user_id": {stringID(owner.ID)},
		}
		duplicate := other.request("POST", "/account/profile", conflict.Encode(), "application/x-www-form-urlencoded", nil)
		checkStatus(t, duplicate, 422)
		if !strings.Contains(duplicate.body, "already in use") {
			t.Fatal("duplicate public handle should provide a recoverable form error")
		}
		conflict.Set("handle", "otherreader")
		checkStatus(t, other.request("POST", "/account/profile", conflict.Encode(), "application/x-www-form-urlencoded", nil), 303)
		otherProfile, err := q.ProfileByUserID(ctx, owner.ID)
		if err != nil || otherProfile.Handle != "paperghost" {
			t.Fatal("a profile write changed another account's identity")
		}

		form.Set("csrf_token", c.token())
		form.Set("visibility", "private")
		checkStatus(t, c.request("POST", "/account/profile", form.Encode(), "application/x-www-form-urlencoded", nil), 303)
		checkStatus(t, c.get("/u/paperghost"), 404)
	})
	t.Run("scoped progress and validation", func(t *testing.T) {
		headers := map[string]string{"X-CSRF-Token": c.token()}
		for _, body := range []string{`{"page":0}`, `{"page":999}`, `{"page":2,"user_id":2}`, `{"page":2} {"page":3}`, `{"page":"2"}`} {
			checkStatus(t, c.request("POST", chapterPath, body, "application/json", headers), 400)
		}
		checkStatus(t, c.request("POST", chapterPath, `{"page":3}`, "application/json", headers), 204)
		if r := c.get(chapterPath); !strings.Contains(r.body, `"page":3`) {
			t.Fatal(r.body)
		}
		other := freshClient(t, s.URL)
		checkStatus(t, other.signIn("other@example.test", "a quiet place to read"), 303)
		if r := other.get(chapterPath); !strings.Contains(r.body, `"page":0`) {
			t.Fatal("progress leaked across accounts", r.body)
		}
		_, _ = p.Exec(ctx, "UPDATE chapters SET published=false WHERE id=$1", chapter)
		checkStatus(t, c.get("/read/"+stringID(chapter)), 404)
		checkStatus(t, c.request("POST", chapterPath, `{"page":2}`, "application/json", headers), 404)
		_, _ = p.Exec(ctx, "UPDATE chapters SET published=true WHERE id=$1", chapter)
	})
	t.Run("guest progress import is atomic and does not replace account bookmarks", func(t *testing.T) {
		var secondChapter, thirdChapter int64
		if err = p.QueryRow(ctx, "SELECT id FROM chapters ORDER BY id OFFSET 1 LIMIT 1").Scan(&secondChapter); err != nil {
			t.Fatal(err)
		}
		if err = p.QueryRow(ctx, "SELECT id FROM chapters ORDER BY id OFFSET 2 LIMIT 1").Scan(&thirdChapter); err != nil {
			t.Fatal(err)
		}
		headers := map[string]string{"X-CSRF-Token": c.token()}
		body := `{"items":[{"chapter":` + stringID(chapter) + `,"page":2},{"chapter":` + stringID(secondChapter) + `,"page":2}]}`
		result := c.request("POST", "/api/progress/import", body, "application/json", headers)
		checkStatus(t, result, 200)
		if !strings.Contains(result.body, `"imported":1`) || !strings.Contains(result.body, `"existing":1`) {
			t.Fatal("unexpected import result", result.body)
		}
		if r := c.get(chapterPath); !strings.Contains(r.body, `"page":3`) {
			t.Fatal("guest progress replaced the account bookmark", r.body)
		}
		if r := c.get("/api/progress/" + stringID(secondChapter)); !strings.Contains(r.body, `"page":2`) {
			t.Fatal("guest bookmark was not imported", r.body)
		}

		invalid := `{"items":[{"chapter":` + stringID(thirdChapter) + `,"page":2},{"chapter":` + stringID(chapter) + `,"page":999}]}`
		checkStatus(t, c.request("POST", "/api/progress/import", invalid, "application/json", headers), 400)
		if r := c.get("/api/progress/" + stringID(thirdChapter)); !strings.Contains(r.body, `"page":0`) {
			t.Fatal("invalid import partially committed", r.body)
		}
		for _, malformed := range []string{
			`{"items":[]}`,
			`{"items":[{"chapter":` + stringID(chapter) + `,"page":2,"user_id":1}]}`,
			`{"items":[{"chapter":` + stringID(chapter) + `,"page":2},{"chapter":` + stringID(chapter) + `,"page":3}]}`,
		} {
			checkStatus(t, c.request("POST", "/api/progress/import", malformed, "application/json", headers), 400)
		}
	})
	t.Run("session rotates and logout revokes copied cookie", func(t *testing.T) {
		u, _ := url.Parse(s.URL)
		var first *http.Cookie
		for _, cookie := range c.http.Jar.Cookies(u) {
			if cookie.Name == a.Sessions.Cookie.Name {
				copy := *cookie
				first = &copy
			}
		}
		if first == nil {
			t.Fatal("missing session")
		}
		checkStatus(t, c.signIn("reader@example.test", "a quiet place to read"), 303)
		replay := freshClient(t, s.URL)
		replay.http.Jar.SetCookies(u, []*http.Cookie{first})
		checkStatus(t, replay.get(chapterPath), 401)
		var second *http.Cookie
		for _, cookie := range c.http.Jar.Cookies(u) {
			if cookie.Name == a.Sessions.Cookie.Name {
				copy := *cookie
				second = &copy
			}
		}
		if second == nil || second.Value == first.Value {
			t.Fatal("token not rotated")
		}
		checkStatus(t, c.request("POST", "/logout", "csrf_token="+url.QueryEscape(c.token()), "application/x-www-form-urlencoded", nil), 303)
		replay.http.Jar.SetCookies(u, []*http.Cookie{second})
		checkStatus(t, replay.get(chapterPath), 401)
	})
	t.Run("revocation expiration and disabled accounts", func(t *testing.T) {
		checkStatus(t, c.signIn("reader@example.test", "a quiet place to read"), 303)
		_, _ = p.Exec(ctx, "UPDATE users SET auth_version=auth_version+1 WHERE email='reader@example.test'")
		checkStatus(t, c.get(chapterPath), 401)
		checkStatus(t, c.signIn("reader@example.test", "a quiet place to read"), 303)
		_, _ = p.Exec(ctx, "UPDATE sessions SET expiry=now()-interval '1 second'")
		checkStatus(t, c.get(chapterPath), 401)
		_, _ = p.Exec(ctx, "UPDATE users SET disabled=true WHERE email='reader@example.test'")
		checkStatus(t, c.signIn("reader@example.test", "a quiet place to read"), 422)
		_, _ = p.Exec(ctx, "UPDATE users SET disabled=false WHERE email='reader@example.test'")
	})
	t.Run("rate limiting is shared by clients", func(t *testing.T) {
		_, _ = p.Exec(ctx, "DELETE FROM login_limits")
		for i := 0; i < 10; i++ {
			_, err = q.LoginAttempt(ctx, hashKey("account:reader@example.test"))
			if err != nil {
				t.Fatal(err)
			}
		}
		checkStatus(t, freshClient(t, s.URL).signIn("reader@example.test", "a quiet place to read"), 429)
	})
	t.Run("health and storage failures fail closed", func(t *testing.T) {
		checkStatus(t, c.get("/health/ready"), 200)
		_, _ = p.Exec(ctx, "DELETE FROM login_limits")
		checkStatus(t, c.signIn("reader@example.test", "a quiet place to read"), 303)
		p.Close()
		checkStatus(t, c.get("/health/ready"), 503)
		checkStatus(t, c.get("/catalog"), 503)
		checkStatus(t, c.get(chapterPath), 503)
	})
}

func TestProductionCookiePolicy(t *testing.T) {
	p := testutil.Database(t)
	a := New(config.Config{Env: "production", BaseURL: "https://read.example"}, p, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !a.Sessions.Cookie.Secure || !a.Sessions.Cookie.HttpOnly || a.Sessions.Cookie.SameSite != http.SameSiteLaxMode || a.Sessions.Cookie.Name != "__Host-komanook_session" || a.Sessions.Cookie.Domain != "" {
		t.Fatal("unsafe production cookie")
	}
	r := httptest.NewRequest("GET", "https://read.example/login", nil)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	for _, cookie := range w.Result().Cookies() {
		if !cookie.Secure || !cookie.HttpOnly || cookie.Domain != "" || cookie.Path != "/" {
			t.Fatal("unsafe CSRF cookie", cookie)
		}
	}
	if w.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("HSTS missing")
	}
}

func TestIdleExpiry(t *testing.T) {
	p := testutil.Database(t)
	a := New(config.Config{Env: "test", BaseURL: "http://localhost"}, p, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.Sessions.IdleTimeout = 20 * time.Millisecond
	ctx, err := a.Sessions.Load(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	a.Sessions.Put(ctx, "marker", true)
	token, _, err := a.Sessions.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	loaded, err := a.Sessions.Load(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if a.Sessions.GetBool(loaded, "marker") {
		t.Fatal("idle expiry not enforced")
	}
}

func TestEscapedReaderTranscript(t *testing.T) {
	var buf bytes.Buffer
	err := readerView(store.ChapterRow{SeriesTitle: "<script>bad</script>"}, []store.Page{{Number: 1, Transcript: `" onload="alert(1)`, Image: "sample.webp"}}, "token", 0).Render(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "<script>bad") || strings.Contains(buf.String(), `alt="" onload=`) {
		t.Fatal("unescaped reader content")
	}
}
