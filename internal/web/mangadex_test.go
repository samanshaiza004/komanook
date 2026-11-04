package web

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"komanook/internal/mangadex"
)

func TestMangaDexChapterAvailabilityRequiresCreditsAndHonorsRemoval(t *testing.T) {
	chapter := mangadex.Chapter{Groups: []mangadex.Group{
		{ID: "11111111-1111-4111-8111-111111111111", Name: "Active group"},
	}}
	if !mangaDexChapterAvailable(chapter, map[string]struct{}{}) {
		t.Fatal("credited, unblocked chapter should be available")
	}
	if mangaDexChapterAvailable(mangadex.Chapter{}, map[string]struct{}{}) {
		t.Fatal("chapter without a scanlation group must not be readable")
	}
	if mangaDexChapterAvailable(chapter, map[string]struct{}{"11111111-1111-4111-8111-111111111111": {}}) {
		t.Fatal("chapter from a removed group should be hidden")
	}
	chapter.Groups = append(chapter.Groups, mangadex.Group{ID: "22222222-2222-4222-8222-222222222222", Name: "Second group"})
	if mangaDexChapterAvailable(chapter, map[string]struct{}{"11111111-1111-4111-8111-111111111111": {}}) {
		t.Fatal("a chapter linked to any removed group should be hidden")
	}
}

func TestMangaDexOffsetsStayOnBoundedPageBoundaries(t *testing.T) {
	for _, value := range []struct {
		query string
		max   int
		size  int
		want  int
		valid bool
	}{
		{"", 1000, 10, 0, true},
		{"20", 1000, 10, 20, true},
		{"21", 1000, 10, 0, false},
		{"-10", 1000, 10, 0, false},
		{"1010", 1000, 10, 0, false},
	} {
		got, valid := parseOffset(value.query, value.max, value.size)
		if got != value.want || valid != value.valid {
			t.Errorf("parseOffset(%q)=(%d,%v), want (%d,%v)", value.query, got, valid, value.want, value.valid)
		}
	}
}

func TestMangaDexReaderShowsSourceAndGroupCredits(t *testing.T) {
	component := mangaDexReaderView(
		mangadex.Chapter{
			ID:         "22222222-2222-4222-8222-222222222222",
			MangaID:    "11111111-1111-4111-8111-111111111111",
			MangaTitle: "A Community Story",
			Number:     "3",
			Title:      "A New Day",
			Groups:     []mangadex.Group{{ID: "33333333-3333-4333-8333-333333333333", Name: "Trusted Group"}},
		},
		[]mangadex.Page{{URL: "https://uploads.mangadex.org/data-saver/0123456789abcdef0123456789abcdef/1-page.jpg"}},
		"mailto:operator@example.test",
	)
	var output bytes.Buffer
	if err := component.Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}
	page := output.String()
	for _, expected := range []string{
		`data-signed-in="false"`,
		`https://uploads.mangadex.org/data-saver/0123456789abcdef0123456789abcdef/1-page.jpg`,
		`https://mangadex.org/group/33333333-3333-4333-8333-333333333333`,
		"Trusted Group",
		"No ads",
		"not stored by Komanook",
		`href="mailto:operator@example.test"`,
	} {
		if !strings.Contains(page, expected) {
			t.Errorf("reader output is missing %q", expected)
		}
	}
}
