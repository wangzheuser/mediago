package ledu

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sophomoresty/mediago/internal/util"
)

var leduM3U8KeyURIRe = regexp.MustCompile(`URI="([^"]+)"`)

func anyMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func firstTextFromMaps(v any, keys ...string) string {
	for _, node := range nestedMaps(v) {
		for _, key := range keys {
			if s := firstText(node[key]); s != "" {
				return s
			}
		}
	}
	return ""
}

func leduGetJSONURL(c *util.Client, rawURL string, headers map[string]string) (any, error) {
	body, err := c.GetString(rawURL, leduMediaHeaders(headers))
	if err != nil {
		return nil, err
	}
	payload, err := leduParseJSON([]byte(body))
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func prepareLeduM3U8(c *util.Client, m3u8URL string, node map[string]any, headers map[string]string) (string, string, bool) {
	resolved := resolveLeduM3U8URL(c, m3u8URL, headers)
	body, err := c.GetString(resolved, leduMediaHeaders(headers))
	if err != nil || !strings.Contains(body, "#EXTM3U") {
		return "", "", false
	}
	rewritten := rewriteLeduM3U8(resolved, body, firstText(node["encKey"], node["key"], node["hlsKey"]), firstText(node["encIv"], node["iv"], node["IV"]))
	return leduM3U8DataURL(rewritten), rewritten, true
}

func resolveLeduM3U8URL(c *util.Client, m3u8URL string, headers map[string]string) string {
	if !strings.Contains(strings.ToLower(m3u8URL), ".m3u8") {
		return m3u8URL
	}
	payload, err := leduGetJSON(c, cloudlearnHost, previewSourcePath, map[string]string{"m3u8Url": m3u8URL}, headers)
	if err != nil {
		return m3u8URL
	}
	if u := firstTextFromMaps(payload, "url", "m3u8Url", "videoUrl", "mp4Url"); u != "" {
		return normalizeLeduURL(u)
	}
	_ = c
	return m3u8URL
}

func applyLeduNodeContext(headers map[string]string, node map[string]any) {
	set := func(k string, vals ...any) {
		if v := firstText(vals...); v != "" {
			headers[k] = v
		}
	}
	set("stdSubject", node["stdSubject"], node["subjectId"], node["pcStdSubject"])
	set("stdGrade", node["stdGrade"], node["gradeId"])
	set("stdCourseId", node["stdCourseId"], node["pcStdCourseId"], node["courseId"], node["classCourseId"])
	set("stdClassId", node["stdClassId"], node["classId"], node["class_id"])
	set("branchId", node["branchId"], node["areaId"])
	set("liveId", node["liveId"], node["live_id"], node["chapterId"], node["chapter_id"])
	set("liveType", node["liveTypeString"], node["liveType"], node["taskTypeString"], node["task_type_string"])
	set("lecturerId", node["lecturerId"], node["lecturer_id"], node["teacherId"])
	set("tutorId", node["tutorId"], node["tutor_id"])
}

func resolveLeduMaterialURL(c *util.Client, headers map[string]string, material map[string]any) string {
	if u := normalizeLeduURL(firstText(material["itemUrl"], material["fileUrl"], material["url"], material["downloadUrl"], material["resourceUrl"], material["attachmentUrl"], material["highLightItemUrl"])); u != "" {
		return u
	}
	if noteID := firstText(material["noteId"], material["note_id"]); noteID != "" {
		if payload, err := leduGetJSON(c, courseAPIHost, "/course/v1/note/"+url.PathEscape(noteID), nil, headers); err == nil {
			if u := normalizeLeduURL(firstTextFromMaps(payload, "url", "fileUrl", "downloadUrl", "resourceUrl", "attachmentUrl", "pdfUrl")); u != "" {
				return u
			}
		}
	}
	if paperID := firstText(material["paperId"], material["paper_id"]); paperID != "" {
		if payload, err := leduPostJSON(c, courseAPIHost, coursePaperLinkPath, map[string]any{"paperId": paperID}, headers); err == nil {
			if u := normalizeLeduURL(firstTextFromMaps(payload, "pdfUrl", "url", "fileUrl", "downloadUrl", "resourceUrl")); u != "" {
				return u
			}
		}
		if payload, err := leduGetJSON(c, cloudlearnHost, handoutPDFPath, map[string]string{"paperId": paperID}, headers); err == nil {
			if u := normalizeLeduURL(firstTextFromMaps(payload, "pdfUrl", "url", "fileUrl", "downloadUrl", "resourceUrl")); u != "" {
				return u
			}
		}
	}
	return ""
}

func normalizeLeduURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "data:") {
		return raw
	}
	return ""
}

func cloneAnyMap(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func firstNonNil(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func valuesFromAny(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, it := range x {
			if s := firstText(it); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := firstText(x); s != "" {
			return []string{s}
		}
		return nil
	}
}

func uniqueLeduTexts(values []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func pickIndex(values []string, index int) string {
	if len(values) == 0 {
		return ""
	}
	if index >= 0 && index < len(values) {
		return values[index]
	}
	return values[0]
}

func rewriteLeduM3U8(m3u8URL, m3u8Text, encKey, encIV string) string {
	keyBytes := leduProcessKeyOrIV(encKey)
	ivBytes := leduProcessKeyOrIV(encIV)
	keyLineSeen := false
	out := make([]string, 0, strings.Count(m3u8Text, "\n")+2)

	for _, raw := range strings.Split(m3u8Text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-KEY:") {
			keyLineSeen = true
			if len(keyBytes) > 0 {
				line = `#EXT-X-KEY:METHOD=AES-128,URI="` + leduKeyDataURL(keyBytes) + `"`
				if len(ivBytes) > 0 {
					line += ",IV=0x" + hex.EncodeToString(ivBytes)
				}
				out = append(out, line)
				continue
			}
			out = append(out, leduM3U8KeyURIRe.ReplaceAllStringFunc(line, func(match string) string {
				parts := leduM3U8KeyURIRe.FindStringSubmatch(match)
				if len(parts) != 2 {
					return match
				}
				return `URI="` + resolveLeduM3U8Line(parts[1], m3u8URL) + `"`
			}))
			continue
		}
		if !strings.HasPrefix(line, "#") {
			line = resolveLeduM3U8Line(line, m3u8URL)
		}
		out = append(out, line)
	}

	if len(keyBytes) > 0 && !keyLineSeen {
		keyLine := `#EXT-X-KEY:METHOD=AES-128,URI="` + leduKeyDataURL(keyBytes) + `"`
		if len(ivBytes) > 0 {
			keyLine += ",IV=0x" + hex.EncodeToString(ivBytes)
		}
		insertAt := 0
		if len(out) > 0 && strings.HasPrefix(out[0], "#EXTM3U") {
			insertAt = 1
		}
		out = append(out, "")
		copy(out[insertAt+1:], out[insertAt:])
		out[insertAt] = keyLine
	}
	return strings.Join(out, "\n") + "\n"
}

func resolveLeduM3U8Line(raw, baseURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "data:") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return raw
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return base.ResolveReference(ref).String()
}

func leduProcessKeyOrIV(value string) []byte {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X"))
	if value == "" {
		return nil
	}
	if len(value) > 16 && strings.HasPrefix(value, "1") {
		return leduFit16([]byte(leduSwapPairs(value[1:])))
	}
	if len(value)%2 == 0 {
		if decoded, err := hex.DecodeString(value); err == nil {
			return leduFit16(decoded)
		}
	}
	swapped := leduSwapPairs(value)
	if len(swapped)%2 == 0 {
		if decoded, err := hex.DecodeString(swapped); err == nil {
			return leduFit16(decoded)
		}
	}
	return leduFit16([]byte(swapped))
}

func leduSwapPairs(value string) string {
	runes := []rune(value)
	for i := 0; i+1 < len(runes); i += 2 {
		runes[i], runes[i+1] = runes[i+1], runes[i]
	}
	return string(runes)
}

func leduFit16(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	if len(data) >= 16 {
		return data[:16]
	}
	out := make([]byte, 16)
	copy(out, data)
	return out
}

func leduKeyDataURL(key []byte) string {
	return "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(key)
}

func leduM3U8DataURL(manifest string) string {
	return "data:application/vnd.apple.mpegurl;base64," + base64.StdEncoding.EncodeToString([]byte(manifest))
}
