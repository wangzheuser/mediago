package haozaixian

import (
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type kv struct{ k, v string }

func encodePairs(pairs []kv) string {
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, url.QueryEscape(p.k)+"="+url.QueryEscape(p.v))
	}
	return strings.Join(parts, "&")
}

func queryURL(base string, pairs ...kv) string {
	if len(pairs) == 0 {
		return base
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + encodePairs(pairs)
}

func cookieHeader(jar http.CookieJar, origins []string) string {
	seen := map[string]bool{}
	var parts []string
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil {
			continue
		}
		for _, c := range jar.Cookies(u) {
			if c.Name == "" || seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			parts = append(parts, c.Name+"="+c.Value)
		}
	}
	return strings.Join(parts, "; ")
}

func cloneHeaders(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
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

func listAt(m map[string]any, key string) []map[string]any { return listMaps(m[key]) }

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := str(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
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
		if math.Trunc(t) == t {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		f := float64(t)
		if math.Trunc(f) == f {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func intVal(v any) int {
	s := str(v)
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return int(f)
}

func truthy(v any) bool {
	s := strings.ToLower(str(v))
	return s != "" && s != "0" && s != "false" && s != "none" && s != "no"
}

var unsafeNameRe = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]+`)

func cleanName(s string) string {
	s = strings.TrimSpace(unsafeNameRe.ReplaceAllString(s, ""))
	return strings.Join(strings.Fields(s), " ")
}

func firstExisting(vals ...any) any {
	for _, v := range vals {
		if str(v) != "" {
			return v
		}
	}
	return nil
}

func extFormat(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	p := raw
	if err == nil {
		p = u.Path
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(p)), ".")
	if ext == "m3u8" || strings.Contains(strings.ToLower(raw), ".m3u8") {
		return "m3u8"
	}
	if ext == "" || len(ext) > 8 {
		return "mp4"
	}
	return ext
}

func normalizeMediaURL(raw string) string {
	s := strings.TrimSpace(strings.Trim(raw, "\"'"))
	if strings.HasPrefix(s, "//") {
		s = "https:" + s
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	return ""
}

func urlsFromAny(v any) []string {
	seen := map[string]bool{}
	var out []string
	var add func(any)
	add = func(x any) {
		switch t := x.(type) {
		case string:
			if u := normalizeMediaURL(t); u != "" && !seen[strings.ToLower(u)] {
				seen[strings.ToLower(u)] = true
				out = append(out, u)
			}
		case []any:
			for _, item := range t {
				add(item)
			}
		case map[string]any:
			for _, key := range []string{"url", "urls", "playUrl", "videoUrl", "videoURL", "mainUrl", "cdnUrl", "fileUrl", "resourceUrl", "lbpVideoAddress", "videoAddress"} {
				if val, ok := t[key]; ok {
					add(val)
				}
			}
		}
	}
	add(v)
	return out
}

func appendUnique(dst []string, vals ...string) []string {
	seen := map[string]bool{}
	for _, v := range dst {
		seen[strings.ToLower(v)] = true
	}
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" && !seen[strings.ToLower(v)] {
			seen[strings.ToLower(v)] = true
			dst = append(dst, v)
		}
	}
	return dst
}

func collectMaps(v any) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			out = append(out, t)
			for _, child := range t {
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		}
	}
	walk(v)
	return out
}
