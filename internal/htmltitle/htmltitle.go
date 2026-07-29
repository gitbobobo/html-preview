// Package htmltitle extracts a human-readable title from an HTML document.
//
// It is intentionally non-failing: every read or parse error is swallowed and
// reported as an empty string so that title extraction can never break upload
// or replace flows. The caller owns the source of the bytes (file handle,
// buffer, network stream); this package only parses.
package htmltitle

import (
	"io"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// MaxTitleRunes is the maximum number of runes retained from an extracted
// title. Longer titles are truncated to this length and an ellipsis is
// appended.
const MaxTitleRunes = 200

// ExtractHTMLTitle reads HTML from r and returns the page title, resolved by
// priority:
//  1. the first non-empty <title> element's text content;
//  2. else <meta property="og:title" content="...">;
//  3. else <meta name="twitter:title" content="...">;
//  4. else an empty string.
//
// HTML entities are decoded by the parser (in both text nodes and attribute
// values). The result is trimmed of surrounding whitespace and, when longer
// than MaxTitleRunes, truncated to MaxTitleRunes runes with a trailing "…".
// Any read or parse error yields "".
func ExtractHTMLTitle(r io.Reader) string {
	doc, err := html.Parse(r)
	if err != nil {
		return ""
	}

	var titleText, ogTitle, twitterTitle string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				// Keep searching until the first <title> with non-empty text.
				if titleText == "" {
					if t := strings.TrimSpace(textContent(n)); t != "" {
						titleText = t
					}
				}
			case "meta":
				content := attrValue(n, "content")
				if content == "" {
					break
				}
				if attrValue(n, "property") == "og:title" && ogTitle == "" {
					ogTitle = content
				}
				if attrValue(n, "name") == "twitter:title" && twitterTitle == "" {
					twitterTitle = content
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	switch {
	case titleText != "":
		return cleanTitle(titleText)
	case ogTitle != "":
		return cleanTitle(ogTitle)
	case twitterTitle != "":
		return cleanTitle(twitterTitle)
	}
	return ""
}

// textContent returns the concatenated text of all descendant text nodes of n.
func textContent(n *html.Node) string {
	var b strings.Builder
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return b.String()
}

// attrValue returns the value of the first attribute named key on n, or "".
func attrValue(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// cleanTitle trims surrounding whitespace and truncates to MaxTitleRunes,
// appending an ellipsis when truncation occurs.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > MaxTitleRunes {
		s = string([]rune(s)[:MaxTitleRunes]) + "…"
	}
	return s
}
