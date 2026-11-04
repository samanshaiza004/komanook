package mangadex

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testClient(t *testing.T, transport roundTripFunc) *Client {
	t.Helper()
	client := NewClient(&http.Client{Transport: transport})
	client.apiGate.interval = 0
	client.atHomeGate.interval = 0
	return client
}

func apiResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestSearchUsesBoundedAPIQueryAndSelectsEnglishTitle(t *testing.T) {
	client := testClient(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.mangadex.org" || request.URL.Path != "/manga" || request.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
		query := request.URL.Query()
		if query.Get("title") != "Kiichi!!" || query.Get("limit") != "10" || query.Get("offset") != "10" {
			t.Fatalf("unexpected query: %v", query)
		}
		if request.Header.Get("User-Agent") == "" {
			t.Fatal("missing identifying user agent")
		}
		return apiResponse(http.StatusOK, `{"result":"ok","response":"collection","limit":10,"offset":10,"total":12,"data":[{"id":"11111111-1111-4111-8111-111111111111","type":"manga","attributes":{"title":{"ja-ro":"Kiichi!!","en":"Kiichi!!"},"description":{"en":"A description"},"status":"completed","year":2002,"contentRating":"safe"}}]}`), nil
	})
	result, err := client.Search(context.Background(), "Kiichi!!", 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 12 || result.Limit != 10 || len(result.Items) != 1 || result.Items[0].Title != "Kiichi!!" || result.Items[0].Year != 2002 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestFeedIncludesGroupNamesAndOmitsUncreditedChapters(t *testing.T) {
	client := testClient(t, func(request *http.Request) (*http.Response, error) {
		query := request.URL.Query()
		if query.Get("translatedLanguage[]") != "en" || query.Get("includes[]") != "scanlation_group" || query.Get("limit") != "50" {
			t.Fatalf("missing bounded feed filters: %v", query)
		}
		return apiResponse(http.StatusOK, `{"limit":50,"offset":0,"total":2,"data":[{"id":"22222222-2222-4222-8222-222222222222","type":"chapter","attributes":{"chapter":"1","title":"First","translatedLanguage":"en","pages":4},"relationships":[{"id":"11111111-1111-4111-8111-111111111111","type":"manga"},{"id":"33333333-3333-4333-8333-333333333333","type":"scanlation_group"}]},{"id":"44444444-4444-4444-8444-444444444444","type":"chapter","attributes":{"chapter":"2","title":"Uncredited","translatedLanguage":"en","pages":4},"relationships":[{"id":"11111111-1111-4111-8111-111111111111","type":"manga"}]}],"included":[{"id":"33333333-3333-4333-8333-333333333333","type":"scanlation_group","attributes":{"name":"Sample Group"}}]}`), nil
	})
	result, err := client.Feed(context.Background(), "11111111-1111-4111-8111-111111111111", 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Items) != 2 || result.Items[0].MangaID == "" || len(result.Items[0].Groups) != 1 || result.Items[0].Groups[0].Name != "Sample Group" || len(result.Items[1].Groups) != 0 {
		t.Fatalf("unexpected feed: %#v", result)
	}
}

func TestPagesUsesOnlyDataSaverAndRejectsUntrustedImageHosts(t *testing.T) {
	client := testClient(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/at-home/server/55555555-5555-4555-8555-555555555555" {
			t.Fatalf("unexpected at-home request: %s", request.URL)
		}
		return apiResponse(http.StatusOK, `{"baseUrl":"https://home-1.mangadex.network","chapter":{"hash":"0123456789abcdef0123456789abcdef","data":["original.png"],"dataSaver":["1-page one.jpg","2-page-two.jpg"]}}`), nil
	})
	pages, err := client.Pages(context.Background(), "55555555-5555-4555-8555-555555555555")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("pages=%d", len(pages))
	}
	first, _ := url.Parse(pages[0].URL)
	if first.Host != "home-1.mangadex.network" || first.Path != "/data-saver/0123456789abcdef0123456789abcdef/1-page one.jpg" {
		t.Fatalf("unexpected first page URL: %s", pages[0].URL)
	}

	malicious := testClient(t, func(*http.Request) (*http.Response, error) {
		return apiResponse(http.StatusOK, `{"baseUrl":"https://attacker.example","chapter":{"hash":"0123456789abcdef0123456789abcdef","dataSaver":["page.jpg"]}}`), nil
	})
	if _, err = malicious.Pages(context.Background(), "55555555-5555-4555-8555-555555555555"); err != ErrInvalidImage {
		t.Fatalf("host validation error=%v", err)
	}
}

func TestRateLimitIsNotRetried(t *testing.T) {
	calls := 0
	client := testClient(t, func(*http.Request) (*http.Response, error) {
		calls++
		return apiResponse(http.StatusTooManyRequests, `{"errors":[]}`), nil
	})
	_, err := client.Search(context.Background(), "Demon Slayer", 0)
	if err != ErrRateLimited || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
