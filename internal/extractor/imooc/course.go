package imooc

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

type imoocCourseEntry struct {
	Kind     string
	ID       string
	CourseID string
	Title    string
	URL      string
}

func fetchImoocCourseEntries(c *util.Client, h map[string]string, host, cid string, opts *extractor.ExtractOpts) (string, []*extractor.MediaInfo, error) {
	switch {
	case strings.Contains(host, "coding.imooc.com"):
		return fetchCodingCourseEntries(c, h, host, cid)
	case strings.Contains(host, "class.imooc.com"):
		return fetchClassCourseEntries(c, h, host, cid)
	default:
		return fetchFreeCourseEntries(c, h, host, cid)
	}
}

func fetchFreeCourseEntries(c *util.Client, h map[string]string, host, cid string) (string, []*extractor.MediaInfo, error) {
	body, err := c.GetString(fmt.Sprintf("%s/learn/%s", host, url.QueryEscape(cid)), h)
	if err != nil {
		return "", nil, err
	}
	title := cleanHTMLTitle(firstNonEmpty(match1(body, `name\s*:\s*'([^']+)'`), match1(body, `<title>(.*?)</title>`)))
	items := parseFreeItems(body)
	entries := make([]*extractor.MediaInfo, 0, len(items))
	for _, item := range items {
		switch item.Kind {
		case "video":
			mi, err := extractImoocVideo(c, h, host, cid, item.ID, item.Title)
			if err == nil && mi != nil {
				entries = append(entries, mi)
			}
		case "code":
			if content := fetchFreeHTMLResource(c, h, host, "code", item.ID); content != "" {
				entries = append(entries, htmlEntry(item.Title, content, map[string]any{"kind": "code", "code_id": item.ID}))
			}
		case "exercise":
			if content := fetchFreeHTMLResource(c, h, host, "ceping", item.ID); content != "" {
				entries = append(entries, htmlEntry(item.Title, content, map[string]any{"kind": "exercise", "exercise_id": item.ID}))
			}
		}
	}
	return title, entries, nil
}

func parseFreeItems(body string) []imoocCourseEntry {
	re := regexp.MustCompile(`(?is)<a\s+[^>]*href=['"]/(video|code|ceping)/(\d+)['"][^>]*>(.*?)</a>`)
	counts := map[string]int{}
	var out []imoocCourseEntry
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		kind := m[1]
		if kind == "ceping" {
			kind = "exercise"
		}
		counts[kind]++
		title := cleanHTMLTitle(firstNonEmpty(match1(m[3], `\d+-\d+\s+([^\n<]+)`), stripTags(m[3]), m[2]))
		prefix := map[string]string{"video": "[", "code": "(", "exercise": "@"}[kind]
		suffix := map[string]string{"video": "]", "code": ")", "exercise": "@"}[kind]
		out = append(out, imoocCourseEntry{Kind: kind, ID: m[2], Title: fmt.Sprintf("%s%d%s--%s", prefix, counts[kind], suffix, title)})
	}
	return out
}

func fetchCodingCourseEntries(c *util.Client, h map[string]string, host, cid string) (string, []*extractor.MediaInfo, error) {
	body, err := c.GetString(fmt.Sprintf("%s/learn/list/%s.html", host, url.QueryEscape(cid)), h)
	if err != nil {
		return "", nil, err
	}
	title := cleanHTMLTitle(firstNonEmpty(match1(body, `'Name'\s*:\s*'([^']+)'`), match1(body, `<title>(.*?)</title>`)))
	items := parsePaidLessonItems(body, cid)
	entries := make([]*extractor.MediaInfo, 0, len(items))
	for _, item := range items {
		switch item.Kind {
		case "video":
			mi, err := extractImoocVideo(c, h, host, cid, item.ID, item.Title)
			if err == nil && mi != nil {
				entries = append(entries, mi)
			}
		case "text":
			if content := fetchImoocTextContent(c, h, host, cid, item.ID); content != "" {
				entries = append(entries, htmlEntry(item.Title, content, map[string]any{"kind": "text", "mid": item.ID, "course_id": cid}))
			}
		}
	}
	return title, entries, nil
}

func fetchClassCourseEntries(c *util.Client, h map[string]string, host, cid string) (string, []*extractor.MediaInfo, error) {
	body, err := c.GetString(fmt.Sprintf("%s/sc/%s/learn", host, url.QueryEscape(cid)), h)
	if err != nil {
		return "", nil, err
	}
	title := cleanHTMLTitle(firstNonEmpty(match1(body, `<h1\s+class=['"]stage-title['"][\s\S]*?<a\s+[^>]*>([\s\S]*?)</a>`), match1(body, `<title>(.*?)-慕课网体系课</title>`), match1(body, `<title>(.*?)</title>`)))
	entries := make([]*extractor.MediaInfo, 0)
	for _, item := range parseClassDirectLessonItems(body) {
		courseID := firstNonEmpty(item.CourseID, cid)
		if item.Kind == "text" {
			if content := fetchImoocTextContent(c, h, host, courseID, item.ID); content != "" {
				entries = append(entries, htmlEntry(item.Title, content, map[string]any{"kind": "text", "mid": item.ID, "course_id": courseID}))
			}
			continue
		}
		mi, err := extractImoocVideo(c, h, host, courseID, item.ID, item.Title)
		if err == nil && mi != nil {
			entries = append(entries, mi)
		}
	}
	for i, courseID := range parseClassInnerCourseIDs(body) {
		entries = append(entries, fetchClassInnerEntries(c, h, host, courseID, i+1)...)
	}
	return title, entries, nil
}

func parsePaidLessonItems(body, courseID string) []imoocCourseEntry {
	re := regexp.MustCompile(`(?is)<em\s+class=['"]type-text['"]>\s*(视频|图文)[\s\S]*?</em>\s*<a\s+[^>]*href=['"]/lesson/\d+\.html#mid=(\d+)['"][^>]*>[\s\S]*?<span\s+class=['"]title_info['"]>([\s\S]*?)</span>`)
	var out []imoocCourseEntry
	video, text := 0, 0
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		kind := "video"
		idx := 0
		if strings.Contains(m[1], "图文") {
			kind = "text"
			text++
			idx = text
		} else {
			video++
			idx = video
		}
		brackets := [2]string{"[", "]"}
		if kind == "text" {
			brackets = [2]string{"(", ")"}
		}
		out = append(out, imoocCourseEntry{Kind: kind, ID: m[2], CourseID: courseID, Title: cleanHTMLTitle(fmt.Sprintf("%s%d%s--%s", brackets[0], idx, brackets[1], stripTags(m[3])))})
	}
	return out
}

func parseClassDirectLessonItems(body string) []imoocCourseEntry {
	re := regexp.MustCompile(`(?is)<div\s+class=['"]media-box['"][^>]*>(.*?)</div>\s*</div>`)
	var out []imoocCourseEntry
	video, text := 0, 0
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		block := m[1]
		link := regexp.MustCompile(`(?is)<a\s+[^>]*href=['"]/lesson/(\d+)[^'"]*mid=(\d+)[^'"]*['"][^>]*>(.*?)</a>`).FindStringSubmatch(block)
		if len(link) == 0 {
			continue
		}
		kind := "video"
		idx := 0
		if strings.Contains(block, "图文") {
			kind = "text"
			text++
			idx = text
		} else {
			video++
			idx = video
		}
		brackets := [2]string{"[", "]"}
		if kind == "text" {
			brackets = [2]string{"(", ")"}
		}
		title := cleanHTMLTitle(firstNonEmpty(match1(block, `<h2[^>]*class=['"]media-title['"][^>]*>([\s\S]*?)</h2>`), stripTags(link[3]), link[2]))
		out = append(out, imoocCourseEntry{Kind: kind, CourseID: link[1], ID: link[2], Title: fmt.Sprintf("%s%d%s--%s", brackets[0], idx, brackets[1], title)})
	}
	return out
}

func parseClassInnerCourseIDs(body string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range regexp.MustCompile(`(?i)(?:href|data-url)=['"]?/course/(\d+)`).FindAllStringSubmatch(body, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

func fetchClassInnerEntries(c *util.Client, h map[string]string, host, courseID string, index int) []*extractor.MediaInfo {
	body, err := c.GetString(fmt.Sprintf("%s/course/%s", host, url.QueryEscape(courseID)), h)
	if err != nil {
		return nil
	}
	var entries []*extractor.MediaInfo
	for i, item := range parseAttachmentItems(body) {
		entries = append(entries, fileEntry(fmt.Sprintf("(%d.%d)--%s", index, i+1, item.Title), item.URL, map[string]any{"kind": "attachment", "course_id": courseID}))
	}
	for _, item := range parsePaidLessonItems(body, courseID) {
		if item.Kind == "text" {
			if content := fetchImoocTextContent(c, h, host, courseID, item.ID); content != "" {
				entries = append(entries, htmlEntry(item.Title, content, map[string]any{"kind": "text", "mid": item.ID, "course_id": courseID}))
			}
			continue
		}
		mi, err := extractImoocVideo(c, h, host, courseID, item.ID, item.Title)
		if err == nil && mi != nil {
			entries = append(entries, mi)
		}
	}
	return entries
}

func parseAttachmentItems(body string) []imoocCourseEntry {
	re := regexp.MustCompile(`(?is)<li\s+class=['"]clearfix['"][^>]*>\s*<a\s+[^>]*href=['"]([^'"]+)['"][^>]*>[\s\S]*?<span\s+class=['"]text['"]>([\s\S]*?)</span>`)
	var out []imoocCourseEntry
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		u := strings.TrimSpace(html.UnescapeString(m[1]))
		if strings.HasPrefix(u, "//") {
			u = "https:" + u
		} else if strings.HasPrefix(u, "/") {
			u = "https://class.imooc.com" + u
		}
		out = append(out, imoocCourseEntry{Kind: "file", URL: u, Title: cleanHTMLTitle(stripTags(m[2]))})
	}
	return out
}

func fetchFreeMongoID(c *util.Client, h map[string]string, mid string) string {
	body, err := c.GetString(fmt.Sprintf("https://www.imooc.com/video/%s", url.QueryEscape(mid)), h)
	if err != nil {
		return ""
	}
	return firstNonEmpty(match1(body, `mongo_id\s*=\s*"(.*?)"`), match1(body, `mongo_id\s*=\s*'(.*?)'`))
}

func fetchFreeHTMLResource(c *util.Client, h map[string]string, host, kind, id string) string {
	body, err := c.GetString(fmt.Sprintf("%s/%s/%s", host, kind, url.QueryEscape(id)), h)
	if err != nil {
		return ""
	}
	if kind == "code" {
		return normalizeHTML(firstNonEmpty(match1(body, `(?is)<div\s+id=['"]J_PanelCode['"][^>]*>([\s\S]*?)</div>`), match1(body, `(?is)<div\s+class=['"]J_PanelCode['"][^>]*>([\s\S]*?)</div>`)))
	}
	info := match1(body, `(?is)<div\s+class=['"]examinfo['"][^>]*>([\s\S]*?)</div>`)
	option := match1(body, `(?is)<div\s+class=['"]examOption['"][^>]*>([\s\S]*?)</div>`)
	return normalizeHTML(info + option)
}

func fetchImoocTextContent(c *util.Client, h map[string]string, host, cid, mid string) string {
	api := fmt.Sprintf("%s/lesson/ajaxmediainfo?mid=%s", host, url.QueryEscape(mid))
	if strings.Contains(host, "coding.imooc.com") && cid != "" {
		api += "&cid=" + url.QueryEscape(cid)
	}
	hh := map[string]string{"X-Requested-With": "XMLHttpRequest"}
	for k, v := range h {
		hh[k] = v
	}
	body, err := c.GetString(api, hh)
	if err != nil {
		return ""
	}
	var env map[string]any
	if json.Unmarshal([]byte(body), &env) != nil {
		return ""
	}
	media := asMap(asMap(env["data"])["media_info"])
	if len(media) == 0 {
		return ""
	}
	if desc := firstString(media, "program_description"); desc != "" {
		return normalizeHTML(desc)
	}
	if arr, ok := media["content_md"].([]any); ok {
		var parts []string
		for _, it := range arr {
			m := asMap(it)
			if firstString(m, "type") == "string" {
				parts = append(parts, firstString(m, "content"))
			}
		}
		return normalizeHTML(strings.Join(parts, "\n"))
	}
	return ""
}

func htmlEntry(title, content string, extra map[string]any) *extractor.MediaInfo {
	if content == "" {
		return nil
	}
	return &extractor.MediaInfo{
		Site:  "imooc",
		Title: cleanHTMLTitle(title),
		Streams: map[string]extractor.Stream{"html": {
			Quality: "html",
			URLs:    []string{dataHTMLURL(content)},
			Format:  "html",
		}},
		Extra: mergeExtra(extra, map[string]any{"html_content": content}),
	}
}

func fileEntry(title, rawURL string, extra map[string]any) *extractor.MediaInfo {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	return &extractor.MediaInfo{Site: "imooc", Title: cleanHTMLTitle(title), Streams: map[string]extractor.Stream{"file": {Quality: "file", URLs: []string{rawURL}, Format: pickFormat(rawURL), Headers: map[string]string{"Referer": "https://class.imooc.com/"}}}, Extra: mergeExtra(extra, map[string]any{"file_url": rawURL})}
}

func dataHTMLURL(content string) string {
	return "data:text/html;charset=utf-8," + url.PathEscape(content)
}

func normalizeHTML(s string) string {
	s = strings.TrimSpace(html.UnescapeString(s))
	s = strings.ReplaceAll(s, `src="//`, `src="http://`)
	s = strings.ReplaceAll(s, `href="//`, `href="http://`)
	s = strings.ReplaceAll(s, `src="\/\/`, `src="http://`)
	s = strings.ReplaceAll(s, `href="\/\/`, `href="http://`)
	return s
}

func stripTags(s string) string {
	s = regexp.MustCompile(`(?is)<script[\s\S]*?</script>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?is)<style[\s\S]*?</style>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

func cleanHTMLTitle(s string) string {
	s = strings.TrimSpace(stripTags(s))
	if s == "" {
		return "imooc"
	}
	return regexp.MustCompile(`[\\/:*?"<>|\r\n\t]+`).ReplaceAllString(s, "_")
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func firstString(m map[string]any, keys ...string) string {
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

func mergeExtra(base map[string]any, more map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for k, v := range more {
		base[k] = v
	}
	return base
}

func match1(s, pat string) string {
	if m := regexp.MustCompile(pat).FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(html.UnescapeString(m[1]))
	}
	return ""
}

func pickFormat(raw string) string {
	p := strings.ToLower(strings.SplitN(strings.SplitN(raw, "?", 2)[0], "#", 2)[0])
	if i := strings.LastIndex(p, "."); i >= 0 && i < len(p)-1 {
		ext := strings.TrimSpace(p[i+1:])
		if ext != "" && len(ext) <= 6 {
			return ext
		}
	}
	if strings.Contains(strings.ToLower(raw), ".m3u8") {
		return "m3u8"
	}
	return "mp4"
}
