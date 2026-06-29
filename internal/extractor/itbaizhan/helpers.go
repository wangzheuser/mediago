package itbaizhan

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
)

func parseVideoInfo(child videoChild, index []int, stageID, chapterID, prefix string) itbzVideo {
	videoID := str(child.CourseID)
	name := cleanTitle(str(child.CourseName))
	if videoID == "" || name == "" {
		return itbzVideo{}
	}
	return itbzVideo{
		Name:       fmt.Sprintf("%s/[%s]--%s", prefix, joinInts(index, "."), name),
		VideoID:    videoID,
		StageID:    stageID,
		ChapterID:  chapterID,
		StageIndex: index[0],
		Extra: map[string]any{
			"video_time": str(child.VideoTime),
			"input_time": str(child.InputTime),
			"free":       str(child.Free),
			"is_free":    str(child.IsFree),
		},
	}
}

func parseFileInfo(training trainingInfo, index []int, prefix string) itbzFile {
	fileID := str(training.TID)
	name := firstNonEmpty(cleanTitle(str(training.TName)), "资料")
	if fileID == "" || name == "" {
		return itbzFile{}
	}
	return itbzFile{
		Name:   fmt.Sprintf("%s/(%s)--%s", prefix, joinInts(index, "."), name),
		URL:    referer + "index/training/write/id/" + url.PathEscape(fileID) + ".html",
		Fmt:    "html",
		FileID: fileID,
	}
}

func parseCourseRef(raw string) courseRef {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return courseRef{}
	}
	p := strings.Trim(u.Path, "/")
	if m := regexp.MustCompile(`(?i)^course/id/(\d+)\.html$`).FindStringSubmatch(p); len(m) == 2 {
		return courseRef{VideoID: m[1]}
	}
	if m := regexp.MustCompile(`(?i)^stages/id/(\d+)`).FindStringSubmatch(p); len(m) == 2 {
		return courseRef{CourseID: m[1]}
	}
	if strings.EqualFold(p, "nav/detail") {
		return courseRef{CourseID: strings.TrimSpace(u.Query().Get("id"))}
	}
	if m := regexp.MustCompile(`(?i)^course/([\w-]+)`).FindStringSubmatch(p); len(m) == 2 {
		return courseRef{Slug: m[1]}
	}
	return courseRef{CourseID: firstNonEmpty(u.Query().Get("id"), u.Query().Get("course_id"), u.Query().Get("courseId"))}
}

type playInfo struct{ PolyvVID, Playsafe, Title string }

func parsePlayInfo(text string) playInfo {
	text = strings.ReplaceAll(strings.ReplaceAll(text, `\"`, `"`), `\'`, `'`)
	info := playInfo{Title: extractTitle(text)}
	if m := regexp.MustCompile(`vid\s*:\s*['"]([^'"]+)['"]`).FindStringSubmatch(text); len(m) == 2 {
		info.PolyvVID = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`playsafe\s*:\s*['"]([^'"]+)['"]`).FindStringSubmatch(text); len(m) == 2 {
		info.Playsafe = strings.TrimSpace(m[1])
	}
	return info
}

func extractTitle(text string) string {
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?is)<div\s+class=["']zhang_course["'][^>]*>(.*?)</div>`),
		regexp.MustCompile(`(?is)<title>(.*?)</title>`),
	} {
		if m := re.FindStringSubmatch(text); len(m) == 2 {
			title := regexp.MustCompile(`(?is)<.*?>`).ReplaceAllString(m[1], "")
			title = html.UnescapeString(strings.TrimSpace(title))
			title = regexp.MustCompile(`[-_]-百战程序员|[-_]-百战未来|[-_]-尚学堂`).Split(title, 2)[0]
			return cleanTitle(title)
		}
	}
	return ""
}

func extractStageIDs(text, cid string) []string {
	ids := []string{}
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?is)<li\b[^>]*\bdata-id=["'](\d+)["'][^>]*>.*?</li>`).FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	if len(ids) == 0 {
		for _, m := range regexp.MustCompile(`data-id=["'](\d+)["']`).FindAllStringSubmatch(text, -1) {
			if m[1] != cid && !seen[m[1]] {
				seen[m[1]] = true
				ids = append(ids, m[1])
			}
		}
	}
	return ids
}

func (x *itbzCtx) courseIDForSlug(rawURL, slug string) string {
	if slug == "" {
		return ""
	}
	target, _ := url.Parse(rawURL)
	targetPath := strings.Trim(strings.ToLower(target.Path), "/")
	for _, c := range x.getCourseList() {
		courseURL := strings.Trim(strings.ToLower(c["url"]), "/")
		if courseURL == targetPath || strings.Contains(courseURL, slug) {
			x.title = c["title"]
			return c["course_id"]
		}
	}
	return ""
}

func (x *itbzCtx) firstPurchasedCourse() (string, string) {
	for _, c := range x.getCourseList() {
		if c["course_id"] != "" {
			return c["course_id"], c["title"]
		}
	}
	return "", ""
}

func (x *itbzCtx) getCourseList() []map[string]string {
	body, err := x.requestText(course_list_url, map[string]string{"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"})
	if err != nil || body == "" {
		return nil
	}
	return parseMineCourseList(body)
}

var perInfoSplitRe = regexp.MustCompile(`(?is)<div\b[^>]*class=["'][^"']*\bper_info\b[^"']*["'][^>]*>`)

func parseMineCourseList(text string) []map[string]string {
	var out []map[string]string
	seen := map[string]bool{}
	parts := perInfoSplitRe.Split(text, -1)
	for _, body := range parts[1:] {
		title := ""
		if m := regexp.MustCompile(`(?is)<p\b[^>]*class=["'][^"']*\bper_info_title\b[^"']*["'][^>]*>(.*?)</p>`).FindStringSubmatch(body); len(m) == 2 {
			title = cleanTitle(regexp.MustCompile(`(?is)<.*?>`).ReplaceAllString(m[1], ""))
		}
		if title == "" {
			if m := regexp.MustCompile(`(?is)<img\b[^>]*\balt=["'](?P<title>[^"']+)["']`).FindStringSubmatch(body); len(m) == 2 {
				title = cleanTitle(m[1])
			}
		}
		for _, m := range regexp.MustCompile(`(?is)<a\b[^>]*\bhref=["'](?P<url>[^"']+)["']`).FindAllStringSubmatch(body, -1) {
			if id := stageIDFromURL(m[1]); id != "" && title != "" && !seen[id] {
				seen[id] = true
				out = append(out, map[string]string{"purchased": "true", "url": m[1], "course_id": id, "title": title})
				break
			}
		}
	}
	return out
}

func stageIDFromURL(raw string) string {
	u, _ := url.Parse(html.UnescapeString(raw))
	if m := regexp.MustCompile(`(?i)(?:^|/)stages/id/(\d+)`).FindStringSubmatch(strings.Trim(u.Path, "/")); len(m) == 2 {
		return m[1]
	}
	if strings.EqualFold(strings.Trim(u.Path, "/"), "nav/detail") {
		return u.Query().Get("id")
	}
	return ""
}

func formatPolyvVID(vid string) string {
	vid = strings.TrimSpace(vid)
	if vid == "" || strings.Contains(vid, "_") {
		return vid
	}
	return fmt.Sprintf("%s_%s", vid, vid[:1])
}

func cookieHeader(jar http.CookieJar, origins []string) string {
	seen := map[string]bool{}
	var parts []string
	for _, raw := range origins {
		u, _ := url.Parse(raw)
		for _, c := range jar.Cookies(u) {
			key := c.Name + "=" + c.Value
			if !seen[key] {
				seen[key] = true
				parts = append(parts, key)
			}
		}
	}
	return strings.Join(parts, "; ")
}

func cloneHeaders(h map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range h {
		out[k] = v
	}
	return out
}

func mergeUnique(base []string, more ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range append(base, more...) {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.0f", t), "0"), ".")
	default:
		if v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func cleanTitle(s string) string {
	s = html.UnescapeString(s)
	s = regexp.MustCompile(`(?is)<.*?>`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func joinInts(xs []int, sep string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprint(x)
	}
	return strings.Join(parts, sep)
}

func extFormat(raw string) string {
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(urlPath(raw))), ".")
	if ext == "m3u8" || ext == "mp4" {
		return ext
	}
	return ext
}

func urlPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Path
}
