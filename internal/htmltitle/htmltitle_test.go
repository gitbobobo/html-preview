package htmltitle

import (
	"errors"
	"strings"
	"testing"
)

// errReader is an io.Reader whose Read always fails, used to exercise the
// "any read error yields ''" contract of ExtractHTMLTitle.
type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errors.New("read error") }

func TestExtractHTMLTitle_TitleElement(t *testing.T) {
	r := strings.NewReader(`<!DOCTYPE html><html><head><title>  Hello &amp; Welcome  </title></head><body></body></html>`)
	if got := ExtractHTMLTitle(r); got != "Hello & Welcome" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractHTMLTitle_FirstNonEmptyTitle(t *testing.T) {
	r := strings.NewReader(`<html><head><title>   </title><title>Real Title</title></head></html>`)
	if got := ExtractHTMLTitle(r); got != "Real Title" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractHTMLTitle_OgFallback(t *testing.T) {
	r := strings.NewReader(`<html><head><title></title><meta property="og:title" content="OG &copy; 2026"></head></html>`)
	if got := ExtractHTMLTitle(r); got != "OG © 2026" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractHTMLTitle_TwitterFallback(t *testing.T) {
	r := strings.NewReader(`<html><head><meta name="twitter:title" content="Bird Site"></head></html>`)
	if got := ExtractHTMLTitle(r); got != "Bird Site" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractHTMLTitle_PriorityTitleOverMeta(t *testing.T) {
	r := strings.NewReader(`<html><head><title>Element Wins</title><meta property="og:title" content="OG"></head></html>`)
	if got := ExtractHTMLTitle(r); got != "Element Wins" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractHTMLTitle_NoTitle(t *testing.T) {
	r := strings.NewReader(`<html><head></head><body><h1>nothing</h1></body></html>`)
	if got := ExtractHTMLTitle(r); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractHTMLTitle_EmptyReader(t *testing.T) {
	if got := ExtractHTMLTitle(strings.NewReader("")); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractHTMLTitle_ReaderError(t *testing.T) {
	if got := ExtractHTMLTitle(errReader{}); got != "" {
		t.Fatalf("expected empty for read error, got %q", got)
	}
}

func TestExtractHTMLTitle_Truncation(t *testing.T) {
	long := strings.Repeat("あ", MaxTitleRunes+10) // multibyte runes
	r := strings.NewReader("<html><head><title>" + long + "</title></head></html>")
	got := ExtractHTMLTitle(r)
	want := strings.Repeat("あ", MaxTitleRunes) + "…"
	if got != want {
		t.Fatalf("expected len %d, got len %d", len(want), len(got))
	}
}

func TestExtractHTMLTitle_MalformedHTML(t *testing.T) {
	// Parser is error-tolerant; should still extract if a title is present.
	r := strings.NewReader(`<html><head><title>Still Works</title>`)
	if got := ExtractHTMLTitle(r); got != "Still Works" {
		t.Fatalf("got %q", got)
	}
}
