package haozaixian

import (
	"encoding/json"
	"strings"
)

func (x *hzCtx) pickVideoInfoURLs(videoAddress any) []string {
	items := listAsAny(videoAddress)
	if len(items) == 0 {
		return nil
	}
	mode := x.qualityMode()
	var pickWords []string
	var fallbackWords [][]string
	switch mode {
	case "sd":
		pickWords = []string{"sd", "流畅", "720p"}
		fallbackWords = [][]string{{"hd", "清晰", "810p"}, {"fhd", "超清", "900p"}}
	case "hd":
		pickWords = []string{"hd", "清晰", "810p"}
		fallbackWords = [][]string{{"fhd", "超清", "900p"}, {"sd", "流畅", "720p"}}
	default:
		pickWords = []string{"fhd", "超清", "900p"}
		fallbackWords = [][]string{{"hd", "清晰", "810p"}, {"sd", "流畅", "720p"}}
	}
	var out []string
	match := func(words []string) []string {
		var urls []string
		for _, item := range items {
			m := asMap(item)
			heap := strings.ToLower(strings.Join([]string{str(m["definition"]), str(m["definitionType"]), str(m["title"]), str(m["resolution"]), str(m["keyName"]), str(m["name"])}, " "))
			for _, w := range words {
				if strings.Contains(heap, strings.ToLower(w)) {
					urls = append(urls, urlsFromAny(m["urls"])...)
					break
				}
			}
		}
		return urls
	}
	out = append(out, match(pickWords)...)
	for _, words := range fallbackWords {
		out = append(out, match(words)...)
	}
	fallbackIdx := []int{0, 1, 2}
	switch mode {
	case "sd":
		fallbackIdx = []int{2, 1, 0}
	case "hd":
		fallbackIdx = []int{1, 0, 2}
	}
	for _, idx := range fallbackIdx {
		if idx < len(items) {
			out = append(out, urlsFromAny(asMap(items[idx])["urls"])...)
		}
	}
	return uniqueStrings(out)
}

func (x *hzCtx) pickMultiClarityUrls(mixInfo map[string]any) []string {
	raw := mixInfo["multiClarityPlaybackVideoData"]
	if raw == nil || str(raw) == "" {
		return nil
	}
	var value any = raw
	if s, ok := raw.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(s), &parsed); err != nil {
			return nil
		}
		value = parsed
	}
	list := listAsAny(value)
	if len(list) == 0 {
		return nil
	}
	order := []string{"FHD", "HD", "SD"}
	switch x.qualityMode() {
	case "sd":
		order = []string{"SD", "HD", "FHD"}
	case "hd":
		order = []string{"HD", "FHD", "SD"}
	}
	var out []string
	for _, want := range order {
		for _, item := range list {
			m := asMap(item)
			if strings.ToUpper(str(m["definition"])) == want {
				out = append(out, urlsFromAny(m["url"])...)
			}
		}
	}
	return uniqueStrings(out)
}

func (x *hzCtx) qualityMode() string {
	s := strings.ToLower(strings.TrimSpace(x.quality))
	switch {
	case strings.Contains(s, "fhd"), strings.Contains(s, "1080"), strings.Contains(s, "900"), strings.Contains(s, "超清"):
		return "fhd"
	case strings.Contains(s, "sd"), strings.Contains(s, "480"), strings.Contains(s, "标清"):
		return "sd"
	case strings.Contains(s, "hd"), strings.Contains(s, "720"), strings.Contains(s, "高清"):
		return "hd"
	default:
		return "fhd"
	}
}

func (x *hzCtx) isPlainM3U8URL(u string) bool {
	if u == "" {
		return false
	}
	body, err := x.c.GetString(u, map[string]string{"User-Agent": x.headers["User-Agent"]})
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(body), "#EXTM3U")
}

func listAsAny(v any) []any {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		return t
	case []map[string]any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, item)
		}
		return out
	default:
		return []any{t}
	}
}

func uniqueStrings(vals []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		k := strings.ToLower(v)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}

func (x *hzCtx) getCourseEmphasisImages(lessonID, courseID string) ([]string, []string) {
	if lessonID == "" || courseID == "" {
		return nil, nil
	}
	root, err := x.requestJSON(queryURL(course_emphasis_detail_url,
		kv{k: "courseId", v: x.cid},
		kv{k: "lessonId", v: lessonID},
		kv{k: "os", v: ""},
		kv{k: "appId", v: "haokezaixianAPP"},
	), nil)
	if err != nil || str(root["errNo"]) != "0" {
		return nil, nil
	}
	data := asMap(root["data"])
	pick := func(items []map[string]any) []string {
		seen := map[string]bool{}
		var out []string
		for _, item := range items {
			u := str(item["bgImageUrl"])
			if u != "" && !seen[strings.ToLower(u)] {
				seen[strings.ToLower(u)] = true
				out = append(out, u)
			}
		}
		return out
	}
	return pick(listAt(data, "teacherList")), pick(listAt(data, "myList"))
}

func (x *hzCtx) getLessonLectureImages(lessonID, courseID string) []string {
	if lessonID == "" || courseID == "" {
		return nil
	}
	root, err := x.requestJSON(queryURL(lesson_lecture_url,
		kv{k: "courseId", v: x.cid},
		kv{k: "lessonId", v: lessonID},
		kv{k: "pn", v: "0"},
		kv{k: "rn", v: "200"},
		kv{k: "os", v: ""},
		kv{k: "appId", v: "haokezaixianAPP"},
	), nil)
	if err != nil || str(root["errNo"]) != "0" {
		return nil
	}
	rows := listAt(asMap(root["data"]), "lecture")
	var out []string
	seen := map[string]bool{}
	for _, row := range rows {
		u := str(row["bgImageUrl"])
		if u != "" && !seen[strings.ToLower(u)] {
			seen[strings.ToLower(u)] = true
			out = append(out, u)
		}
	}
	return out
}
