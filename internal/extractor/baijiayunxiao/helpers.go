package baijiayunxiao

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/nichuanfang/medigo/internal/extractor"
)

var (
	coursePathRe    = regexp.MustCompile(`(?i)^https?://([^/#?]+)/(?:.*?/)?course/(\d+)`)
	evURLRe         = regexp.MustCompile(`(?i)^https?://(?:[\w-]+\.)*baijia(?:yun|cloud)\.com/.*?/([\w-]+)\.(ev[12])\S*$`)
	urlLikeRe       = regexp.MustCompile(`https?://[^\s"'<>]+`)
	tokenRe         = regexp.MustCompile(`(?i)token=([\w_-]+)|"token"\s*:\s*"([\w_-]+)"`)
	classIDRe       = regexp.MustCompile(`(?i)(?:classid|room_id)=(\d+)|"(?:video_id|room_id|classid)"\s*:\s*"?(\d+)"?`)
	directParamKeys = []string{"room_id", "classid", "vid", "token"}
)

func parsePlaybackParams(raw string) (playbackParams, bool) {
	decoded := html.UnescapeString(strings.TrimSpace(raw))
	u, err := url.Parse(decoded)
	if err != nil {
		return playbackParams{}, false
	}
	q := u.Query()
	token := q.Get("token")
	roomID := firstNonEmpty(q.Get("room_id"), q.Get("classid"))
	vid := q.Get("vid")
	if token != "" && roomID != "" {
		return playbackParams{roomID: roomID, token: token}, true
	}
	if token != "" && vid != "" {
		return playbackParams{vid: vid, token: token, isVOD: true}, true
	}
	for _, k := range directParamKeys {
		if strings.Contains(decoded, k+"=") {
			return playbackParams{}, false
		}
	}
	return playbackParams{}, false
}

func parseCourseURL(raw string) (courseURL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return courseURL{}, false
	}
	q := u.Query()
	cid := firstNonEmpty(q.Get("course_id"), q.Get("courseId"))
	domain := u.Host
	if cid == "" {
		if m := coursePathRe.FindStringSubmatch(raw); m != nil {
			domain, cid = m[1], m[2]
		}
	}
	if cid == "" || !strings.Contains(domain, "baijiayunxiao") {
		return courseURL{}, false
	}
	return courseURL{domain: domain, cid: cid}, true
}

func findPlaybackURLInText(text string) string {
	var payload any
	if json.Unmarshal([]byte(text), &payload) == nil {
		if u := findPlaybackURLInValue(payload); u != "" {
			return u
		}
	}
	for _, candidate := range urlLikeRe.FindAllString(html.UnescapeString(text), -1) {
		if _, ok := parsePlaybackParams(candidate); ok {
			return candidate
		}
	}
	return ""
}

func findPlaybackURLInValue(v any) string {
	switch x := v.(type) {
	case string:
		if _, ok := parsePlaybackParams(x); ok {
			return x
		}
	case map[string]any:
		for _, vv := range x {
			if u := findPlaybackURLInValue(vv); u != "" {
				return u
			}
		}
	case []any:
		for _, vv := range x {
			if u := findPlaybackURLInValue(vv); u != "" {
				return u
			}
		}
	}
	return ""
}

func collectLessons(nodes []courseNode, prefix []string) []lessonRef {
	var out []lessonRef
	for i, node := range nodes {
		title := firstNonEmpty(node.PeriodsTitle, node.Title, node.Name, fmt.Sprintf("课时%d", i+1))
		id := firstNonEmpty(anyString(node.ID), anyString(node.VideoID), anyString(node.RoomID))
		if id != "" {
			out = append(out, lessonRef{ID: id, Title: strings.Join(append(prefix, title), " - ")})
		}
		children := append([]courseNode{}, node.Child...)
		children = append(children, node.Children...)
		out = append(out, collectLessons(children, append(prefix, title))...)
	}
	return out
}

func mediaInfo(site, title, mediaURL, format string, headers map[string]string) *extractor.MediaInfo {
	return &extractor.MediaInfo{
		Site:  site,
		Title: title,
		Streams: map[string]extractor.Stream{
			"best": {Quality: "best", URLs: []string{mediaURL}, Format: format, Headers: headers},
		},
	}
}

func refererFromRawURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host + "/"
	}
	return urlHome
}

func pickRegex(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

func pickFormat(mediaURL string) string {
	lower := strings.ToLower(mediaURL)
	switch {
	case strings.Contains(lower, ".m3u8"):
		return "m3u8"
	case strings.Contains(lower, ".ev1"):
		return ".ev1"
	case strings.Contains(lower, ".ev2"):
		return ".ev2"
	case strings.Contains(lower, ".mp4"):
		return "mp4"
	}
	return "mp4"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func anyString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int:
		return fmt.Sprint(x)
	case json.Number:
		return x.String()
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}
