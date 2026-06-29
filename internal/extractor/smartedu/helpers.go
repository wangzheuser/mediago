package smartedu

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

func selectVideoItem(r map[string]any) map[string]any {
	list := items(r)
	flags := []string{"href-720p-m3u8", "href-m3u8", "href", "href-480p-m3u8", "href-360p-m3u8"}
	for _, f := range flags {
		for _, it := range list {
			if str(it["ti_file_flag"]) == f && isVideoFmt(it) {
				return it
			}
		}
	}
	for _, it := range list {
		if isVideoFmt(it) {
			return it
		}
	}
	return nil
}

func selectFileItem(r map[string]any) map[string]any {
	list := items(r)
	fileFmt := map[string]bool{"pdf": true, "ppt": true, "pptx": true, "doc": true, "docx": true, "xls": true, "xlsx": true, "zip": true, "rar": true, "7z": true}
	for _, f := range []string{"source", "pdf", "href"} {
		for _, it := range list {
			if str(it["ti_file_flag"]) == f && fileFmt[strings.ToLower(str(it["ti_format"]))] {
				return it
			}
		}
	}
	for _, ext := range []string{"pdf", "ppt", "pptx", "doc", "docx", "xls", "xlsx", "zip", "rar", "7z"} {
		for _, it := range list {
			if strings.ToLower(str(it["ti_format"])) == ext {
				return it
			}
		}
	}
	return nil
}

func itemURL(it map[string]any) string {
	for _, k := range []string{"ti_storage", "url", "download_url", "href"} {
		if u := str(it[k]); u != "" {
			return normalizeStorage(u)
		}
	}
	if arr, ok := it["ti_storages"].([]any); ok {
		for _, v := range arr {
			if m, ok := v.(map[string]any); ok {
				if u := str(m["ti_storage"]); u != "" {
					return normalizeStorage(u)
				}
			}
		}
	}
	return ""
}

func normalizeStorage(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, `\/`, `/`))
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "cs_path:${ref-path}", privateHost)
	return normalize(s, privateHost)
}

func relationResources(m map[string]any) []map[string]any {
	var out []map[string]any
	if rel, ok := m["relations"].(map[string]any); ok {
		for _, k := range []string{"national_course_resource", "tch_materials", "basic_works", "prepare_lessons", "elite_lessons"} {
			out = append(out, mapsFromAny(rel[k])...)
		}
	}
	return out
}
func collectResourceMaps(v any) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			if len(items(t)) > 0 || str(t["id"]) != "" {
				out = append(out, t)
			}
			for _, c := range t {
				walk(c)
			}
		case []any:
			for _, c := range t {
				walk(c)
			}
		}
	}
	walk(v)
	return out
}
func mapsFromAny(v any) []map[string]any {
	var out []map[string]any
	if a, ok := v.([]any); ok {
		for _, x := range a {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}
func items(r map[string]any) []map[string]any { return mapsFromAny(r["ti_items"]) }
func isVideoFmt(it map[string]any) bool {
	f := strings.ToLower(firstNonEmpty(str(it["ti_format"]), extFormat(itemURL(it))))
	return f == "m3u8" || f == "mp4"
}
func itemSize(it map[string]any) int64 {
	var n int64
	switch v := it["ti_size"].(type) {
	case float64:
		n = int64(v)
	case json.Number:
		n, _ = v.Int64()
	}
	return n
}
func extFormat(u string) string {
	e := strings.TrimPrefix(strings.ToLower(path.Ext(strings.Split(u, "?")[0])), ".")
	return e
}
func staticBases() []string  { return []string{staticBase0, staticBase1} }
func specialBases() []string { return []string{special0, special1, special2, special3} }
func tplURLs(tpl string, bases []string, id string) []string {
	out := make([]string, 0, len(bases))
	for _, b := range bases {
		out = append(out, fmt.Sprintf(tpl, b, url.PathEscape(id)))
	}
	return out
}
func firstQuery(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			return v
		}
	}
	return ""
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
func globalTitle(m map[string]any) string {
	for _, k := range []string{"title", "name", "global_title", "globalTitle"} {
		if s := str(m[k]); s != "" {
			return s
		}
		if mm, ok := m[k].(map[string]any); ok {
			if s := str(mm["zh-CN"]); s != "" {
				return s
			}
			for _, v := range mm {
				if s := str(v); s != "" {
					return s
				}
			}
		}
	}
	return ""
}
func normalize(s, base string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	if strings.HasPrefix(s, "/") && base != "" {
		b, _ := url.Parse(base)
		u, _ := url.Parse(s)
		return b.ResolveReference(u).String()
	}
	return s
}
func isPrivate(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Host)
	return strings.Contains(h, "-private.") || strings.Contains(h, "ndr-private.")
}
func privateToPublic(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	u.Host = strings.Replace(strings.Replace(u.Host, "-private.", ".", 1), "ndr-private.", "ndr.", 1)
	return u.String()
}

func cookieHeader(jar http.CookieJar, bases []string) string {
	seen := map[string]bool{}
	var parts []string
	for _, raw := range bases {
		u, _ := url.Parse(raw)
		for _, c := range jar.Cookies(u) {
			if !seen[c.Name] {
				seen[c.Name] = true
				parts = append(parts, c.Name+"="+c.Value)
			}
		}
	}
	return strings.Join(parts, "; ")
}
func decodeAccessToken(cookie string) string {
	return decodeSmarteduAuth(cookie).accessToken
}
func decodeSmarteduAuth(cookie string) smarteduAuth {
	for _, part := range strings.Split(cookie, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 || (kv[0] != "UC_TOKEN" && !strings.HasPrefix(kv[0], "UC_TOKEN-")) {
			continue
		}
		raw := kv[1] + strings.Repeat("=", (4-len(kv[1])%4)%4)
		b, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			auth := smarteduAuth{
				accessToken:  str(m["access_token"]),
				refreshToken: str(m["refresh_token"]),
				macKey:       str(m["mac_key"]),
			}
			if d := str(m["diff"]); d != "" {
				auth.diff, _ = strconv.ParseInt(d, 10, 64)
			}
			return auth
		}
	}
	return smarteduAuth{}
}
