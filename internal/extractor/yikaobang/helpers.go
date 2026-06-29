package yikaobang

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/Sophomoresty/mediago/internal/util"
)

var (
	ykbUnsafeTitleRe = regexp.MustCompile(`[\s\r\n\t]+`)
	ykbURLRe         = regexp.MustCompile(`(?i)https?:\\?/\\?/[^"'<>\s]+`)
)

func cloneHeaders(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func joinErrors(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil && strings.TrimSpace(err.Error()) != "" {
			parts = append(parts, err.Error())
		}
	}
	return strings.Join(parts, "; ")
}

func regexFirst(re *regexp.Regexp, text string) string {
	match := re.FindStringSubmatch(text)
	for i := 1; i < len(match); i++ {
		if strings.TrimSpace(match[i]) != "" {
			return strings.TrimSpace(match[i])
		}
	}
	return ""
}

func asMap(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return nil
}

func asList(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return nil
}

func textValue(m map[string]any, keys ...string) string {
	if len(m) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if text := anyText(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func anyText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		f := float64(v)
		if math.Trunc(f) == f {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func int64Value(values ...any) int64 {
	for _, value := range values {
		s := strings.TrimSpace(anyText(value))
		if s == "" {
			continue
		}
		s = strings.ReplaceAll(s, ",", "")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			continue
		}
		return int64(f)
	}
	return 0
}

func hasAny(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

func hasText(m map[string]any, keys ...string) bool {
	return textValue(m, keys...) != ""
}

func cleanTitle(title string) string {
	title = strings.TrimSpace(strings.Trim(title, `"'`))
	title = strings.ReplaceAll(title, "\u3000", " ")
	title = ykbUnsafeTitleRe.ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	return util.SanitizeFilename(title)
}

func normalizeYikaobangURL(raw, baseURL string, preferFileBase bool) string {
	s := strings.TrimSpace(strings.Trim(raw, `"'`))
	if s == "" || strings.EqualFold(s, "null") || strings.EqualFold(s, "undefined") || strings.EqualFold(s, "none") {
		return ""
	}
	s = strings.ReplaceAll(s, `\/`, `/`)
	s = strings.ReplaceAll(s, `\u0026`, "&")
	if strings.HasPrefix(strings.ToLower(s), `http:\/\/`) || strings.HasPrefix(strings.ToLower(s), `https:\/\/`) {
		s = strings.ReplaceAll(s, `\/`, `/`)
	}
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	if strings.HasPrefix(strings.ToLower(s), "http://") || strings.HasPrefix(strings.ToLower(s), "https://") || strings.HasPrefix(strings.ToLower(s), "data:application/vnd.apple.mpegurl") {
		return s
	}
	if strings.HasPrefix(s, "/") {
		base := baseURL
		if preferFileBase {
			base = ykbFileBase
		}
		if u, err := url.Parse(base); err == nil && u.Scheme != "" && u.Host != "" {
			u.Path = path.Join("/", s)
			u.RawQuery = ""
			return u.String()
		}
	}
	if preferFileBase && !strings.Contains(s, "://") {
		return strings.TrimRight(ykbFileBase, "/") + "/" + strings.TrimLeft(s, "/")
	}
	return s
}

func extractURLsFromText(text string) []string {
	matches := ykbURLRe.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		u := normalizeYikaobangURL(match, ykbHomeURL, false)
		u = strings.TrimRight(u, `"'),.;`)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

func ykbTextHasDownloadURL(text string) bool {
	lower := strings.ToLower(text)
	for _, needle := range []string{".m3u8", ".mp4", ".flv", ".mp3", ".m4a", ".aac", ".pdf", ".ppt", ".pptx", ".doc", ".docx", ".xls", ".xlsx", ".zip", ".rar", ".7z", "/handout/", "/download/"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func ykbLooksLikeMediaURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(lower, "data:application/vnd.apple.mpegurl") || strings.Contains(lower, ".m3u8") || strings.Contains(lower, ".mp4") || strings.Contains(lower, ".flv") || strings.Contains(lower, ".mp3") || strings.Contains(lower, ".m4a") || strings.Contains(lower, ".aac") || strings.Contains(lower, "/hls/")
}

func ykbLooksLikeFileURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	for _, ext := range []string{".pdf", ".ppt", ".pptx", ".doc", ".docx", ".xls", ".xlsx", ".zip", ".rar", ".7z", ".txt", ".md"} {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	return strings.Contains(lower, "/download/") || strings.Contains(lower, "/handout/") || strings.Contains(lower, "/source/file/")
}

func ykbFormat(raw, fallback string) string {
	if fallback = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fallback)), "."); fallback != "" {
		return fallback
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "data:application/vnd.apple.mpegurl") || strings.Contains(lower, ".m3u8") || strings.Contains(lower, "m3u8") {
		return "m3u8"
	}
	if u, err := url.Parse(raw); err == nil {
		if ext := strings.TrimPrefix(strings.ToLower(path.Ext(u.Path)), "."); ext != "" {
			return ext
		}
	}
	if ykbLooksLikeFileURL(raw) {
		return "file"
	}
	return "mp4"
}

func streamHeaders(headers map[string]string) map[string]string {
	out := cloneHeaders(headers)
	if _, ok := out["Referer"]; !ok {
		out["Referer"] = ykbRefererURL
	}
	return out
}
