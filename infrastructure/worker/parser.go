package worker

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

type PageData struct {
	Title   string
	MetaDesc string
	Links   []string
	Body    string
}

// ParsePage extracts title, meta description, and outbound links from HTML.
func ParsePage(r io.Reader, baseURL string, extractBody bool) PageData {
	body, err := io.ReadAll(r)
	if err != nil {
		return PageData{}
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return PageData{}
	}

	var data PageData
	if extractBody {
		data.Body = string(body)
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if n.FirstChild != nil {
					data.Title = strings.TrimSpace(n.FirstChild.Data)
				}
			case "meta":
				var name, content string
				for _, a := range n.Attr {
					switch strings.ToLower(a.Key) {
					case "name":
						name = strings.ToLower(a.Val)
					case "content":
						content = a.Val
					}
				}
				if name == "description" {
					data.MetaDesc = content
				}
			case "a":
				for _, a := range n.Attr {
					if strings.ToLower(a.Key) == "href" {
						if link := resolveURL(baseURL, a.Val); link != "" {
							data.Links = append(data.Links, link)
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return data
}

func resolveURL(base, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		// inherit scheme from base
		if strings.HasPrefix(base, "https") {
			return "https:" + href
		}
		return "http:" + href
	}
	// relative — strip fragment, join with base
	base = strings.Split(base, "#")[0]
	if strings.HasPrefix(href, "/") {
		// absolute path — attach to base origin
		if idx := strings.Index(base[8:], "/"); idx != -1 {
			return base[:8+idx] + href
		}
		return base + href
	}
	// relative path
	if idx := strings.LastIndex(base, "/"); idx != -1 {
		return base[:idx+1] + href
	}
	return href
}
