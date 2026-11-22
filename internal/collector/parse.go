package collector

import (
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Input is a single form control with its current value.
type Input struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Form is a page form with its controls and their values.
type Form struct {
	Action string  `json:"action"`
	Inputs []Input `json:"inputs"`
}

// ParseDocument extracts the maximal AI record from a fetched HTML page.
// Missing elements stay zero-valued (empty title when absent, not an error).
func textOf(n *html.Node) string {
	var b strings.Builder
	var inner func(*html.Node)
	inner = func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
		for k := c.FirstChild; k != nil; k = k.NextSibling {
			inner(k)
		}
	}
	inner(n)
	return strings.TrimSpace(b.String())
}

// selectedOption returns the selected option value (first option fallback).
func selectedOption(sel *html.Node) string {
	fallback := ""
	for c := sel.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "option" {
			continue
		}
		v := ""
		selected := false
		for _, a := range c.Attr {
			if a.Key == "value" {
				v = a.Val
			}
			if a.Key == "selected" {
				selected = true
			}
		}
		if v == "" {
			v = textOf(c)
		}
		if fallback == "" {
			fallback = v
		}
		if selected {
			return v
		}
	}
	return fallback
}

func ParseDocument(url, finalURL string, status int, htmlDoc string) Result {
	res := Result{URL: url, FinalURL: finalURL, Status: status, Fetched: time.Now()}
	doc, err := html.Parse(strings.NewReader(htmlDoc))
	if err != nil {
		res.Error = "parse: " + err.Error()
		return res
	}
	res.Meta = map[string]string{}
	var text strings.Builder
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inBody bool) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if res.Title == "" {
					res.Title = textOf(n)
				}
			case "meta":
				var key, val string
				for _, a := range n.Attr {
					switch strings.ToLower(a.Key) {
					case "name", "property":
						key = a.Val
					case "content":
						val = a.Val
					}
				}
				if key != "" {
					res.Meta[key] = val
				}
			case "h1", "h2", "h3":
				if t := textOf(n); t != "" {
					res.Headings = append(res.Headings, t)
				}
			case "a":
				for _, a := range n.Attr {
					if a.Key == "href" && a.Val != "" {
						res.Links = append(res.Links, a.Val)
					}
				}
			case "script":
				for _, a := range n.Attr {
					if a.Key == "type" && a.Val == "application/ld+json" {
						if res.JSONLD != "" {
							res.JSONLD += "\n"
						}
						res.JSONLD += textOf(n)
					}
				}
			case "form":
				f := Form{}
				for _, a := range n.Attr {
					if a.Key == "action" {
						f.Action = a.Val
					}
				}
				var gather func(*html.Node)
				gather = func(c *html.Node) {
					if c.Type == html.ElementNode && (c.Data == "input" || c.Data == "select" || c.Data == "textarea" || c.Data == "button") {
						in := Input{}
						for _, a := range c.Attr {
							switch a.Key {
							case "name":
								in.Name = a.Val
							case "type":
								in.Type = a.Val
							case "value":
								in.Value = a.Val
							}
						}
						if in.Type == "" && c.Data == "input" {
							in.Type = "text"
						}
						switch c.Data {
						case "textarea":
							in.Type = "textarea"
							if v := textOf(c); v != "" {
								in.Value = v
							}
						case "select":
							in.Type = "select"
							in.Value = selectedOption(c)
						case "button", "option":
							return // button text in page Text; options via select
						}
						if in.Name == "" && in.Value == "" {
							return
						}
						f.Inputs = append(f.Inputs, in)
					}
					for k := c.FirstChild; k != nil; k = k.NextSibling {
						gather(k)
					}
				}
				gather(n)
				res.Forms = append(res.Forms, f)
			case "body":
				inBody = true
			}
		}
		if n.Type == html.TextNode && inBody {
			if t := strings.TrimSpace(n.Data); t != "" {
				// Skip inline code and styles; they are not page copy.
				for p := n.Parent; p != nil; p = p.Parent {
					if p.Type == html.ElementNode &&
						(p.Data == "script" || p.Data == "style" || p.Data == "noscript") {
						t = ""
						break
					}
				}
				if t != "" {
					text.WriteString(t + " ")
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inBody)
		}
	}
	walk(doc, false)
	res.Text = strings.TrimSpace(text.String())
	return res
}
