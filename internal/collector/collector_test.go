package collector

import (
	"strings"
	"testing"
)

const maximalFixture = `<!doctype html><html><head>
<title></title>
<meta name="description" content="AI summary here">
<meta property="og:title" content="OG Title">
<script type="application/ld+json">{"@type":"Article","headline":"LD Headline"}</script>
</head><body>
<h1>Main Heading</h1><h2>Sub Heading</h2>
<p>Hello world body text.</p>
<a href="/next">Next page</a><a href="https://other.example/out">Out</a>
<form action="/search"><input type="text" name="q" value="preset query"><input type="hidden" name="tok" value="abc123"><button type="submit">Go</button></form>
<form action="/login"><input type="email" name="email" value=""><input type="password" name="pw"></form>
</body></html>`

func TestParseDocumentMaximal(t *testing.T) {
	res := ParseDocument("https://seed.example/page", "https://seed.example/page", 200, maximalFixture)
	if res.URL != "https://seed.example/page" || res.FinalURL != "https://seed.example/page" {
		t.Fatalf("urls: %+v", res)
	}
	if res.Status != 200 {
		t.Fatalf("status: %d", res.Status)
	}
	if res.Title != "" {
		t.Fatalf("title should be empty, got %q", res.Title)
	}
	if res.Meta["description"] != "AI summary here" || res.Meta["og:title"] != "OG Title" {
		t.Fatalf("meta: %+v", res.Meta)
	}
	if len(res.Headings) != 2 || res.Headings[0] != "Main Heading" {
		t.Fatalf("headings: %+v", res.Headings)
	}
	if !strings.Contains(res.Text, "Hello world body text.") {
		t.Fatalf("text: %q", res.Text)
	}
	if len(res.Links) != 2 {
		t.Fatalf("links: %+v", res.Links)
	}
	if len(res.Forms) != 2 {
		t.Fatalf("forms: %+v", res.Forms)
	}
	f0 := res.Forms[0]
	if f0.Action != "/search" || len(f0.Inputs) != 2 || f0.Inputs[0].Value != "preset query" || f0.Inputs[1].Value != "abc123" {
		t.Fatalf("form0: %+v", f0)
	}
	if !strings.Contains(res.JSONLD, "LD Headline") {
		t.Fatalf("jsonld: %q", res.JSONLD)
	}
	if res.Error != "" {
		t.Fatalf("error: %q", res.Error)
	}
}
