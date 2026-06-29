package duanshu

import (
	"fmt"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

func normalizeType(kind string) string {
	s := strings.ToLower(strings.TrimSpace(kind))
	switch s {
	case "text", "image_text", "graphic", "article":
		return "article"
	case "class", "cource", "series", "course":
		return "course"
	case "column":
		return "column"
	case "audio":
		return "audio"
	case "video":
		return "video"
	default:
		return "single"
	}
}

func collectContentItems(v any) []contentItem {
	var out []contentItem
	var walk func(any, bool)
	walk = func(x any, inList bool) {
		switch vv := x.(type) {
		case map[string]any:
			id := valueString(vv, "content_id", "contentId", "id")
			title := valueString(vv, "content_title", "title", "name")
			kind := normalizeType(firstNonEmpty(valueString(vv, "content_type", "type"), "single"))
			if id != "" && (inList || hasAny(vv, "content_id", "contentId", "content_type", "type", "is_test", "raw_title")) {
				out = append(out, contentItem{ID: id, Title: title, Kind: kind, Test: truthy(vv["is_test"])})
			}
			for _, k := range []string{"content_list", "list", "items", "data", "response"} {
				if child, ok := vv[k]; ok {
					childInList := inList || duanshuContentListKey(k) || (k == "data" && hasAny(vv, "page", "last_page", "total_pages"))
					walk(child, childInList)
				}
			}
		case []any:
			for _, child := range vv {
				walk(child, inList)
			}
		}
	}
	walk(v, false)
	return out
}

func collectClassItems(v any) []contentItem {
	var out []contentItem
	var walk func(any, string, bool)
	walk = func(x any, prefix string, inList bool) {
		switch vv := x.(type) {
		case map[string]any:
			title := firstNonEmpty(valueString(vv, "title", "name"), prefix)
			classID := valueString(vv, "class_id", "classId", "id")
			if classID != "" && (inList || hasAny(vv, "class_id", "classId") || hasAny(vv, "course_id", "chapter_idx")) {
				out = append(out, contentItem{Class: classID, Title: title})
			}
			for _, k := range []string{"classes", "class_list", "classList", "contents", "children", "list", "data", "response"} {
				if child, ok := vv[k]; ok {
					childInList := inList || duanshuClassListKey(k) || (k == "data" && hasAny(vv, "page", "last_page", "total_pages"))
					walk(child, title, childInList)
				}
			}
		case []any:
			for _, child := range vv {
				walk(child, prefix, inList)
			}
		}
	}
	walk(v, "", false)
	return out
}

func duanshuContentListKey(key string) bool {
	switch key {
	case "content_list", "list", "items":
		return true
	default:
		return false
	}
}

func duanshuClassListKey(key string) bool {
	switch key {
	case "classes", "class_list", "classList", "contents", "children", "list":
		return true
	default:
		return false
	}
}

func findMediaURL(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range []string{"url", "video_path", "video_url", "video_patch", "audio_url", "audio_path", "m3u8", "play_url", "playUrl"} {
			if s := normalizeURL(valueString(x, k)); isMediaURL(s) {
				return s
			}
		}
		for _, k := range []string{"play_data", "audio_data", "content", "data", "response"} {
			if child, ok := x[k]; ok {
				if s := findMediaURL(child); s != "" {
					return s
				}
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

func collectStringsByKeys(v any, keys ...string) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(any)
	walk = func(x any) {
		switch vv := x.(type) {
		case map[string]any:
			for _, key := range keys {
				if s := valueString(vv, key); s != "" && !seen[s] {
					seen[s] = true
					out = append(out, s)
				}
			}
			for _, child := range vv {
				walk(child)
			}
		case []any:
			for _, child := range vv {
				walk(child)
			}
		}
	}
	walk(v)
	return out
}

func pickTitle(v any) string {
	switch x := v.(type) {
	case map[string]any:
		if s := valueString(x, "title", "name", "content_title", "raw_title"); s != "" {
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

func hasNextPage(v any, page int) bool {
	var found bool
	var walk func(any)
	walk = func(x any) {
		if found {
			return
		}
		m, ok := x.(map[string]any)
		if !ok {
			if a, ok := x.([]any); ok {
				for _, child := range a {
					walk(child)
				}
			}
			return
		}
		if p, ok := m["page"].(map[string]any); ok {
			last := intValue(p["last_page"])
			if last == 0 {
				last = intValue(p["total_pages"])
			}
			found = last > page
			return
		}
		for _, child := range m {
			walk(child)
		}
	}
	walk(v)
	return found
}

func mediaInfo(title, mediaURL string, headers map[string]string) *extractor.MediaInfo {
	format := formatFromURL(mediaURL)
	return &extractor.MediaInfo{Site: "duanshu", Title: util.SanitizeFilename(title), Streams: map[string]extractor.Stream{"best": {Quality: "best", URLs: []string{mediaURL}, Format: format, NeedMerge: format == "m3u8", Headers: headers}}}
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
	s = strings.ReplaceAll(s, `\/`, `/`)
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	return s
}

func isMediaURL(s string) bool {
	low := strings.ToLower(s)
	return strings.HasPrefix(low, "http") && (strings.Contains(low, ".mp4") || strings.Contains(low, ".m3u8") || strings.Contains(low, ".mp3") || strings.Contains(low, ".flv") || strings.Contains(low, ".m4a") || isDownloadURL(low))
}

func isDownloadURL(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if !strings.HasPrefix(low, "http") {
		return false
	}
	for _, ext := range []string{".pdf", ".ppt", ".pptx", ".doc", ".docx", ".xls", ".xlsx", ".zip", ".rar", ".7z", ".txt"} {
		if strings.Contains(low, ext) {
			return true
		}
	}
	return false
}

func firstValueByKeys(v any, keys ...string) string {
	sought := map[string]bool{}
	for _, key := range keys {
		sought[key] = true
	}
	var out string
	var walk func(any)
	walk = func(x any) {
		if out != "" {
			return
		}
		switch vv := x.(type) {
		case map[string]any:
			for key, value := range vv {
				if sought[key] {
					out = strings.TrimSpace(fmt.Sprint(value))
					if out != "" && out != "<nil>" {
						return
					}
				}
			}
			for _, child := range vv {
				walk(child)
			}
		case []any:
			for _, child := range vv {
				walk(child)
			}
		}
	}
	walk(v)
	return out
}

func intValue(v any) int {
	var n int
	_, _ = fmt.Sscanf(fmt.Sprint(v), "%d", &n)
	return n
}

func truthy(v any) bool {
	s := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
	return s != "" && s != "0" && s != "false" && s != "<nil>"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
