package dongao

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

var (
	directMediaRe = regexp.MustCompile(`(?i)https?:\\?/\\?/[^"'<>\s]+\.(?:m3u8|mp4|flv|mov|m4v|mp3|m4a)(?:\?[^"'<>\s]*)?`)
	kvMediaRe     = regexp.MustCompile(`(?is)(?:source|url|path|playUrl|playbackUrl|video_url)\s*[:=]\s*["']([^"']+)["']`)
	anchorRe      = regexp.MustCompile(`(?is)<a\b[^>]*href\s*=\s*["']([^"']+)["'][^>]*>([\s\S]*?)</a>`)
	directFileRe  = regexp.MustCompile(`(?i)(?:https?:\\?/\\?/|/)[^"'<>\s]+\.(?:pdf|pptx?|docx?|xlsx?|xls|zip|rar|7z|caj)(?:\?[^"'<>\s]*)?`)
)

type resourceRef struct {
	Title  string `json:"title"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

func findMediaInText(text string) string {
	for _, m := range directMediaRe.FindAllString(text, -1) {
		if s := normalizeURL(m); isMediaURL(s) {
			return s
		}
	}
	for _, m := range kvMediaRe.FindAllStringSubmatch(text, -1) {
		if s := normalizeURL(m[1]); isMediaURL(s) {
			return s
		}
	}
	if payload := parseJSONText(text); payload != nil {
		if s := findMediaURL(payload); s != "" {
			return s
		}
	}
	return ""
}

func collectLectureNodes(v any, fallbackTitle string) []lectureNode {
	seen := map[string]bool{}
	var out []lectureNode
	var walk func(any, string)
	walk = func(x any, title string) {
		switch vv := x.(type) {
		case map[string]any:
			nextTitle := firstNonEmpty(valueString(vv, "lectureName", "lectureTitle", "title", "name", "videoName", "courseName"), title, fallbackTitle)
			id := valueString(vv, "lectureId", "lectureID", "listenLectureId", "liveNumberId", "liveLectureId", "id")
			if id != "" && !seen[id] && (hasAny(vv, "lectureId", "lectureID", "listenLectureId", "liveNumberId", "liveLectureId") || strings.Contains(strings.ToLower(nextTitle), "讲")) {
				seen[id] = true
				out = append(out, lectureNode{ID: id, Title: nextTitle})
			}
			for _, child := range vv {
				walk(child, nextTitle)
			}
		case []any:
			for _, child := range vv {
				walk(child, title)
			}
		}
	}
	walk(v, fallbackTitle)
	return out
}

func findMediaURL(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range []string{"source", "mainSource", "videoSource", "url", "path", "playUrl", "playbackUrl", "video_url", "m3u8"} {
			if s := normalizeURL(valueString(x, k)); isMediaURL(s) {
				return s
			}
		}
		for _, child := range x {
			if s := findMediaURL(child); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range x {
			if s := findMediaURL(child); s != "" {
				return s
			}
		}
	case string:
		if s := normalizeURL(x); isMediaURL(s) {
			return s
		}
	}
	return ""
}

func pickTitle(v any) string {
	switch x := v.(type) {
	case map[string]any:
		if s := valueString(x, "courseName", "lectureName", "lectureTitle", "name", "title", "videoName"); s != "" {
			return s
		}
		for _, child := range x {
			if s := pickTitle(child); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range x {
			if s := pickTitle(child); s != "" {
				return s
			}
		}
	}
	return ""
}

func extractJSONObjects(text string) []string {
	var out []string
	for _, marker := range []string{"courseCatalog", "liveAndCourseMap", "lectureList", "listenParam"} {
		idx := strings.Index(text, marker)
		if idx < 0 {
			continue
		}
		start := strings.LastIndex(text[:idx], "{")
		if start < 0 {
			continue
		}
		if obj := balancedJSON(text[start:]); obj != "" {
			out = append(out, obj)
		}
	}
	return out
}

func balancedJSON(s string) string {
	depth := 0
	inStr := byte(0)
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inStr != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == inStr {
				inStr = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inStr = ch
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return ""
}

func mediaInfo(title, mediaURL string, headers map[string]string) *extractor.MediaInfo {
	format := "mp4"
	if strings.Contains(strings.ToLower(mediaURL), ".m3u8") {
		format = "m3u8"
	}
	return &extractor.MediaInfo{Site: "dongao", Title: util.SanitizeFilename(title), Streams: map[string]extractor.Stream{"best": {Quality: "best", URLs: []string{mediaURL}, Format: format, Headers: headers}}}
}

func collectResourceRefsFromText(text, baseURL string) []resourceRef {
	refs := make([]resourceRef, 0)
	seen := map[string]bool{}
	add := func(title, rawURL string) {
		resourceURL := normalizeURLWithBase(rawURL, baseURL)
		if resourceURL == "" || !isFileURL(resourceURL) || seen[resourceURL] {
			return
		}
		seen[resourceURL] = true
		refs = append(refs, resourceRef{
			Title:  firstNonEmpty(cleanText(stripTags(title)), fileTitleFromURL(resourceURL), "课程资料"),
			URL:    resourceURL,
			Format: fileExtension(resourceURL),
		})
	}
	for _, m := range anchorRe.FindAllStringSubmatch(text, -1) {
		href := html.UnescapeString(m[1])
		if strings.Contains(strings.ToLower(href), "/lecture/") {
			continue
		}
		add(m[2], href)
	}
	for _, raw := range directFileRe.FindAllString(text, -1) {
		add("", raw)
	}
	if payload := parseJSONText(text); payload != nil {
		for _, ref := range collectResourceRefsFromAny(payload, baseURL) {
			if !seen[ref.URL] {
				seen[ref.URL] = true
				refs = append(refs, ref)
			}
		}
	}
	return refs
}

func collectResourceRefsFromAny(v any, baseURL string) []resourceRef {
	var refs []resourceRef
	var walk func(any, string)
	walk = func(x any, title string) {
		switch vv := x.(type) {
		case map[string]any:
			nextTitle := firstNonEmpty(valueString(vv, "fileName", "fileTitle", "resourceName", "title", "name", "lectureName", "courseName"), title)
			for _, key := range []string{"handoutUrl", "handoutURL", "handout", "pdfUrl", "pptUrl", "docUrl", "paperUrl", "fileUrl", "fileURL", "downloadUrl", "resourceUrl", "attachmentUrl", "coursewareUrl", "url", "path"} {
				if raw := valueString(vv, key); raw != "" {
					resourceURL := normalizeURLWithBase(raw, baseURL)
					if isFileURL(resourceURL) {
						refs = append(refs, resourceRef{
							Title:  firstNonEmpty(nextTitle, fileTitleFromURL(resourceURL), "课程资料"),
							URL:    resourceURL,
							Format: fileExtension(resourceURL),
						})
					}
				}
			}
			for _, child := range vv {
				walk(child, nextTitle)
			}
		case []any:
			for _, child := range vv {
				walk(child, title)
			}
		case string:
			resourceURL := normalizeURLWithBase(vv, baseURL)
			if isFileURL(resourceURL) {
				refs = append(refs, resourceRef{
					Title:  firstNonEmpty(title, fileTitleFromURL(resourceURL), "课程资料"),
					URL:    resourceURL,
					Format: fileExtension(resourceURL),
				})
			}
		}
	}
	walk(v, "")
	return dedupeResourceRefs(refs)
}

func resourceEntriesFromRefs(refs []resourceRef, headers map[string]string) []*extractor.MediaInfo {
	refs = dedupeResourceRefs(refs)
	entries := make([]*extractor.MediaInfo, 0, len(refs))
	for _, ref := range refs {
		entries = append(entries, resourceMediaInfo(ref, headers))
	}
	return entries
}

func resourceMediaInfo(ref resourceRef, headers map[string]string) *extractor.MediaInfo {
	format := firstNonEmpty(ref.Format, fileExtension(ref.URL), "bin")
	title := util.SanitizeFilename(firstNonEmpty(ref.Title, fileTitleFromURL(ref.URL), "课程资料"))
	return &extractor.MediaInfo{
		Site:  "dongao",
		Title: strings.TrimSuffix(title, "."+format),
		Streams: map[string]extractor.Stream{"best": {
			Quality: "best",
			URLs:    []string{ref.URL},
			Format:  format,
			Headers: cloneHeaders(headers),
		}},
		Extra: map[string]any{"type": "file", "source_url": ref.URL},
	}
}

func addResourceExtra(entry *extractor.MediaInfo, refs []resourceRef) {
	if entry == nil || len(refs) == 0 {
		return
	}
	if entry.Extra == nil {
		entry.Extra = map[string]any{}
	}
	entry.Extra["resources"] = dedupeResourceRefs(refs)
}

func resourceRefsFromExtra(entry *extractor.MediaInfo) []resourceRef {
	if entry == nil || entry.Extra == nil {
		return nil
	}
	switch refs := entry.Extra["resources"].(type) {
	case []resourceRef:
		return refs
	case []any:
		out := make([]resourceRef, 0, len(refs))
		for _, item := range refs {
			if m, ok := item.(map[string]any); ok {
				out = append(out, resourceRef{Title: valueString(m, "title", "Title"), URL: valueString(m, "url", "URL"), Format: valueString(m, "format", "Format")})
			}
		}
		return out
	default:
		return nil
	}
}

func dedupeResourceRefs(refs []resourceRef) []resourceRef {
	seen := map[string]bool{}
	out := make([]resourceRef, 0, len(refs))
	for _, ref := range refs {
		ref.URL = normalizeURLWithBase(ref.URL, referer)
		if ref.URL == "" || !isFileURL(ref.URL) || seen[ref.URL] {
			continue
		}
		seen[ref.URL] = true
		ref.Title = firstNonEmpty(ref.Title, fileTitleFromURL(ref.URL), "课程资料")
		ref.Format = firstNonEmpty(ref.Format, fileExtension(ref.URL), "bin")
		out = append(out, ref)
	}
	return out
}

func dedupeMediaEntries(entries []*extractor.MediaInfo) []*extractor.MediaInfo {
	seen := map[string]bool{}
	out := make([]*extractor.MediaInfo, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		key := entry.Title
		for _, stream := range entry.Streams {
			if len(stream.URLs) > 0 {
				key += "|" + stream.URLs[0]
				break
			}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entry)
	}
	return out
}

func valueString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func hasAny(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func normalizeURL(s string) string {
	s = strings.TrimSpace(strings.Trim(s, `"'`))
	s = strings.ReplaceAll(s, `\\/`, `/`)
	s = strings.ReplaceAll(s, `\/`, `/`)
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	if strings.HasPrefix(s, "/") {
		return origin + s
	}
	return s
}

func normalizeURLWithBase(s, baseURL string) string {
	s = strings.TrimSpace(html.UnescapeString(strings.Trim(s, `"'`)))
	s = strings.ReplaceAll(s, `\\/`, `/`)
	s = strings.ReplaceAll(s, `\/`, `/`)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	if baseURL == "" {
		return normalizeURL(s)
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return normalizeURL(s)
	}
	ref, err := url.Parse(s)
	if err != nil {
		return normalizeURL(s)
	}
	return base.ResolveReference(ref).String()
}

func isMediaURL(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(low, "http") && (strings.Contains(low, ".m3u8") || strings.Contains(low, ".mp4") || strings.Contains(low, ".flv") || strings.Contains(low, ".mov") || strings.Contains(low, ".m4v") || strings.Contains(low, ".mp3") || strings.Contains(low, ".m4a"))
}

func isFileURL(s string) bool {
	switch fileExtension(s) {
	case "pdf", "ppt", "pptx", "doc", "docx", "xls", "xlsx", "zip", "rar", "7z", "caj":
		return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "http")
	default:
		return false
	}
}

func fileExtension(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	path := raw
	if err == nil {
		path = u.Path
	}
	if i := strings.LastIndex(path, "."); i >= 0 && i < len(path)-1 {
		return strings.ToLower(strings.TrimSpace(path[i+1:]))
	}
	return ""
}

func fileTitleFromURL(raw string) string {
	u, err := url.Parse(raw)
	path := raw
	if err == nil {
		path = u.Path
	}
	if i := strings.LastIndex(path, "/"); i >= 0 && i < len(path)-1 {
		name, _ := url.PathUnescape(path[i+1:])
		ext := fileExtension(name)
		return strings.TrimSuffix(name, "."+ext)
	}
	return ""
}

func stripTags(s string) string {
	return regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(s, " ")
}

func cloneHeaders(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func appendQuery(raw string, query url.Values) string {
	if len(query) == 0 {
		return raw
	}
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + query.Encode()
}
