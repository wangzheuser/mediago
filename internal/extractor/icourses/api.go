package icourses

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var pseudoCookieNames = map[string]bool{
	"icourses_website_user_token": true,
	"icourses_website_user_info":  true,
	"icourses_token":              true,
	"icourses_user_info":          true,
}

func (x *icoursesCtx) apiGet(apiPath string, params map[string]string) (any, error) {
	u := apiPath
	if !strings.HasPrefix(u, "http") {
		u = strings.TrimRight(api_root, "/") + "/" + strings.TrimLeft(apiPath, "/")
	}
	if len(params) > 0 {
		values := url.Values{}
		for k, v := range params {
			if strings.TrimSpace(v) != "" {
				values.Set(k, v)
			}
		}
		if encoded := values.Encode(); encoded != "" {
			if strings.Contains(u, "?") {
				u += "&" + encoded
			} else {
				u += "?" + encoded
			}
		}
	}
	body, err := x.c.GetString(u, x.headers)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil, fmt.Errorf("icourses api returned non-json response: %w", err)
	}
	code := str(root["code"])
	msg := firstNonEmpty(str(root["msg"]), str(root["message"]))
	if code == "30007" || code == "30008" {
		if msg == "" {
			msg = "login required"
		}
		return nil, fmt.Errorf("icourses login required: %s", msg)
	}
	if b, ok := root["success"].(bool); ok && !b {
		if msg == "" {
			msg = "api returned error"
		}
		return nil, fmt.Errorf("icourses api returned error: %s", msg)
	}
	if code != "" && code != "0" && code != "200" && root["success"] != true {
		if msg == "" {
			msg = "api returned error"
		}
		return nil, fmt.Errorf("icourses api returned error: code=%s msg=%s", code, msg)
	}
	if data, ok := root["data"]; ok {
		return data, nil
	}
	return root, nil
}

func cookieMap(jar http.CookieJar, origins []string) map[string]string {
	out := map[string]string{}
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil {
			continue
		}
		for _, c := range jar.Cookies(u) {
			if c.Name == "" {
				continue
			}
			out[c.Name] = c.Value
		}
	}
	return out
}

func cookieHeaderFromMap(cookies map[string]string) string {
	var parts []string
	for k, v := range cookies {
		if pseudoCookieNames[k] || v == "" {
			continue
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ";")
}

func pickList(data any, keys ...string) []map[string]any {
	if list := listMaps(data); len(list) > 0 {
		return list
	}
	m := asMap(data)
	if len(m) == 0 {
		return nil
	}
	if len(keys) == 0 {
		keys = []string{"list", "records", "items", "rows", "chapterList", "resourcesList", "courseSubList", "data"}
	}
	for _, k := range keys {
		if list := listMaps(m[k]); len(list) > 0 {
			return list
		}
	}
	return nil
}

func composeTitle(courseName, schoolName, teacherName string) string {
	courseName = cleanName(courseName)
	var suffix []string
	if schoolName != "" {
		suffix = append(suffix, cleanName(schoolName))
	}
	if teacherName != "" {
		suffix = append(suffix, cleanName(teacherName))
	}
	joined := strings.Join(nonEmpty(suffix...), "_")
	if courseName != "" && joined != "" {
		return cleanName(courseName + "_" + joined)
	}
	if courseName != "" {
		return courseName
	}
	return cleanName(joined)
}

func stripExt(name string) string {
	name = cleanName(name)
	if name == "" {
		return "未命名资源"
	}
	ext := strings.ToLower(path.Ext(name))
	if ext != "" {
		switch ext {
		case ".mp4", ".m3u8", ".pdf", ".ppt", ".pptx", ".doc", ".docx", ".xls", ".xlsx", ".zip", ".rar", ".7z", ".txt", ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".caj":
			stem := strings.TrimSuffix(name, path.Ext(name))
			if stem != "" {
				return stem
			}
		}
	}
	return name
}

func unwrapResourceURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if src := u.Query().Get("src"); src != "" {
		if decoded, err := url.QueryUnescape(src); err == nil && decoded != "" {
			return decoded
		}
		return src
	}
	return raw
}

func guessExt(raw, mediaType string) string {
	u := unwrapResourceURL(raw)
	if parsed, err := url.Parse(u); err == nil {
		if ext := strings.ToLower(path.Ext(parsed.Path)); ext != "" {
			return ext
		}
	}
	mediaType = strings.ToLower(mediaType)
	switch {
	case strings.Contains(mediaType, "mp4") || strings.Contains(mediaType, "video"):
		return ".mp4"
	case strings.Contains(mediaType, "pdf"):
		return ".pdf"
	case strings.Contains(mediaType, "ppt"):
		return ".ppt"
	case strings.Contains(mediaType, "doc") || strings.Contains(mediaType, "word"):
		return ".doc"
	default:
		return ""
	}
}

func kindFromExt(ext, mediaType string) string {
	ext = strings.ToLower(ext)
	mediaType = strings.ToLower(mediaType)
	switch ext {
	case ".pdf":
		return "pdf"
	case ".ppt", ".pptx", ".pps", ".ppsx":
		return "ppt"
	case ".doc", ".docx":
		return "doc"
	case ".mp4", ".m3u8", ".m4v", ".mov", ".webm", ".flv":
		return "video"
	}
	switch {
	case mediaType == "mp4" || mediaType == "video" || mediaType == "mp4video":
		return "video"
	case strings.Contains(mediaType, "pdf"):
		return "pdf"
	case strings.Contains(mediaType, "ppt"):
		if ext == "" || ext == ".ppt" || ext == ".pptx" || ext == ".pps" || ext == ".ppsx" {
			return "ppt"
		}
	case strings.Contains(mediaType, "doc") || strings.Contains(mediaType, "word"):
		if ext == "" || ext == ".doc" || ext == ".docx" {
			return "doc"
		}
	}
	return "attach"
}

func parseSizeBytes(v any) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case float64:
		return normalizeSizeNumber(t)
	case int:
		return normalizeSizeNumber(float64(t))
	case int64:
		return normalizeSizeNumber(float64(t))
	case string:
		s := strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(t, " ", "")))
		s = strings.ReplaceAll(s, "IB", "B")
		if s == "" {
			return 0
		}
		mul := float64(1)
		switch {
		case strings.HasSuffix(s, "GB"):
			mul, s = 1024*1024*1024, strings.TrimSuffix(s, "GB")
		case strings.HasSuffix(s, "MB"):
			mul, s = 1024*1024, strings.TrimSuffix(s, "MB")
		case strings.HasSuffix(s, "KB"):
			mul, s = 1024, strings.TrimSuffix(s, "KB")
		case strings.HasSuffix(s, "G"):
			mul, s = 1024*1024*1024, strings.TrimSuffix(s, "G")
		case strings.HasSuffix(s, "M"):
			mul, s = 1024*1024, strings.TrimSuffix(s, "M")
		case strings.HasSuffix(s, "K"):
			mul, s = 1024, strings.TrimSuffix(s, "K")
		case strings.HasSuffix(s, "B"):
			mul, s = 1, strings.TrimSuffix(s, "B")
		}
		f, _ := strconv.ParseFloat(s, 64)
		return int64(f * mul)
	default:
		return 0
	}
}

func normalizeSizeNumber(f float64) int64 {
	if f <= 0 {
		return 0
	}
	if f < 1024 {
		return int64(f * 1024 * 1024)
	}
	if f < 1024*1024 {
		return int64(f * 1024)
	}
	return int64(f)
}

func cleanName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(s)
	s = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]+`).ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func nonEmpty(vals ...string) []string {
	out := vals[:0]
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, strings.TrimSpace(v))
		}
	}
	return out
}

func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	case float64:
		if math.Trunc(t) == t {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func listMaps(v any) []map[string]any {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
