package ckjr

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Sophomoresty/mediago/internal/util"
)

const qcloudRSAPublicKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC3pDA7GTxOvNbXRGMi9QSIzQEI
+EMD1HcUPJSQSFuRkZkWo4VQECuPRg/xVjqwX1yUrHUvGQJsBwTS/6LIcQiSwYsO
qf+8TWxGQOJyW46gPPQVzTjNTiUoq435QB0v11lNxvKWBQIZLmacUZ2r1APta7i/
MY4Lx9XlZVMZNUdUywIDAQAB
-----END PUBLIC KEY-----`

func collectMediaCandidates(c *util.Client, data any, headers map[string]string) []mediaCandidate {
	var out []mediaCandidate
	seen := map[string]bool{}
	seenAuth := map[string]bool{}

	var appendCand func(mediaCandidate)
	appendCand = func(cand mediaCandidate) {
		cand = normalizeCandidate(cand)
		if cand.URL == "" || seen[cand.URL] {
			return
		}
		seen[cand.URL] = true
		out = append(out, cand)
	}

	var walk func(any, string)
	walk = func(v any, sourceKey string) {
		switch x := v.(type) {
		case map[string]any:
			if auth := qcloudAuth(x); auth != nil && c != nil {
				key := auth["app_id"] + "|" + auth["file_id"] + "|" + auth["psign"]
				if !seenAuth[key] {
					seenAuth[key] = true
					if cands, err := requestQCloudCandidates(c, auth, headers); err == nil {
						for _, cand := range cands {
							if cand.SourceKey == "" {
								cand.SourceKey = sourceKey
							}
							appendCand(cand)
						}
					}
				}
			}
			if u := datumAccessURL(x); u != "" {
				appendCand(mediaCandidate{URL: u, SourceKey: firstNonEmpty(sourceKey, "datum"), Kind: "file"})
			}
			if htmlText := directHTMLContent(x); htmlText != "" {
				appendCand(mediaCandidate{URL: dataURL("text/html", normalizeHTMLDocument(htmlText)), Format: "html", Kind: "file", SourceKey: firstNonEmpty(sourceKey, "content"), Extra: map[string]any{"html_content": htmlText}})
			}
			for k, vv := range x {
				key := strings.ToLower(strings.TrimSpace(k))
				if s, ok := vv.(string); ok {
					if decrypted := maybeDecryptMediaValue(key, s); decrypted != "" {
						appendTextCandidates(decrypted, key, appendCand)
					}
				}
				walk(vv, key)
			}
		case []any:
			for _, vv := range x {
				walk(vv, sourceKey)
			}
		case string:
			appendTextCandidates(x, sourceKey, appendCand)
			if nested := loadNestedJSON(x); nested != nil {
				walk(nested, sourceKey)
			}
		}
	}

	walk(data, "")
	return out
}

func appendTextCandidates(text, sourceKey string, appendCand func(mediaCandidate)) {
	for _, raw := range extractTextMediaURLs(text) {
		kind := inferMediaTypeFromKey(sourceKey)
		appendCand(mediaCandidate{URL: raw, SourceKey: sourceKey, Kind: kind})
	}
}

func normalizeCandidate(cand mediaCandidate) mediaCandidate {
	cand.URL = normalizeMediaText(cand.URL)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(cand.URL)), "#extm3u") {
		text := strings.TrimSpace(cand.URL)
		cand.URL = dataURL("application/vnd.apple.mpegurl", text)
		cand.Format = "m3u8"
		cand.Kind = "video"
		cand.NeedMerge = true
		if cand.Extra == nil {
			cand.Extra = map[string]any{}
		}
		cand.Extra["m3u8_text"] = text
	}
	if cand.URL == "" {
		return cand
	}
	if cand.Format == "" {
		cand.Format = pickCandidateFormat(cand)
	}
	if cand.Kind == "" {
		cand.Kind = firstNonEmpty(inferMediaTypeFromKey(cand.SourceKey), mediaKindFromFormat(cand.Format))
	}
	if cand.Extra == nil {
		cand.Extra = map[string]any{}
	}
	return cand
}

func extractTextMediaURLs(text string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = normalizeMediaText(s)
		if s == "" {
			return
		}
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(s)), "#extm3u") || isMediaURL(s) {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	for _, candidate := range []string{text, normalizeMediaText(text)} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		add(strings.TrimRight(candidate, `'" )],;`))
		for _, m := range mediaRe.FindAllString(candidate, -1) {
			add(strings.TrimRight(m, `'" )],;`))
		}
	}
	return out
}

func maybeDecryptMediaValue(sourceKey, value string) string {
	if strings.TrimSpace(value) == "" || strings.HasPrefix(strings.TrimSpace(value), "http") || strings.HasPrefix(strings.TrimSpace(value), "#EXTM3U") {
		return ""
	}
	key := regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(strings.ToLower(sourceKey), "")
	if key == "" {
		return ""
	}
	for _, hint := range []string{"playurl", "reviewurl", "liveurl", "videourl", "audiourl", "m3u8url", "mp3url", "m4aurl", "pdfurl", "downloadurl", "fileurl", "encode"} {
		if strings.Contains(key, hint) {
			return ckjrDecryptURL(value)
		}
	}
	return ""
}

func inferMediaTypeFromKey(sourceKey string) string {
	key := regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(strings.ToLower(sourceKey), "")
	if key == "" {
		return ""
	}
	for _, hint := range []string{"pdf", "doc", "ppt", "xls", "xlsx", "zip", "rar", "file", "attach", "download", "material", "datum"} {
		if strings.Contains(key, hint) {
			return "file"
		}
	}
	for _, hint := range []string{"audio", "voice", "sound", "mp3", "m4a", "aac", "wav"} {
		if strings.Contains(key, hint) {
			return "audio"
		}
	}
	for _, hint := range []string{"video", "play", "m3u8", "hls", "stream", "media", "source", "transcode", "mp4", "flv", "live"} {
		if strings.Contains(key, hint) {
			return "video"
		}
	}
	return ""
}

func loadNestedJSON(text string) any {
	queue := []string{strings.TrimSpace(text), normalizeMediaText(text)}
	seen := map[string]bool{}
	for len(queue) > 0 {
		s := strings.TrimSpace(queue[0])
		queue = queue[1:]
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		if !(strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") || strings.HasPrefix(s, `"`)) {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			continue
		}
		switch vv := v.(type) {
		case map[string]any, []any:
			return vv
		case string:
			queue = append(queue, vv, normalizeMediaText(vv))
		}
	}
	return nil
}

func extractHTMLContent(data any) string {
	for _, node := range walkMaps(data) {
		if htmlText := directHTMLContent(node); htmlText != "" {
			return htmlText
		}
		for _, key := range []string{"content", "detailContent", "graphicDetail", "graphicContent", "articleContent", "editorValue", "description", "intro", "introduce", "txtContent"} {
			switch vv := node[key].(type) {
			case map[string]any, []any:
				if htmlText := extractHTMLContent(vv); htmlText != "" {
					return htmlText
				}
			case string:
				if s := strings.TrimSpace(vv); s != "" {
					return normalizeHTMLDocument(s)
				}
			}
		}
	}
	return ""
}

func normalizeHTMLDocument(htmlText string) string {
	htmlText = strings.TrimSpace(htmlText)
	lower := strings.ToLower(htmlText)
	if strings.Contains(lower, "<html") || strings.Contains(lower, "<body") {
		return htmlText
	}
	for _, hint := range []string{"<div", "<p", "<img", "<span", "<table", "<br", "&nbsp;"} {
		if strings.Contains(lower, hint) {
			return "<html><body>" + htmlText + "</body></html>"
		}
	}
	return "<html><body><pre style=\"white-space: pre-wrap;\">" + html.EscapeString(htmlText) + "</pre></body></html>"
}

func dataURL(mime, content string) string {
	return "data:" + mime + ";charset=utf-8," + url.PathEscape(content)
}

func dataM3U8URL(content string) string {
	return "data:application/vnd.apple.mpegurl;base64," + base64.StdEncoding.EncodeToString([]byte(content))
}

func datumAccessURL(node map[string]any) string {
	qiniuObject := firstNonEmpty(textValue(node, "qiniuObject", "qiniu_object", "objectKey", "object_key", "fileKey"), textValue(asMap(node["datum"]), "qiniuObject", "qiniu_object"))
	if qiniuObject == "" {
		return ""
	}
	base := firstNonEmpty(textValue(node, "accessUrl", "accessURL", "downloadHost", "host"), "https://qnoss.ckjr001.com")
	return ckjrJoinURL(strings.TrimRight(base, "/")+"/", strings.TrimLeft(qiniuObject, "/"))
}

func requestQCloudCandidates(c *util.Client, auth map[string]string, headers map[string]string) ([]mediaCandidate, error) {
	apiURL := fmt.Sprintf(url1, url.PathEscape(auth["app_id"]), url.PathEscape(auth["file_id"]))
	overlayKey := genOverlay()
	overlayIV := genOverlay()
	cipherKey, _ := rsaEncryptOverlayHex(overlayKey)
	cipherIV, _ := rsaEncryptOverlayHex(overlayIV)
	q := url.Values{"keyId": {"1"}, "psign": {auth["psign"]}}
	if cipherKey != "" && cipherIV != "" {
		q.Set("cipheredOverlayKey", cipherKey)
		q.Set("cipheredOverlayIv", cipherIV)
	}
	h := map[string]string{"Accept": "*/*", "User-Agent": ckjrUA}
	for _, k := range []string{"Referer", "Origin", "Cookie"} {
		if headers[k] != "" {
			h[k] = headers[k]
		}
	}
	body, err := c.GetString(apiURL+"?"+q.Encode(), h)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, err
	}
	if code, ok := payload["code"]; ok {
		s := strings.TrimSpace(fmt.Sprint(code))
		if s != "" && s != "0" && s != "<nil>" {
			return nil, nil
		}
	}
	return qcloudCandidatesFromPayload(c, payload, h, overlayKey, overlayIV), nil
}

func qcloudCandidatesFromPayload(c *util.Client, payload map[string]any, headers map[string]string, overlayKey, overlayIV string) []mediaCandidate {
	media := asMap(payload["media"])
	if len(media) == 0 {
		media = payload
	}
	size := parseSize(asMap(media["basicInfo"])["size"])
	var out []mediaCandidate
	add := func(u string, src string, item map[string]any) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		cand := mediaCandidate{URL: u, Format: pickFormat(u), Kind: "video", SourceKey: src, Size: firstPositiveSize(parseSize(item["size"]), size), Extra: map[string]any{"source_type": src}}
		if cand.Format == "m3u8" {
			cand.NeedMerge = true
		}
		out = append(out, cand)
	}

	streaming := asMap(media["streamingInfo"])
	drmToken := textValue(streaming, "drmToken", "drm_token")
	for _, key := range []string{"drmOutput", "plainOutput"} {
		for _, item := range mapsFromAny(streaming[key]) {
			if u := textValue(item, "url", "playURL", "playUrl"); u != "" {
				finalURL, text := qcloudFinalM3U8(c, u, drmToken, headers, overlayKey, overlayIV)
				candURL := firstNonEmpty(finalURL, u)
				extra := map[string]any{"source_type": "qcloud_" + key}
				if drmToken != "" {
					extra["drm_token"] = drmToken
				}
				if text != "" {
					extra["m3u8_text"] = text
					candURL = dataM3U8URL(text)
				}
				out = append(out, mediaCandidate{URL: candURL, Format: pickFormat(candURL), Kind: "video", SourceKey: "qcloud_" + key, Size: firstPositiveSize(parseSize(item["size"]), size), NeedMerge: strings.Contains(strings.ToLower(candURL), "m3u8") || strings.HasPrefix(candURL, "data:application/vnd.apple.mpegurl"), Extra: extra})
			}
		}
	}
	if mpl := asMap(streaming["masterPlayList"]); len(mpl) > 0 {
		if u := textValue(mpl, "url", "playUrl", "playURL"); u != "" {
			finalURL, text := qcloudFinalM3U8(c, u, drmToken, headers, overlayKey, overlayIV)
			if text != "" {
				extra := map[string]any{"source_type": "qcloud_masterPlayList", "m3u8_text": text}
				if drmToken != "" {
					extra["drm_token"] = drmToken
				}
				out = append(out, mediaCandidate{URL: dataM3U8URL(text), Format: "m3u8", Kind: "video", SourceKey: "qcloud_masterPlayList", Size: firstPositiveSize(parseSize(mpl["size"]), size), NeedMerge: true, Extra: extra})
			} else {
				add(firstNonEmpty(finalURL, u), "qcloud_masterPlayList", mpl)
			}
		}
	}
	transcode := asMap(media["transcodeInfo"])
	add(textValue(transcode, "url", "playUrl", "playURL"), "qcloud_transcode", transcode)
	transcodes := mapsFromAny(transcode["transcodeList"])
	sort.SliceStable(transcodes, func(i, j int) bool { return qcloudVariantRank(transcodes[i]) > qcloudVariantRank(transcodes[j]) })
	for _, item := range transcodes {
		if u := textValue(item, "url", "playUrl", "playURL"); u != "" {
			add(u, "qcloud_transcode", item)
		}
	}
	if src := asMap(media["sourceVideo"]); len(src) > 0 {
		add(textValue(src, "url", "playUrl", "playURL"), "qcloud_sourceVideo", src)
	}
	for _, key := range []string{"adaptive_streaming", "video_list"} {
		for _, item := range mapsFromAny(media[key]) {
			add(textValue(item, "url", "playUrl", "playURL"), "qcloud_"+key, item)
		}
	}
	if len(out) == 0 {
		for _, u := range extractTextMediaURLs(findMediaURL(payload)) {
			out = append(out, mediaCandidate{URL: u, Format: pickFormat(u), Kind: mediaKindFromFormat(pickFormat(u)), SourceKey: "qcloud_recursive", Size: size, Extra: map[string]any{"source_type": "qcloud_recursive"}})
		}
	}
	return out
}

func genOverlay() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return util.MD5(fmt.Sprint(rand.Reader))[:32]
	}
	return hex.EncodeToString(b)
}

func rsaEncryptOverlayHex(overlayText string) (string, error) {
	block, _ := pem.Decode([]byte(qcloudRSAPublicKey))
	if block == nil {
		return "", fmt.Errorf("qcloud: decode public key")
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("qcloud: not RSA public key")
	}
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(overlayText))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ciphertext), nil
}

func qcloudFinalM3U8(c *util.Client, masterURL, drmToken string, headers map[string]string, overlayKey, overlayIV string) (string, string) {
	if c == nil || !strings.Contains(strings.ToLower(masterURL), ".m3u8") {
		return masterURL, ""
	}
	body, err := c.GetString(masterURL, headers)
	if err != nil || !strings.Contains(body, "#EXTM3U") {
		return masterURL, ""
	}
	variantURL := masterURL
	text := body
	if strings.Contains(body, "#EXT-X-STREAM-INF") {
		variantURL = ckjrSelectVariantURL(body, masterURL)
		if variantURL != masterURL {
			if b, err := c.GetString(variantURL, headers); err == nil && b != "" {
				text = b
			}
		}
	}
	return variantURL, ckjrRewriteM3U8Text(c, text, variantURL, drmToken, headers, overlayKey, overlayIV)
}

func ckjrAppendToken(rawURL, token string) string {
	rawURL = strings.TrimSpace(rawURL)
	token = strings.TrimSpace(token)
	if rawURL == "" || token == "" || strings.Contains(rawURL, "token=") {
		return rawURL
	}
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + "token=" + url.QueryEscape(token)
}

func ckjrSelectVariantURL(masterText, masterURL string) string {
	type candidate struct {
		bw  int
		url string
	}
	var candidates []candidate
	lines := strings.Split(strings.ReplaceAll(masterText, "\r\n", "\n"), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			continue
		}
		bw := 0
		if m := regexp.MustCompile(`BANDWIDTH=(\d+)`).FindStringSubmatch(line); len(m) > 1 {
			bw, _ = strconv.Atoi(m[1])
		}
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" || strings.HasPrefix(next, "#") {
				continue
			}
			candidates = append(candidates, candidate{bw: bw, url: ckjrJoinURL(masterURL, next)})
			break
		}
	}
	if len(candidates) == 0 {
		return masterURL
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].bw > candidates[j].bw })
	return candidates[0].url
}

func ckjrRewriteM3U8Text(c *util.Client, text, baseURL, drmToken string, headers map[string]string, overlayKey, overlayIV string) string {
	var out []string
	uriRe := regexp.MustCompile(`URI="([^"]+)"`)
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if strings.Contains(trimmed, "URI=") {
				line = uriRe.ReplaceAllStringFunc(line, func(match string) string {
					m := uriRe.FindStringSubmatch(match)
					if len(m) < 2 {
						return match
					}
					u := ckjrJoinURL(baseURL, m[1])
					if strings.Contains(trimmed, "#EXT-X-KEY") {
						u = ckjrAppendToken(u, drmToken)
						if dataKey := ckjrQCloudKeyDataURL(c, u, baseURL, headers, overlayKey, overlayIV); dataKey != "" {
							u = dataKey
						}
					}
					return fmt.Sprintf(`URI="%s"`, u)
				})
			}
			out = append(out, line)
			continue
		}
		out = append(out, ckjrJoinURL(baseURL, trimmed))
	}
	return strings.Join(out, "\n") + "\n"
}

func ckjrQCloudKeyDataURL(c *util.Client, keyURL, referer string, headers map[string]string, overlayKey, overlayIV string) string {
	if c == nil || strings.TrimSpace(keyURL) == "" || strings.HasPrefix(strings.ToLower(keyURL), "data:") {
		return ""
	}
	key, iv, ok := ckjrOverlayKeyMaterial(overlayKey, overlayIV)
	if !ok {
		return ""
	}
	h := cloneStringMap(headers)
	if referer != "" {
		h["Referer"] = referer
	}
	raw, err := c.GetBytes(keyURL, h)
	if err != nil || len(raw) == 0 || len(raw)%aes.BlockSize != 0 {
		return ""
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	plain := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, raw)
	if len(plain) != aes.BlockSize {
		if unpadded := ckjrMaybePKCS7Key(plain); len(unpadded) == aes.BlockSize {
			plain = unpadded
		} else {
			return ""
		}
	}
	return "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(plain)
}

func ckjrOverlayKeyMaterial(overlayKey, overlayIV string) ([]byte, []byte, bool) {
	key, err := hex.DecodeString(strings.TrimSpace(overlayKey))
	if err != nil {
		return nil, nil, false
	}
	iv, err := hex.DecodeString(strings.TrimSpace(overlayIV))
	if err != nil || len(iv) != aes.BlockSize {
		return nil, nil, false
	}
	switch len(key) {
	case 16, 24, 32:
		return key, iv, true
	default:
		return nil, nil, false
	}
}

func ckjrMaybePKCS7Key(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > aes.BlockSize || padding > len(data) {
		return nil
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil
		}
	}
	return data[:len(data)-padding]
}

func ckjrJoinURL(base, ref string) string {
	if strings.TrimSpace(ref) == "" || strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "data:") {
		return ref
	}
	if strings.HasPrefix(ref, "//") {
		return "https:" + ref
	}
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return b.ResolveReference(r).String()
}

func mapsFromAny(v any) []map[string]any {
	if m := asMap(v); len(m) > 0 {
		return []map[string]any{m}
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m := asMap(item); len(m) > 0 {
			out = append(out, m)
		}
	}
	return out
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func parseSize(v any) int64 {
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return 0
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	if err != nil || f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return int64(f)
}

func firstPositiveSize(vals ...int64) int64 {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 0
}

func qcloudVariantRank(item map[string]any) int64 {
	for _, key := range []string{"size", "bitrate", "definition", "width"} {
		if n := parseSize(item[key]); n > 0 {
			return n
		}
	}
	return 0
}
