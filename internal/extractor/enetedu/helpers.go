package enetedu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

type fileInfo struct {
	Title      string
	URL        string
	Format     string
	SourceType string
	Raw        map[string]any
}

func apiURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return api_base + path
}

func dataOf(payload any) map[string]any {
	return dataOfAny(payload)
}

func dataOfAny(payload any) map[string]any {
	if m, ok := payload.(map[string]any); ok {
		if d, ok := m["data"].(map[string]any); ok {
			return d
		}
		return m
	}
	return map[string]any{}
}

func walkLivePayload(v any, out *[]videoInfo) {
	switch x := v.(type) {
	case map[string]any:
		title := valueString(x, "name", "title", "realId", "id")
		nodeID := valueString(x, "realId", "id", "node_id")
		media := firstNonEmpty(valueString(x, "playbackUrl", "sourceAddress", "url"), findMediaURL(x))
		if nodeID != "" || isMediaURL(media) {
			*out = append(*out, videoInfo{Title: title, NodeID: nodeID, URL: normalizeURL(media), Raw: x})
		}
		for _, k := range []string{"children", "childList", "list", "data", "records"} {
			if child, ok := x[k]; ok {
				walkLivePayload(child, out)
			}
		}
	case []any:
		for _, child := range x {
			walkLivePayload(child, out)
		}
	}
}

func walkLearningPayload(v any, out *[]videoInfo) {
	switch x := v.(type) {
	case map[string]any:
		title := valueString(x, "fileName", "mediaName", "chapterName", "courseName", "name")
		videoID := valueString(x, "videoId", "mediaId")
		media := firstNonEmpty(valueString(x, "filePath", "playUrl", "url"), findMediaURL(x))
		if videoID != "" || isMediaURL(media) {
			*out = append(*out, videoInfo{Title: title, VideoID: videoID, URL: normalizeURL(media), FileName: valueString(x, "fileName"), ChapterID: valueString(x, "chapterId", "id"), Raw: x})
		}
		for _, k := range []string{"children", "childList", "list", "data", "records"} {
			if child, ok := x[k]; ok {
				walkLearningPayload(child, out)
			}
		}
	case []any:
		for _, child := range x {
			walkLearningPayload(child, out)
		}
	}
}

func findMediaURL(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range []string{"playUrl", "url", "filePath", "sourceAddress", "playbackUrl", "downloadUrl", "resourceUrl", "fileUrl"} {
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

func parseNoticeFiles(detail map[string]any) []fileInfo {
	notice := valueString(detail, "courseNotice")
	if notice == "" {
		return nil
	}
	var payload any
	if err := json.Unmarshal([]byte(notice), &payload); err != nil {
		payload = notice
	}
	var out []fileInfo
	walkNoticePayload(payload, &out)
	return out
}

func walkNoticePayload(v any, out *[]fileInfo) {
	switch x := v.(type) {
	case map[string]any:
		rawURL := firstNonEmpty(valueString(x, "url", "fileUrl", "filePath", "path", "downloadUrl", "resourceUrl"))
		if rawURL != "" {
			fileURL := normalizeURL(rawURL)
			if isDownloadableURL(fileURL) {
				*out = append(*out, fileInfo{
					Title:      firstNonEmpty(valueString(x, "fileName", "name", "title"), "课程通知"),
					URL:        fileURL,
					Format:     fileFormat(fileURL),
					SourceType: "notice",
					Raw:        x,
				})
			}
		}
		for _, child := range x {
			walkNoticePayload(child, out)
		}
	case []any:
		for _, child := range x {
			walkNoticePayload(child, out)
		}
	}
}

func walkFilePayload(v any, out *[]fileInfo) {
	switch x := v.(type) {
	case map[string]any:
		rawURL := firstNonEmpty(valueString(x, "url", "fileUrl", "filePath", "path", "downloadUrl", "resourceUrl"))
		if rawURL != "" {
			fileURL := normalizeURL(rawURL)
			if isDownloadableURL(fileURL) {
				*out = append(*out, fileInfo{
					Title:      firstNonEmpty(valueString(x, "fileName", "name", "title", "resourceName"), fileTitleFromURL(fileURL), "课程资料"),
					URL:        fileURL,
					Format:     fileFormat(fileURL),
					SourceType: "course_file",
					Raw:        x,
				})
			}
		}
		for _, key := range []string{"children", "list", "records", "courseResourcesList", "resources", "data"} {
			if child, ok := x[key]; ok {
				walkFilePayload(child, out)
			}
		}
	case []any:
		for _, child := range x {
			walkFilePayload(child, out)
		}
	}
}

func mediaInfo(title, mediaURL string, headers map[string]string) *extractor.MediaInfo {
	format := "mp4"
	low := strings.ToLower(mediaURL)
	if strings.Contains(low, ".m3u8") {
		format = "m3u8"
	} else if strings.Contains(low, ".mp3") || strings.Contains(low, ".m4a") || strings.Contains(low, ".aac") || strings.Contains(low, ".wav") {
		format = "audio"
	}
	return &extractor.MediaInfo{Site: "enetedu", Title: util.SanitizeFilename(title), Streams: map[string]extractor.Stream{"best": {Quality: "best", URLs: []string{mediaURL}, Format: format, Headers: headers}}}
}

func fileEntries(files []fileInfo, headers map[string]string) []*extractor.MediaInfo {
	entries := make([]*extractor.MediaInfo, 0, len(files))
	for _, file := range files {
		fileURL := normalizeURL(file.URL)
		if !isDownloadableURL(fileURL) {
			continue
		}
		format := firstNonEmpty(file.Format, fileFormat(fileURL), "bin")
		title := strings.TrimSuffix(util.SanitizeFilename(firstNonEmpty(file.Title, fileTitleFromURL(fileURL), "课程资料")), "."+format)
		entries = append(entries, &extractor.MediaInfo{
			Site:  "enetedu",
			Title: title,
			Streams: map[string]extractor.Stream{"best": {
				Quality: "best",
				URLs:    []string{fileURL},
				Format:  format,
				Headers: cloneHeaders(headers),
			}},
			Extra: map[string]any{"type": "file", "source_type": file.SourceType, "raw": file.Raw},
		})
	}
	return entries
}

func dedupe(in []*extractor.MediaInfo) []*extractor.MediaInfo {
	seen := map[string]bool{}
	out := make([]*extractor.MediaInfo, 0, len(in))
	for _, item := range in {
		if item == nil || len(item.Streams) == 0 {
			continue
		}
		key := item.Title
		for _, st := range item.Streams {
			if len(st.URLs) > 0 {
				key += "|" + st.URLs[0]
			}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
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

func normalizeURL(s string) string {
	s = strings.TrimSpace(strings.Trim(s, `"'`))
	s = strings.ReplaceAll(s, `\/`, `/`)
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	if strings.HasPrefix(s, "/") {
		return origin + s
	}
	return s
}

func isMediaURL(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(low, "http") && (strings.Contains(low, ".m3u8") || strings.Contains(low, ".mp4") || strings.Contains(low, ".flv") || strings.Contains(low, ".mov") || strings.Contains(low, ".m4v") || strings.Contains(low, ".mp3") || strings.Contains(low, ".m4a") || strings.Contains(low, ".aac") || strings.Contains(low, ".wav"))
}

func isDownloadableURL(s string) bool {
	return isMediaURL(s) || isFileURL(s)
}

func isFileURL(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if !strings.HasPrefix(low, "http") {
		return false
	}
	return regexp.MustCompile(`\.(?:pdf|pptx?|docx?|xlsx?|xls|zip|rar|7z|caj)(?:[?#]|$)`).MatchString(low)
}

func fileFormat(raw string) string {
	low := strings.ToLower(raw)
	if strings.Contains(low, ".m3u8") {
		return "m3u8"
	}
	u, err := url.Parse(raw)
	path := raw
	if err == nil {
		path = u.Path
	}
	if i := strings.LastIndex(path, "."); i >= 0 && i < len(path)-1 {
		return strings.ToLower(path[i+1:])
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
		if ext := fileFormat(name); ext != "" {
			return strings.TrimSuffix(name, "."+ext)
		}
		return name
	}
	return ""
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
