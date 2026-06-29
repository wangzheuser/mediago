package magedu

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

func mageduCookieString(jar http.CookieJar) string {
	hosts := []string{"edu.magedu.com", "www.magedu.com", "magedu.com"}
	seen, parts := map[string]bool{}, []string{}
	for _, h := range hosts {
		for _, ck := range jar.Cookies(&url.URL{Scheme: "https", Host: h}) {
			if ck.Value != "" && !seen[ck.Name] {
				seen[ck.Name] = true
				parts = append(parts, ck.Name+"="+ck.Value)
			}
		}
	}
	return strings.Join(parts, "; ")
}

func cookieValue(cookie, name string) string {
	for _, p := range strings.Split(cookie, ";") {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 && strings.EqualFold(kv[0], name) {
			v, _ := url.QueryUnescape(kv[1])
			return firstText(v, kv[1])
		}
	}
	return ""
}

func mageduSuccess(resp map[string]any) bool {
	code := firstText(resp["code"])
	return code == "0" || code == "200" || boolOf(resp["success"])
}

func mageduData(resp map[string]any) any {
	if data, ok := resp["data"]; ok {
		if mageduSuccess(resp) || (firstText(resp["code"]) != "401" && firstText(resp["code"]) != "403") {
			return data
		}
	}
	return map[string]any{}
}

func mageduCourseRecords(v any) []map[string]any {
	m := mapAny(v)
	for _, k := range []string{"data", "records", "list"} {
		if r := records(m[k]); len(r) > 0 {
			return r
		}
	}
	return records(v)
}

func mageduNormalizeCourse(m map[string]any) mageduCourse {
	id := firstText(m["id"], m["curriculumId"], m["courseId"], m["cuId"])
	title := firstText(m["title"], m["name"], m["courseName"])
	if id == "" || title == "" {
		return mageduCourse{}
	}
	owner := m["owner"]
	if owner == nil {
		owner = m["purchased"]
	}
	if owner == nil {
		owner = m["isBuy"]
	}
	return mageduCourse{ID: id, Title: title, Price: m["price"], Purchased: boolOf(owner)}
}

func mageduHidden(m map[string]any) bool {
	v := firstText(m["isHide"])
	return v != "" && v != "0" && !strings.EqualFold(v, "false") && !strings.EqualFold(v, "none")
}

func sortMagedu(items []map[string]any) {
	sort.SliceStable(items, func(i, j int) bool {
		return firstInt(items[i]["seqId"], items[i]["sort"], items[i]["id"]) < firstInt(items[j]["seqId"], items[j]["sort"], items[j]["id"])
	})
}

func mageduVideoItem(sec map[string]any, prefix []int) mageduItem {
	raw := firstText(sec["content"], sec["videoId"], sec["vid"])
	if raw == "" {
		return mageduItem{}
	}
	title := mageduSectionTitle(sec, raw)
	sectionID := firstText(sec["id"], sec["sectionId"])
	storageID := firstText(sec["videoStorageId"], sec["video_storage_id"])
	size := int64(numOf(sec["size"]))
	if mageduLooksLikeVideoURL(raw) {
		u := normalizeURL(raw)
		return mageduItem{Kind: "video", Title: indexedTitle(prefix, title), FileURL: u, FileFmt: mediaExt(u), SectionID: sectionID, StorageID: storageID, Size: size}
	}
	if !mageduLooksLikePolyvID(raw) {
		return mageduItem{}
	}
	return mageduItem{Kind: "video", Title: indexedTitle(prefix, title), VideoID: strings.TrimSpace(raw), SectionID: sectionID, StorageID: storageID, Size: size}
}

func mageduInlineFile(sec map[string]any, prefix []int) mageduItem {
	if firstText(sec["sectionType"]) != "2" {
		return mageduItem{}
	}
	u := normalizeURL(firstText(sec["content"]))
	if !mageduLooksLikeDownloadURL(u) {
		return mageduItem{}
	}
	title := indexedFileTitle(prefix, mageduSectionTitle(sec, "课件"))
	return mageduItem{Kind: "file", Title: title, FileURL: u, FileFmt: mediaExt(u), Size: int64(numOf(sec["size"]))}
}

func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	if strings.HasPrefix(u, "/") {
		return strings.TrimRight(urlOrigin, "/") + u
	}
	return u
}

func records(v any) []map[string]any {
	switch x := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, it := range x {
			if m, ok := it.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{x}
	}
	return nil
}

func cloneHeaders(h map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range h {
		out[k] = v
	}
	return out
}
func mapAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
func boolOf(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "1" || strings.EqualFold(x, "true")
	case float64:
		return x != 0
	case int:
		return x != 0
	}
	return false
}
func intOf(v any) int { return int(numOf(v)) }
func numOf(v any) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(firstText(v), ",", ""), 64)
	return f
}
func firstInt(vals ...any) int {
	for _, v := range vals {
		if n := intOf(v); n != 0 {
			return n
		}
	}
	return 0
}
func firstText(vals ...any) string {
	for _, v := range vals {
		if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}
func indexedTitle(prefix []int, title string) string {
	parts := make([]string, len(prefix))
	for i, n := range prefix {
		parts[i] = strconv.Itoa(n)
	}
	if title == "" {
		title = "video"
	}
	return fmt.Sprintf("[%s]--%s", strings.Join(parts, "."), title)
}
func indexedFileTitle(prefix []int, title string) string {
	parts := make([]string, len(prefix))
	for i, n := range prefix {
		parts[i] = strconv.Itoa(n)
	}
	if title == "" {
		title = "资料"
	}
	return fmt.Sprintf("(%s)--%s", strings.Join(parts, "."), title)
}
func mediaExt(u string) string {
	lu := strings.ToLower(u)
	if strings.Contains(lu, ".m3u8") {
		return "m3u8"
	}
	if strings.Contains(lu, ".mp3") || strings.Contains(lu, ".m4a") {
		return "mp3"
	}
	if strings.Contains(lu, ".pdf") {
		return "pdf"
	}
	if strings.Contains(lu, ".ppt") {
		return "ppt"
	}
	if strings.Contains(lu, ".doc") {
		return "doc"
	}
	if strings.Contains(lu, ".zip") || strings.Contains(lu, ".rar") {
		return "archive"
	}
	return "mp4"
}
