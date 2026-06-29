package baijiayunxiao

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

var (
	jsonpRe              = regexp.MustCompile(`(?s)^[\w.$-]+\((.*)\)\s*;?\s*$`)
	materialURLRe        = regexp.MustCompile(`(?i)https?://[^\s"'<>]+?\.(?:pdf|pptx?|docx?|xlsx?|zip|rar|7z|txt|csv|caj)(?:[^\s"'<>]*)?`)
	materialPreviewImgRe = regexp.MustCompile(`(?i)https?://[^\s"'<>]+?\.(?:png|jpe?g|webp)(?:[^\s"'<>]*)?`)
	materialPrefixRe     = regexp.MustCompile(`^\(([\d.]+)\)--`)
	videoPrefixRe        = regexp.MustCompile(`^\[([\d.]+)\]--`)
	bjcloudPrefixRe      = regexp.MustCompile(`(?i)^bjcloudvod://`)
)

var materialExts = map[string]bool{
	".pdf": true, ".ppt": true, ".pptx": true, ".doc": true, ".docx": true,
	".xls": true, ".xlsx": true, ".zip": true, ".rar": true, ".7z": true,
	".txt": true, ".csv": true, ".caj": true,
}

type materialInfo struct {
	Name     string
	URL      string
	FID      string
	Format   string
	PageURLs []string
	Raw      map[string]any
}

type playbackResources struct {
	Title                 string
	VideoURL              string
	MP4URL                string
	AudioURL              string
	DocURL                string
	PackageURL            string
	SignalChatFileInfoURL string
	SignalChatURLs        []string
	SignalUserURL         string
	SignalAllURL          string
	SignalCommandURL      string
	Source                map[string]any
}

func baijiayunxiaoHeaders(jar http.CookieJar, rawURL string) map[string]string {
	referer := refererFromRawURL(rawURL)
	if strings.Contains(referer, "baijiayunxiao") && !strings.HasSuffix(referer, "/") {
		referer += "/"
	}
	headers := map[string]string{
		"Referer":    referer,
		"Origin":     headerOrigin(referer),
		"Accept":     "application/json, text/plain, */*",
		"User-Agent": util.RandomUA(),
	}
	if cookie := baijiayunCookieHeader(jar, rawURL); cookie != "" {
		headers["cookie"] = cookie
		headers["Cookie"] = cookie
		if token := baijiayunStudentToken(cookie); token != "" {
			auth := "Bearer " + token
			headers["authorization"] = auth
			headers["Authorization"] = auth
		}
	}
	return headers
}

func baijiayunStudentToken(cookieHeader string) string {
	for _, part := range strings.Split(cookieHeader, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "studentToken") {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func baijiayunCookieHeader(jar http.CookieJar, rawURL string) string {
	if jar == nil {
		return ""
	}
	hosts := []string{}
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		hosts = append(hosts, u.Host)
	}
	hosts = append(hosts, "www.baijiayun.com", "api.baijiayun.com", "baijiayun.com")
	seen, parts := map[string]bool{}, []string{}
	for _, host := range hosts {
		for _, ck := range jar.Cookies(&url.URL{Scheme: "https", Host: host, Path: "/"}) {
			if ck.Value == "" || seen[ck.Name] {
				continue
			}
			seen[ck.Name] = true
			parts = append(parts, ck.Name+"="+ck.Value)
		}
	}
	return strings.Join(parts, "; ")
}

func headerOrigin(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return urlHome
}

func decodeJSONMap(text string) map[string]any {
	payload := parseJSONPayload(text)
	if m, ok := payload.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func parseJSONPayload(text string) any {
	text = strings.TrimSpace(html.UnescapeString(text))
	if text == "" {
		return map[string]any{}
	}
	var v any
	if err := json.Unmarshal([]byte(text), &v); err == nil {
		return v
	}
	if m := jsonpRe.FindStringSubmatch(text); m != nil {
		if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &v); err == nil {
			return v
		}
	}
	return text
}

func valueMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func valueList(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case []map[string]any:
		out := make([]any, 0, len(x))
		for _, m := range x {
			out = append(out, m)
		}
		return out
	}
	return nil
}

func recordsValue(v any) []map[string]any {
	switch x := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, item := range x {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case []map[string]any:
		return x
	case map[string]any:
		return []map[string]any{x}
	}
	return nil
}

func collectLessonRefs(values ...any) []lessonRef {
	seen := map[string]bool{}
	out := []lessonRef{}
	var walk func(any, []string)
	walk = func(v any, prefix []string) {
		switch x := v.(type) {
		case []any:
			for i, item := range x {
				if m, ok := item.(map[string]any); ok {
					title := firstNonEmpty(anyString(m["periods_title"]), anyString(m["title"]), anyString(m["name"]), fmt.Sprintf("课时%d", i+1))
					nextPrefix := append(append([]string{}, prefix...), title)
					id := firstNonEmpty(anyString(m["id"]), anyString(m["video_id"]), anyString(m["videoId"]), anyString(m["room_id"]), anyString(m["roomId"]), anyString(m["classid"]))
					if id != "" && !seen[id] {
						seen[id] = true
						out = append(out, lessonRef{ID: id, Title: strings.Join(nextPrefix, " - "), Payload: m})
					}
					for _, key := range []string{"child", "children", "periods", "chapter", "chapter_list", "chapterList", "period_list", "periodList", "list"} {
						if child, ok := m[key]; ok {
							walk(child, nextPrefix)
						}
					}
				}
			}
		case []map[string]any:
			for _, m := range x {
				walk([]any{m}, prefix)
			}
		case map[string]any:
			walk([]any{x}, prefix)
		}
	}
	for _, v := range values {
		walk(v, nil)
	}
	return out
}

func extractMaterials(payload any, fallbackName string) []materialInfo {
	queue := []any{}
	if payload != nil {
		queue = append(queue, payload)
	}
	visitedMaps := map[*map[string]any]bool{}
	seen := map[string]bool{}
	var out []materialInfo
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		switch x := cur.(type) {
		case map[string]any:
			ptr := &x
			if visitedMaps[ptr] {
				continue
			}
			visitedMaps[ptr] = true
			appendMaterialFromMap(&out, seen, x, fallbackName)
			for _, child := range x {
				switch child.(type) {
				case map[string]any, []any, []map[string]any, string:
					queue = append(queue, child)
				}
			}
		case []any:
			for _, child := range x {
				switch child.(type) {
				case map[string]any, []any, string:
					queue = append(queue, child)
				}
			}
		case []map[string]any:
			for _, child := range x {
				queue = append(queue, child)
			}
		case string:
			for _, m := range materialsFromText(x, fallbackName) {
				addMaterial(&out, seen, m)
			}
		}
	}
	return out
}

func appendMaterialFromMap(out *[]materialInfo, seen map[string]bool, m map[string]any, fallbackName string) {
	name := firstNonEmpty(anyString(m["itemName"]), anyString(m["materialName"]), anyString(m["resourceName"]), anyString(m["fileName"]), anyString(m["file_name"]), anyString(m["show_name"]), anyString(m["originName"]), anyString(m["originalName"]), anyString(m["name"]), anyString(m["title"]), fallbackName, "资料")
	rawURL := normalizeMediaURL(firstNonEmpty(anyString(m["downloadUrl"]), anyString(m["download_url"]), anyString(m["fileUrl"]), anyString(m["file_url"]), anyString(m["resourceUrl"]), anyString(m["resource_url"]), anyString(m["attachmentUrl"]), anyString(m["attachment_url"]), anyString(m["ossUrl"]), anyString(m["oss_url"]), anyString(m["url"]), anyString(m["href"]), anyString(m["src"]), anyString(m["itemUrl"])))
	fileType := firstNonEmpty(anyString(m["fileType"]), anyString(m["type"]), anyString(m["contentType"]), anyString(m["ext"]), anyString(m["suffix"]), anyString(m["mimeType"]), anyString(m["mime_type"]))
	ext := guessMaterialExt(rawURL, fileType, name)
	fid := firstNonEmpty(anyString(m["fid"]), anyString(m["fileId"]), anyString(m["file_id"]))
	pageURLs := extractPageURLs(firstExisting(m, "page_list", "pageList", "pages"))
	if len(pageURLs) > 0 && (ext == "" || ext == ".pdf" || ext == ".ppt" || ext == ".pptx") {
		addMaterial(out, seen, materialInfo{Name: normalizeMaterialName(name, fallbackName), Format: ".pdf", PageURLs: pageURLs, Raw: m})
	}
	if rawURL != "" && materialExts[ext] && !isMediaExt(ext) {
		addMaterial(out, seen, materialInfo{Name: normalizeMaterialName(firstNonEmpty(safeBasename(rawURL), name), fallbackName), URL: rawURL, Format: ext, Raw: m})
	}
	if fid != "" && materialExts[ext] && !isMediaExt(ext) {
		addMaterial(out, seen, materialInfo{Name: normalizeMaterialName(name, fallbackName), FID: fid, Format: ext, Raw: m})
	}
}

func firstExisting(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func addMaterial(out *[]materialInfo, seen map[string]bool, m materialInfo) {
	key := materialKey(m)
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	*out = append(*out, m)
}

func materialKey(m materialInfo) string {
	switch {
	case m.FID != "":
		return "fid:" + m.FID
	case m.URL != "":
		return "url:" + m.URL
	case len(m.PageURLs) > 0:
		return "pages:" + strings.Join(m.PageURLs, "|")
	default:
		return "name:" + strings.ToLower(m.Name) + ":" + m.Format
	}
}

func materialsFromText(text string, fallbackName string) []materialInfo {
	text = html.UnescapeString(text)
	var out []materialInfo
	seen := map[string]bool{}
	for i, raw := range materialURLRe.FindAllString(text, -1) {
		u := normalizeMediaURL(raw)
		ext := guessMaterialExt(u, "", "")
		if !materialExts[ext] {
			continue
		}
		name := normalizeMaterialName(safeBasename(u), fmt.Sprintf("%s_资料_%d", firstNonEmpty(fallbackName, "资料"), i+1))
		addMaterial(&out, seen, materialInfo{Name: name, URL: u, Format: ext})
	}
	return out
}

func appendMaterialEntries(c *util.Client, domain string, entries *[]*extractor.MediaInfo, seen map[string]bool, materials []materialInfo, headers map[string]string) {
	for _, material := range materials {
		material = resolveCoursewareMaterial(c, domain, material, headers)
		if key := materialKey(material); key == "" || seen[key] {
			continue
		} else {
			seen[key] = true
		}
		entry := materialEntry(domain, material, headers)
		if entry != nil {
			*entries = append(*entries, entry)
		}
	}
}

func materialEntry(domain string, material materialInfo, headers map[string]string) *extractor.MediaInfo {
	name := util.SanitizeFilename(normalizeMaterialName(material.Name, "资料"))
	format := strings.TrimPrefix(firstNonEmpty(material.Format, guessMaterialExt(material.URL, "", name), "bin"), ".")
	extra := map[string]any{"kind": "material"}
	if material.FID != "" {
		extra["fid"] = material.FID
	}
	if len(material.PageURLs) > 0 {
		extra["page_urls"] = material.PageURLs
		extra["domain"] = domain
		data, _ := json.MarshalIndent(map[string]any{"title": name, "pages": material.PageURLs}, "", "  ")
		return &extractor.MediaInfo{Site: "baijiayunxiao", Title: name, Streams: map[string]extractor.Stream{"pages": {Quality: "pages", URLs: []string{"data:application/json;base64," + base64.StdEncoding.EncodeToString(data)}, Format: "json", Headers: headers}}, Extra: extra}
	}
	if material.URL == "" {
		return nil
	}
	return &extractor.MediaInfo{Site: "baijiayunxiao", Title: name, Streams: map[string]extractor.Stream{"file": {Quality: "source", URLs: []string{material.URL}, Format: format, Headers: headers}}, Extra: extra}
}

func resolveCoursewareMaterial(c *util.Client, domain string, material materialInfo, headers map[string]string) materialInfo {
	if material.FID == "" || domain == "" {
		return material
	}
	body, err := c.GetString(fmt.Sprintf(urlCourseWarePreview, domain, url.QueryEscape(material.FID)), headers)
	if err != nil {
		return material
	}
	payload := parseJSONPayload(body)
	var previewURL string
	if m := valueMap(payload); len(m) > 0 {
		previewURL = normalizeMediaURL(anyString(m["data"]))
		if previewURL == "" {
			previewURL = findURLWithExt(m)
		}
	} else if s, ok := payload.(string); ok {
		previewURL = normalizeMediaURL(s)
	}
	if previewURL == "" {
		return material
	}
	ext := guessMaterialExt(previewURL, "", material.Name)
	if materialExts[ext] && !strings.Contains(strings.ToLower(previewURL), "docpreview") {
		material.URL = previewURL
		material.Format = ext
		return material
	}
	if htmlText, err := c.GetString(previewURL, headers); err == nil {
		pages := extractPreviewPageURLs(htmlText)
		if len(pages) > 0 {
			material.PageURLs = pages
			material.Format = ".pdf"
			return material
		}
	}
	material.URL = previewURL
	if material.Format == "" && ext != "" {
		material.Format = ext
	}
	return material
}

func fetchDocMaterials(c *util.Client, docURL, fallbackName string, headers map[string]string) []materialInfo {
	body, err := c.GetString(normalizeMediaURL(docURL), headers)
	if err != nil {
		return nil
	}
	payload := parseJSONPayload(body)
	var out []materialInfo
	seen := map[string]bool{}
	for _, rec := range recordsValue(payload) {
		ext := strings.ToLower(firstNonEmpty(anyString(rec["ext"]), guessMaterialExt(anyString(rec["url"]), anyString(rec["fileType"]), anyString(rec["name"]))))
		if ext != ".pptx" && ext != ".ppt" && ext != ".pdf" {
			continue
		}
		pages := extractPageURLs(firstExisting(rec, "page_list", "pageList", "pages"))
		if len(pages) == 0 {
			continue
		}
		name := normalizeMaterialName(firstNonEmpty(anyString(rec["name"]), anyString(rec["title"]), fallbackName), fallbackName)
		addMaterial(&out, seen, materialInfo{Name: name, Format: ".pdf", PageURLs: pages, Raw: rec})
	}
	if len(out) > 0 {
		return out
	}
	return extractMaterials(payload, fallbackName)
}

func resolvePlaybackDetailed(c *util.Client, p playbackParams, headers map[string]string) (playbackResources, error) {
	var apiURL string
	if p.isVOD || p.vid != "" {
		apiURL = fmt.Sprintf(urlGetPlayURL, url.QueryEscape(firstNonEmpty(p.vid, p.roomID)), url.QueryEscape(p.token))
	} else {
		apiURL = fmt.Sprintf(urlGetPlayInfo, url.QueryEscape(p.roomID), url.QueryEscape(p.token))
	}
	body, err := c.GetString(apiURL, headers)
	if err != nil {
		return playbackResources{}, err
	}
	payload := decodeJSONMap(body)
	data := valueMap(payload["data"])
	res := playbackResources{
		Title:                 firstNonEmpty(anyString(data["playback_title"]), anyString(valueMap(data["video_info"])["title"]), anyString(valueMap(data["class_data"])["title"]), anyString(data["title"])),
		AudioURL:              normalizeMediaURL(anyString(data["audio_url"])),
		PackageURL:            normalizeMediaURL(anyString(valueMap(data["package_signal"])["package_url"])),
		SignalChatFileInfoURL: normalizeMediaURL(anyString(valueMap(valueMap(data["signal"])["chatFileInfo"])["url"])),
		SignalUserURL:         normalizeMediaURL(anyString(valueMap(valueMap(data["signal"])["user"])["url"])),
		SignalAllURL:          normalizeMediaURL(anyString(valueMap(valueMap(data["signal"])["all"])["url"])),
		SignalCommandURL:      normalizeMediaURL(anyString(valueMap(valueMap(data["signal"])["command"])["url"])),
		DocURL:                normalizeMediaURL(anyString(valueMap(valueMap(data["signal"])["doc"])["url"])),
		Source:                data,
	}
	for _, rec := range recordsValue(valueMap(data["signal"])["chat"]) {
		if u := normalizeMediaURL(anyString(rec["url"])); u != "" {
			res.SignalChatURLs = append(res.SignalChatURLs, u)
		}
	}
	if res.DocURL == "" {
		res.DocURL = firstHTTPMaterialFeed(data, "doc")
	}
	playInfo := valueMap(data["play_info"])
	if len(playInfo) == 0 {
		playInfo = data
	}
	res.VideoURL, res.MP4URL = extractPlayURLs(playInfo)
	if res.VideoURL == "" {
		res.VideoURL = normalizeMediaURL(firstNonEmpty(anyString(data["playback_url"]), anyString(data["video_url"]), anyString(data["play_url"]), anyString(data["url"])))
	}
	if res.MP4URL == "" && strings.Contains(strings.ToLower(res.VideoURL), ".mp4") {
		res.MP4URL = res.VideoURL
	}
	if res.VideoURL == "" {
		return res, fmt.Errorf("baijiayunxiao playback: no playable URL in detailed response")
	}
	return res, nil
}

func extractPlayURLs(playInfo map[string]any) (string, string) {
	candidates := []map[string]any{}
	for _, rec := range recordsValue(playInfo["video"]) {
		candidates = append(candidates, rec)
	}
	if len(candidates) == 0 {
		for _, v := range playInfo {
			if m := valueMap(v); len(m) > 0 {
				candidates = append(candidates, m)
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return numericValue(candidates[i]["size"]) > numericValue(candidates[j]["size"])
	})
	var mp4URL, evURL, otherURL string
	for _, rec := range candidates {
		u := normalizeMediaURL(firstNonEmpty(anyString(rec["url"]), anyString(rec["play_url"]), anyString(rec["video_url"]), anyString(rec["cdn_url"])))
		if u == "" {
			u = normalizeMediaURL(firstNonEmpty(anyString(rec["enc_url"]), anyString(rec["encUrl"])))
		}
		if u == "" {
			continue
		}
		ext := strings.ToLower(path.Ext(pathFromURL(u)))
		switch ext {
		case ".mp4":
			if mp4URL == "" {
				mp4URL = u
			}
		case ".ev1", ".ev2":
			if evURL == "" {
				evURL = u
			}
		default:
			if otherURL == "" {
				otherURL = u
			}
		}
	}
	return firstNonEmpty(mp4URL, evURL, otherURL), mp4URL
}

func mergeExtra(info *extractor.MediaInfo, extra map[string]any) {
	if info.Extra == nil {
		info.Extra = map[string]any{}
	}
	for k, v := range extra {
		info.Extra[k] = v
	}
}

func firstNonNil(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

func priceYuan(v any) any {
	s := strings.ReplaceAll(anyString(v), ",", "")
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return v
	}
	if f > 100 {
		return f / 100
	}
	return f
}

func guessMaterialExt(itemURL, fileType, itemName string) string {
	for _, s := range []string{itemURL, itemName} {
		if p := pathFromURL(s); p != "" {
			ext := strings.ToLower(path.Ext(p))
			if materialExts[ext] {
				return ext
			}
		}
	}
	ft := strings.ToLower(strings.TrimSpace(fileType))
	for _, pair := range [][2]string{{"application/pdf", ".pdf"}, {"pdf", ".pdf"}, {"pptx", ".pptx"}, {"ppt", ".ppt"}, {"docx", ".docx"}, {"doc", ".doc"}, {"xlsx", ".xlsx"}, {"xls", ".xls"}, {"zip", ".zip"}, {"rar", ".rar"}, {"7z", ".7z"}, {"txt", ".txt"}, {"csv", ".csv"}, {"caj", ".caj"}} {
		if strings.Contains(ft, pair[0]) {
			return pair[1]
		}
	}
	return ""
}

func safeBasename(raw string) string {
	p := pathFromURL(raw)
	if p == "" {
		return ""
	}
	base, err := url.PathUnescape(path.Base(p))
	if err != nil {
		base = path.Base(p)
	}
	return strings.TrimSpace(base)
}

func pathFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		return u.Path
	}
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	return raw
}

func normalizeMaterialName(name, fallback string) string {
	name = materialPrefixRe.ReplaceAllString(strings.TrimSpace(firstNonEmpty(name, fallback, "资料")), "")
	name = regexp.MustCompile(`[\r\n\t]+`).ReplaceAllString(name, " ")
	name = util.SanitizeFilename(strings.TrimSpace(name))
	if name == "" {
		return "资料"
	}
	return name
}

func extractVideoIndexPrefix(title string) string {
	if m := videoPrefixRe.FindStringSubmatch(strings.TrimSpace(title)); m != nil {
		return m[1]
	}
	return ""
}

func extractPageURLs(v any) []string {
	seen := map[string]bool{}
	var out []string
	for _, rec := range recordsValue(v) {
		if u := normalizeMediaURL(anyString(rec["url"])); u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	if len(out) == 0 {
		for _, item := range valueList(v) {
			if s, ok := item.(string); ok {
				if u := normalizeMediaURL(s); u != "" && !seen[u] {
					seen[u] = true
					out = append(out, u)
				}
			}
		}
	}
	return out
}

func extractPreviewPageURLs(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range materialPreviewImgRe.FindAllString(html.UnescapeString(text), -1) {
		u := normalizeMediaURL(raw)
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

func firstHTTPMaterialFeed(v any, hint string) string {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if strings.Contains(strings.ToLower(k), strings.ToLower(hint)) {
				if u := normalizeMediaURL(anyString(child)); strings.HasPrefix(u, "http") {
					return u
				}
				if m := valueMap(child); len(m) > 0 {
					if u := normalizeMediaURL(anyString(m["url"])); strings.HasPrefix(u, "http") {
						return u
					}
				}
			}
		}
		for _, child := range x {
			if u := firstHTTPMaterialFeed(child, hint); u != "" {
				return u
			}
		}
	case []any:
		for _, child := range x {
			if u := firstHTTPMaterialFeed(child, hint); u != "" {
				return u
			}
		}
	}
	return ""
}

func findURLWithExt(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for _, key := range []string{"downloadUrl", "download_url", "fileUrl", "file_url", "resourceUrl", "url", "href", "src"} {
			u := normalizeMediaURL(anyString(x[key]))
			if materialExts[guessMaterialExt(u, "", "")] {
				return u
			}
		}
		for _, child := range x {
			if u := findURLWithExt(child); u != "" {
				return u
			}
		}
	case []any:
		for _, child := range x {
			if u := findURLWithExt(child); u != "" {
				return u
			}
		}
	}
	return ""
}

func isMediaExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".mp4", ".m3u8", ".mp3", ".m4a", ".aac", ".wav", ".flv", ".ev1", ".ev2":
		return true
	}
	return false
}

func numericValue(v any) float64 {
	s := strings.ReplaceAll(anyString(v), ",", "")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func decodeBjcloudvod(encoded string) string {
	if !bjcloudPrefixRe.MatchString(encoded) {
		return ""
	}
	payload := bjcloudPrefixRe.ReplaceAllString(encoded, "")
	payload = strings.NewReplacer("-", "+", "_", "/").Replace(payload)
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(decoded) == 0 {
		return ""
	}
	shift := int(decoded[0] % 8)
	decoded = decoded[1:]
	out := make([]byte, len(decoded))
	for i, b := range decoded {
		out[i] = b ^ byte((shift+i)%8)
	}
	return strings.TrimSpace(string(out))
}
