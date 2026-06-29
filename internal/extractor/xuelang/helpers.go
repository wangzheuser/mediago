package xuelang

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

func postJSON(c *util.Client, api string, payload map[string]any, h map[string]string) (map[string]any, error) {
	b, _ := json.Marshal(payload)
	resp, err := c.Post(api, bytes.NewReader(b), h)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return parseJSON(string(raw))
}
func parseJSON(body string) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil, fmt.Errorf("xuelang parse JSON: %w", err)
	}
	return root, nil
}
func headers(cookie string) map[string]string {
	return map[string]string{"cookie": cookie, "Cookie": cookie, "referer": refererURL, "Referer": refererURL, "User-Agent": defaultUA + " " + strconv.FormatInt(time.Now().UnixNano(), 36), "Content-Type": "application/json"}
}
func cookieHeader(jar http.CookieJar) string {
	parts := []string{}
	for _, raw := range []string{refererURL, "https://student-api.iyincaishijiao.com", "https://classroom.iyincaishijiao.com", "https://vod.bytedanceapi.com"} {
		if u, err := url.Parse(raw); err == nil {
			for _, c := range jar.Cookies(u) {
				parts = append(parts, c.Name+"="+c.Value)
			}
		}
	}
	return strings.Join(parts, "; ")
}
func selectCourse(cs []course, cid string) course {
	for _, c := range cs {
		if cid == "" || c.id == cid {
			return c
		}
	}
	return course{}
}
func media(title string, pm playMedia, co course, l lesson) *extractor.MediaInfo {
	if pm.titleSuffix != "" {
		title += pm.titleSuffix
	}
	extra := map[string]any{"course_id": co.id, "room_id": l.roomID, "video_id": pm.videoID, "key_id": pm.keyID}
	if pm.m3u8Text != "" {
		extra["m3u8_text"] = pm.m3u8Text
		extra["source_type"] = "m3u8_text"
	}
	return &extractor.MediaInfo{Site: "xuelang", Title: title, Streams: map[string]extractor.Stream{"default": {Quality: "default", URLs: []string{pm.videoURL}, Format: "m3u8", Size: pm.size, AudioURL: pm.audioURL, NeedMerge: true, Headers: map[string]string{"Referer": refererURL}}}, Extra: extra}
}

func firstMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	for i := 1; i < len(m); i++ {
		if m[i] != "" {
			return m[i]
		}
	}
	return ""
}
func firstMediaURL(v any) string {
	for _, m := range mapsUnder(v) {
		for _, k := range []string{"MainPlayUrl", "BackupPlayUrl", "play_url", "m3u8", "url", "preview_url"} {
			if u := val(m, k); strings.HasPrefix(u, "http") {
				return u
			}
		}
	}
	return ""
}
func mapAt(v any, key string) map[string]any {
	if m, ok := valueAt(v, key).(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
func listUnder(v any, key string) []map[string]any { return listFrom(valueAt(v, key)) }
func listFrom(v any) []map[string]any {
	out := []map[string]any{}
	if a, ok := v.([]any); ok {
		for _, x := range a {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}
func valueAt(v any, key string) any {
	for _, m := range mapsUnder(v) {
		if x, ok := m[key]; ok {
			return x
		}
	}
	return nil
}
func mapsUnder(v any) []map[string]any {
	out := []map[string]any{}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			out = append(out, t)
			for _, y := range t {
				walk(y)
			}
		case []any:
			for _, y := range t {
				walk(y)
			}
		}
	}
	walk(v)
	return out
}
func val(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return asString(m[key])
}
func asString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	case float64:
		if math.Trunc(x) == x {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	}
	return ""
}
func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}
func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}
func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != "" && x != "0" && x != "false"
	}
	return false
}
func unique(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, x := range in {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func uniqueLiveTokens(in []liveToken) []liveToken {
	out := []liveToken{}
	seen := map[string]bool{}
	for _, x := range in {
		if x.token != "" && !seen[x.token] {
			seen[x.token] = true
			out = append(out, x)
		}
	}
	return out
}

func prepareXuelangM3U8(c *util.Client, h map[string]string, m3u8URL, keyHex string) (string, string, bool) {
	text, err := c.GetString(m3u8URL, h)
	if err != nil || !strings.Contains(text, "#EXTM3U") {
		return "", "", false
	}
	base, _ := url.Parse(m3u8URL)
	keyURI := strings.TrimSpace(keyHex)
	out := make([]string, 0, strings.Count(text, "\n")+1)
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-KEY:") && keyURI != "" {
			line = replaceXuelangKeyURI(line, keyURI)
		} else if !strings.HasPrefix(line, "#") {
			line = resolveXuelangLine(base, line)
		}
		out = append(out, line)
	}
	manifest := strings.Join(out, "\n") + "\n"
	return "data:application/vnd.apple.mpegurl;base64," + base64.StdEncoding.EncodeToString([]byte(manifest)), manifest, true
}

func replaceXuelangKeyURI(line, uri string) string {
	if i := strings.Index(line, `URI="`); i >= 0 {
		rest := line[i+5:]
		if j := strings.Index(rest, `"`); j >= 0 {
			return line[:i+5] + uri + rest[j:]
		}
	}
	return line
}

func resolveXuelangLine(base *url.URL, raw string) string {
	if raw == "" || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "data:") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if base == nil {
		return raw
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return base.ResolveReference(ref).String()
}

func decryptXuelangKey(secret, ciphertext string) string {
	if secret == "" || ciphertext == "" {
		return ""
	}
	keyMaterial := secret
	if len(keyMaterial) >= 16 {
		keyMaterial = keyMaterial[len(keyMaterial)-16:] + keyMaterial[:16]
	}
	key := []byte(keyMaterial)
	if len(key) > 32 {
		key = key[:32]
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return ""
	}
	iv := []byte(secret)
	if len(iv) >= 16 {
		iv = iv[len(iv)-16:]
	} else {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil || len(raw)%aes.BlockSize != 0 {
		return ""
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	plain := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, raw)
	plain = pkcs7Unpad(plain)
	if len(plain) == 0 {
		return ""
	}
	hexText := strings.ToUpper(fmt.Sprintf("%X", string(plain)))
	return hexText[:minInt(32, len(hexText))]
}

func pkcs7Unpad(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	n := int(b[len(b)-1])
	if n < 1 || n > aes.BlockSize || n > len(b) {
		return b
	}
	for _, v := range b[len(b)-n:] {
		if int(v) != n {
			return b
		}
	}
	return b[:len(b)-n]
}

func xuelangNodeIDs(v any) []string {
	var out []string
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			switch x := item.(type) {
			case string:
				out = append(out, x)
			case map[string]any:
				out = append(out, firstNonEmpty(val(x, "id"), val(x, "obj_id"), val(x, "node_id")))
			default:
				out = append(out, asString(x))
			}
		}
	case []string:
		out = append(out, t...)
	}
	return unique(out)
}

func joinIndexes(indexes []int) string {
	parts := make([]string, 0, len(indexes))
	for _, n := range indexes {
		if n > 0 {
			parts = append(parts, strconv.Itoa(n))
		}
	}
	if len(parts) == 0 {
		return "1"
	}
	return strings.Join(parts, ".")
}

func cleanXuelangName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "资料"
	}
	return regexp.MustCompile(`[\\/:*?"<>|\r\n\t]+`).ReplaceAllString(s, "_")
}

func xuelangFormat(raw string) string {
	p := strings.ToLower(strings.SplitN(strings.SplitN(raw, "?", 2)[0], "#", 2)[0])
	if i := strings.LastIndex(p, "."); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return "bin"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
