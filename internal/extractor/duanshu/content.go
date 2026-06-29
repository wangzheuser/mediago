package duanshu

import (
	"html"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

func documentInfo(siteTitle string, payload any, headers map[string]string) *extractor.MediaInfo {
	title := util.SanitizeFilename(firstNonEmpty(siteTitle, pickTitle(payload), "duanshu_article"))
	body := findArticleBody(payload)
	if strings.TrimSpace(body) == "" {
		return nil
	}
	doc := buildArticleHTML(title, body)
	return &extractor.MediaInfo{
		Site:  "duanshu",
		Title: title,
		Streams: map[string]extractor.Stream{
			"document": {Quality: "article", URLs: []string{duanshuDataURL("text/html", doc)}, Format: "html", Headers: headers},
		},
		Extra: map[string]any{"type": "article"},
	}
}

func buildArticleHTML(title, body string) string {
	if !looksLikeHTML(body) {
		body = "<p>" + html.EscapeString(body) + "</p>"
	}
	escapedTitle := html.EscapeString(firstNonEmpty(title, "短书图文"))
	return "<!doctype html><html><head><meta charset=\"utf-8\"><title>" + escapedTitle + "</title></head><body><h1>" + escapedTitle + "</h1>" + body + "</body></html>"
}

func findArticleBody(v any) string {
	if s := articleBodyFromAny(v); s != "" {
		return s
	}
	return ""
}

func articleBodyFromAny(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for _, key := range []string{"content", "html", "richText", "rich_text", "body", "detail", "text"} {
			if value, ok := x[key]; ok {
				if body := articleBodyFromAny(value); body != "" {
					return body
				}
			}
		}
		for _, key := range []string{"play_data", "audio_data", "data", "response"} {
			if value, ok := x[key]; ok {
				if body := articleBodyFromAny(value); body != "" {
					return body
				}
			}
		}
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			if body := articleBodyFromAny(item); body != "" {
				parts = append(parts, body)
			}
		}
		return strings.Join(parts, "\n")
	case string:
		s := strings.TrimSpace(strings.ReplaceAll(x, `\/`, `/`))
		if s == "" || isMediaURL(s) || isDownloadURL(s) {
			return ""
		}
		return s
	}
	return ""
}

func looksLikeHTML(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "<p") || strings.Contains(lower, "<div") || strings.Contains(lower, "<img") || strings.Contains(lower, "<br") || strings.Contains(lower, "<section")
}

func duanshuDataURL(mime, content string) string {
	return "data:" + mime + ";charset=utf-8," + url.PathEscape(content)
}

func courseListMedia(shop string, items []contentItem) *extractor.MediaInfo {
	entries := make([]*extractor.MediaInfo, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		key := item.Kind + ":" + item.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		kind := normalizeType(item.Kind)
		if kind == "single" {
			kind = "course"
		}
		link := duanshuContentURL(shop, kind, item.ID)
		title := firstNonEmpty(item.Title, item.ID)
		entries = append(entries, &extractor.MediaInfo{
			Site:  "duanshu",
			Title: util.SanitizeFilename(title),
			Extra: map[string]any{"url": link, "content_id": item.ID, "content_type": kind, "is_test": item.Test},
		})
	}
	return &extractor.MediaInfo{Site: "duanshu", Title: util.SanitizeFilename(firstNonEmpty(shop, "duanshu") + "_courses"), Entries: entries}
}

func duanshuContentURL(shop, kind, id string) string {
	shop = firstNonEmpty(shop, "h5")
	kind = normalizeType(kind)
	if kind == "single" {
		kind = "course"
	}
	return "https://" + shop + ".duanshu.com/#/brief/" + url.PathEscape(kind) + "/" + url.PathEscape(id)
}

func formatFromURL(rawURL string) string {
	pathPart := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Path != "" {
		pathPart = u.Path
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.Split(pathPart, "?")[0])), ".")
	if ext == "" {
		return "mp4"
	}
	return ext
}
