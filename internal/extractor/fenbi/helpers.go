package fenbi

import (
	"fmt"
	"strings"

	"github.com/nichuanfang/medigo/internal/extractor"
	"github.com/nichuanfang/medigo/internal/util"
)

func collectEpisodes(v any) []episodeNode {
	seen := map[string]bool{}
	var out []episodeNode
	var walk func(any, string)
	walk = func(x any, title string) {
		switch vv := x.(type) {
		case map[string]any:
			nextTitle := firstNonEmpty(valueString(vv, "title", "name", "episodeTitle", "lessonTitle", "episodeName", "videoName", "video_name", "coursewareName"), title)
			id := valueString(vv, "id", "episodeId", "episode_id", "episode_id_str", "videoId", "video_id", "contentId")
			if id != "" && !seen[id] && (hasAny(vv, "episodeId", "episode_id", "episode_id_str", "videoId", "video_id") || hasAny(vv, "mediafile", "mediaFile", "duration")) {
				seen[id] = true
				out = append(out, episodeNode{ID: id, Title: nextTitle, Raw: vv})
			}
			for _, k := range []string{"episodes", "episodeList", "episodeNodes", "nodes", "lessons", "lessonList", "tasks", "taskList", "items", "list", "children", "syllabus", "contents", "chapters", "chapterList", "units", "unitList", "data"} {
				if child, ok := vv[k]; ok {
					walk(child, nextTitle)
				}
			}
		case []any:
			for _, child := range vv {
				walk(child, title)
			}
		}
	}
	walk(v, "")
	return out
}

func findMediaURL(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range []string{"url", "mediaUrl", "media_url", "path", "downloadUrl", "download_url", "fileUrl", "file_url", "playUrl", "m3u8"} {
			if s := normalizeURL(valueString(x, k)); isMediaURL(s) {
				return s
			}
		}
		for _, k := range []string{"mediaFiles", "qualities", "mediaList", "mediaSizes", "streamList", "videoList", "definitions", "urls", "files", "list", "streams", "data"} {
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

func pickTitle(v any) string {
	switch x := v.(type) {
	case map[string]any:
		if s := valueString(x, "courseTitle", "lectureTitle", "lectureSetTitle", "title", "name", "episodeTitle", "episodeName", "videoName"); s != "" {
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

func mediaInfo(title, mediaURL string, headers map[string]string) *extractor.MediaInfo {
	format := "mp4"
	if strings.Contains(strings.ToLower(mediaURL), ".m3u8") {
		format = "m3u8"
	}
	return &extractor.MediaInfo{Site: "fenbi", Title: util.SanitizeFilename(title), Streams: map[string]extractor.Stream{"best": {Quality: "best", URLs: []string{mediaURL}, Format: format, Headers: headers}}}
}

func valueString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" && s != "0" {
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
	low := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(low, "http") && (strings.Contains(low, ".m3u8") || strings.Contains(low, ".mp4") || strings.Contains(low, ".flv") || strings.Contains(low, ".mov") || strings.Contains(low, ".m4v") || strings.Contains(low, ".mp3") || strings.Contains(low, ".m4a") || strings.Contains(low, ".aac") || strings.Contains(low, ".wav"))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
