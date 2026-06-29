package zhengbao

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

var directVideoKeys = []string{"videoUrl", "videoURL", "video_url", "videoPath", "video_path", "playUrl", "playURL", "m3u8Url", "m3u8URL", "mp4Url", "mediaUrl"}
var directFileKeys = []string{"fileUrl", "fileURL", "file_url", "downloadUrl", "downloadURL", "pdfUrl", "pdfURL", "wordUrl", "docUrl", "pptUrl", "resourceUrl", "materialUrl"}

func directVideosFromCware(cw cwareInfo) []zbVideo {
	u := directVideoURL(cw.Raw)
	if u == "" {
		return nil
	}
	vid := ""
	if m := videoIDRe.FindStringSubmatch(u); len(m) > 1 {
		vid = m[1]
	}
	if vid == "" {
		vid = firstNonEmpty(firstString(cw.Raw, "videoID", "videoId", "video_id"), "direct-1")
	}
	return []zbVideo{{
		Title:    fmt.Sprintf("[%d.1]--%s", maxInt(cw.Index, 1), firstNonEmpty(cw.Title, "课时")),
		PlayURL:  u,
		VideoID:  vid,
		CwareID:  cw.CwareID,
		Identity: cw.Identity,
	}}
}

func directFilesFromCware(cw cwareInfo) []zbFile {
	urls := directFileURLs(cw.Raw)
	out := make([]zbFile, 0, len(urls))
	for i, u := range urls {
		format := pickFormat(u)
		title := firstNonEmpty(firstString(cw.Raw, "fileName", "filename", "resourceName", "materialName"), cw.Title, "课程资料")
		if len(urls) > 1 {
			title = fmt.Sprintf("%s-%d", title, i+1)
		}
		out = append(out, zbFile{Title: cleanTitle(title), DirectURL: u, Format: format})
	}
	return out
}

func directVideoURL(m map[string]any) string {
	for _, node := range walkMaps(m) {
		for _, key := range directVideoKeys {
			if u := normalizeURL(firstString(node, key)); isVideoURL(u) {
				return u
			}
		}
	}
	for _, u := range mediaURLRe.FindAllString(fmt.Sprint(m), -1) {
		if isVideoURL(u) {
			return normalizeURL(u)
		}
	}
	return ""
}

func directFileURL(m map[string]any) string {
	urls := directFileURLs(m)
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func directFileURLs(m map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	add := func(raw string) {
		u := normalizeURL(raw)
		if u == "" || isVideoURL(u) || pickFormat(u) == "bin" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	for _, node := range walkMaps(m) {
		for _, key := range directFileKeys {
			add(firstString(node, key))
		}
		if raw := firstString(node, "url"); raw != "" && !looksPageURL(raw) {
			add(raw)
		}
	}
	for _, u := range mediaURLRe.FindAllString(fmt.Sprint(m), -1) {
		add(strings.ReplaceAll(u, `\/`, `/`))
	}
	return out
}

func isVideoURL(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, ".m3u8") || strings.Contains(lower, ".mp4")
}

func looksPageURL(raw string) bool {
	u, err := url.Parse(normalizeURL(raw))
	if err != nil {
		return false
	}
	lower := strings.ToLower(u.Path)
	if lower == "" || strings.HasSuffix(lower, "/") {
		return true
	}
	for _, suffix := range []string{".shtm", ".html", ".htm", ".jsp"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return pickFormat(lower) == "bin"
}

func marshalCompactJSON(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return []byte("{}")
	}
	return bytes.TrimSpace(buf.Bytes())
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
