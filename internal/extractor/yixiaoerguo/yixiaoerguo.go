// Package yixiaoerguo implements an extractor for biguo.cn / qianxuecloud playback.
package yixiaoerguo

import (
	"crypto/md5"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

const (
	refererURL             = "https://www.biguo.cn/my/course"
	originURL              = "https://www.biguo.cn"
	apiBase                = "https://api.biguo.cn"
	qxRecordQueryURL       = "https://bjs1.qianxuecloud.com/recordquery"
	qxRecordQueryBackupURL = "https://bjs1.qianxuecloud.com/recordquerybackup"
	qxRecordQueryMuURL     = "https://bjs1.qianxuecloud.com/recordquerymu"
	qxPlaybackQueryWebHLS  = "https://vodquerys1.qianxuecloud.com/playbackquerywebhls"
	qxDataPlaybackQueryH5  = "https://vodquerydatas1.qianxuecloud.com/dataplaybackqueryh5"
	qxReplaySVRURL         = "https://s1rqs.qianxuecloud.com/rqs/wsreplaysvr"
	qxHLSEncryptURL        = "https://svrquerys1.qianxuecloud.com/rqs/hls_encrypt"
	qxMediaReferer         = "https://lives1.qianxuecloud.com/live_sc/"
	courseListPath         = "/api/courses"
	courseChaptersPathFmt  = "/api/courses/%s/chapters"
	productChaptersPathFmt = "/api/courses/products/%s/chapters"
	sectionPlayInfoPathFmt = "/api/courses/sections/%s/%s"
	auditionUnlockPath     = "/api/courses/audition/unlock"
	courseProductPathFmt   = "/api/courses/products/%s"
	courseDetailPathFmt    = "/api/courses/%s"
	xscClient              = "otLVIOEO"
	xscAPIVersion          = "5"
)

var (
	patterns     = []string{`(?:[\w-]+\.)?(?:biguo|qianxuecloud)\.(?:cn|com)/`}
	cidRe        = regexp.MustCompile(`(?i)(?:/courses?/|courseId=|cid=|id=)([0-9a-f]{24})`)
	hex24Re      = regexp.MustCompile(`(?i)[0-9a-f]{24}`)
	titleCleanRe = regexp.MustCompile(`[\\/:*?"<>|\r\n\t]+`)
)

func init() {
	extractor.Register(&Yixiaoerguo{}, extractor.SiteInfo{Name: "Yixiaoerguo", URL: "biguo.cn", NeedAuth: true})
}

type Yixiaoerguo struct{}

func (y *Yixiaoerguo) Patterns() []string { return patterns }

type yxContext struct {
	c       *util.Client
	token   string
	cid     string
	headers map[string]string
}

type yxVideo struct {
	SectionID string
	Type      string
	State     string
	Title     string
	Duration  string
	CanTry    bool
}

func (y *Yixiaoerguo) Extract(rawURL string, opts *extractor.ExtractOpts) (*extractor.MediaInfo, error) {
	if opts == nil || opts.Cookies == nil {
		return nil, fmt.Errorf("yixiaoerguo requires login cookies")
	}
	cid := parseCID(rawURL)
	if cid == "" {
		return nil, fmt.Errorf("yixiaoerguo: cannot parse 24-hex course id from URL")
	}
	c := util.NewClient()
	c.SetCookieJar(opts.Cookies)
	ctx := &yxContext{c: c, token: tokenFromJar(opts.Cookies), cid: cid}
	if ctx.token == "" {
		return nil, fmt.Errorf("yixiaoerguo: sc_token_pro token is required")
	}
	ctx.headers = ctx.apiHeaders(courseListPath)
	if err := ctx.checkCookie(); err != nil {
		return nil, err
	}
	title := ctx.courseTitle()
	payload, err := ctx.chaptersPayload()
	if err != nil {
		return nil, err
	}
	videos := collectVideos(payload)
	if len(videos) == 0 {
		return nil, fmt.Errorf("yixiaoerguo: no chapter sections found")
	}
	entries := make([]*extractor.MediaInfo, 0, len(videos))
	for _, v := range videos {
		if entry := ctx.resolveEntry(v); entry != nil {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("yixiaoerguo: no qianxuecloud media resolved")
	}
	if title == "" {
		title = "yixiaoerguo_" + cid
	}
	return &extractor.MediaInfo{Site: "yixiaoerguo", Title: cleanTitle(title), Entries: entries}, nil
}

func parseCID(raw string) string {
	if m := cidRe.FindStringSubmatch(raw); len(m) > 1 && m[1] != "" {
		return strings.ToLower(m[1])
	}
	for _, loc := range hex24Re.FindAllStringIndex(raw, -1) {
		if hasHexNeighbor(raw, loc[0], loc[1]) {
			continue
		}
		return strings.ToLower(raw[loc[0]:loc[1]])
	}
	return ""
}

func hasHexNeighbor(s string, start, end int) bool {
	return (start > 0 && isASCIIHex(s[start-1])) || (end < len(s) && isASCIIHex(s[end]))
}

func isASCIIHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func tokenFromJar(jar http.CookieJar) string {
	for _, raw := range []string{refererURL, originURL + "/", apiBase + "/"} {
		u, _ := url.Parse(raw)
		for _, ck := range jar.Cookies(u) {
			if ck.Name == "sc_token_pro" || ck.Name == "token" || ck.Name == "Authorization" {
				return normalizeToken(ck.Value)
			}
		}
	}
	return ""
}

func normalizeToken(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(strings.ToLower(v), "bearer ") {
		return strings.TrimSpace(v[7:])
	}
	return v
}

func (x *yxContext) apiHeaders(path string) map[string]string {
	uriPath := path
	if strings.HasPrefix(path, "http") {
		if u, err := url.Parse(path); err == nil {
			uriPath = u.Path
		}
	}
	timestamp := fmt.Sprint(time.Now().UnixMilli())
	nonce := xscNonce()
	saltSeed := nonce + timestamp
	mid := len(saltSeed) / 2
	salt := saltSeed[:2] + saltSeed[mid:mid+2] + saltSeed[len(saltSeed)-2:]
	sum := md5.Sum([]byte(strings.ToUpper("salt=" + salt + "&path=" + uriPath)))
	return map[string]string{"Accept": "application/json, text/plain, */*", "Authorization": x.token, "Content-Type": "application/json", "Origin": originURL, "Referer": refererURL, "XSC-API-VERSION": xscAPIVersion, "XSC-CLIENT": xscClient, "XSC-NONSTR": nonce, "XSC-TIMESTAMP": timestamp, "XSC-SIGN": hex.EncodeToString(sum[:])}
}

func xscNonce() string {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (x *yxContext) checkCookie() error {
	resp, err := x.requestAPI(courseListPath, "GET", map[string]string{"current": "1", "page": "1", "pageSize": "20", "countTotal": "1", "free": "true"}, nil)
	if err != nil {
		return fmt.Errorf("yixiaoerguo check cookie: %w", err)
	}
	if successFalse(resp) {
		return fmt.Errorf("yixiaoerguo check cookie failed: %s", firstString(resp, "message", "msg", "error"))
	}
	return nil
}

func (x *yxContext) courseTitle() string {
	resp, err := x.requestAPI(fmt.Sprintf(courseDetailPathFmt, x.cid), "GET", nil, nil)
	if err == nil {
		if t := firstString(asMap(resp["data"]), "name", "title", "courseName"); t != "" {
			return t
		}
	}
	resp, err = x.requestAPI(fmt.Sprintf(courseProductPathFmt, x.cid), "GET", nil, nil)
	if err != nil {
		return ""
	}
	return firstString(asMap(resp["data"]), "name", "title", "courseName")
}

func (x *yxContext) chaptersPayload() (map[string]any, error) {
	for _, p := range []string{fmt.Sprintf(courseChaptersPathFmt, x.cid), fmt.Sprintf(productChaptersPathFmt, x.cid)} {
		resp, err := x.requestAPI(p, "GET", nil, nil)
		if err != nil {
			continue
		}
		if len(extractItems(dig(resp, "data", "chapters"))) > 0 {
			return resp, nil
		}
	}
	return nil, fmt.Errorf("yixiaoerguo chapters empty")
}

func (x *yxContext) requestAPI(path, method string, params map[string]string, data any) (map[string]any, error) {
	apiURL := path
	if !strings.HasPrefix(apiURL, "http") {
		apiURL = apiBase + path
	}
	if method == "" {
		method = "GET"
	}
	h := x.apiHeaders(path)
	if strings.EqualFold(method, "GET") {
		if len(params) > 0 {
			u, _ := url.Parse(apiURL)
			q := u.Query()
			for k, v := range params {
				q.Set(k, v)
			}
			u.RawQuery = q.Encode()
			apiURL = u.String()
		}
		body, err := x.c.GetString(apiURL, h)
		if err != nil {
			return nil, err
		}
		return parseJSON(body)
	}
	b, _ := json.Marshal(data)
	resp, err := x.c.Post(apiURL, strings.NewReader(string(b)), h)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseJSON(string(body))
}

func collectVideos(payload map[string]any) []yxVideo {
	chapters := extractItems(dig(payload, "data", "chapters"))
	var out []yxVideo
	var walk func(any, []int, []string)
	walk = func(v any, idx []int, names []string) {
		if arr, ok := v.([]any); ok {
			for i, it := range arr {
				walk(it, append(idx, i+1), append([]string{}, names...))
			}
			return
		}
		m := asMap(v)
		if len(m) == 0 {
			return
		}
		name := firstString(m, "name", "title", "sectionName")
		if name != "" {
			names = append(names, name)
		}
		children := firstNonNil(m["sections"], m["children"])
		if len(extractItems(children)) > 0 {
			walk(children, idx, names)
			return
		}
		id := firstString(m, "id", "sectionId", "periodId")
		if id == "" {
			return
		}
		out = append(out, yxVideo{SectionID: id, Type: firstString(m, "type", "sectionType"), State: firstString(m, "state"), Title: cleanTitle(fmt.Sprintf("[%s]--%s", joinIdx(idx), firstNonEmpty(name, id))), Duration: firstString(m, "duration", "expected_duration"), CanTry: boolValue(firstNonNil(m["can_try"], m["canTry"]))})
	}
	walk(chapters, nil, nil)
	return out
}

func (x *yxContext) resolveEntry(v yxVideo) *extractor.MediaInfo {
	play := x.sectionPlayInfo(v)
	token := extractQXToken(play)
	if token == "" {
		return nil
	}
	media := getQXRecordMedia(x.c, token, v.Duration)
	playURLs := media.URLs
	if len(playURLs) == 0 && media.URL != "" {
		playURLs = []string{media.URL}
	}
	playURL := ""
	if len(playURLs) > 0 {
		playURL = playURLs[0]
	}
	if playURL == "" {
		playURL = getQXHLSURL(x.c, token)
		if playURL != "" {
			playURLs = []string{playURL}
		}
	}
	if playURL == "" {
		return nil
	}
	stream := extractor.Stream{Quality: "best", URLs: playURLs, Format: pickFormat(playURL), Size: media.SizeBytes, Headers: map[string]string{"Referer": qxMediaReferer}}
	stream.NeedMerge = len(playURLs) > 1 || stream.Format == "m3u8"
	extra := map[string]any{"section_id": v.SectionID, "qx_token": token, "qx_duration": media.Duration, "qx_size_mb": media.Size}
	if len(media.Segments) > 0 {
		extra["qx_segments"] = media.Segments
	}
	return &extractor.MediaInfo{Site: "yixiaoerguo", Title: v.Title, Streams: map[string]extractor.Stream{"default": stream}, Extra: extra}
}

func (x *yxContext) sectionPlayInfo(v yxVideo) map[string]any {
	order := []string{"playback_info", "record_info", "live_info"}
	typ, state := strings.ToUpper(v.Type), strings.ToUpper(v.State)
	if typ == "LIVE" && !(state == "4" || state == "ENDED" || state == "FINISHED") {
		order = []string{"live_info", "playback_info", "record_info"}
	} else if typ == "RECORD" || typ == "VIDEO" {
		order = []string{"record_info", "playback_info", "live_info"}
	}
	for _, kind := range order {
		resp, err := x.requestAPI(fmt.Sprintf(sectionPlayInfoPathFmt, v.SectionID, kind), "GET", nil, nil)
		if err == nil && len(asMap(resp["data"])) > 0 {
			return asMap(resp["data"])
		}
	}
	if v.CanTry {
		_, _ = x.requestAPI(auditionUnlockPath, "POST", nil, map[string]string{"courseId": x.cid, "sectionId": v.SectionID})
	}
	return map[string]any{}
}

func extractQXToken(v any) string {
	if t := firstString(asMap(digAny(v, "qx", "app")), "token"); t != "" {
		return t
	}
	for _, u := range findURLs(v, "url", "h5Ur") {
		if parsed, err := url.Parse(u); err == nil {
			if t := parsed.Query().Get("token"); t != "" {
				return t
			}
		}
	}
	return ""
}

func getQXRecordMedia(c *util.Client, token string, expectedDuration string) qxMediaInfo {
	for _, apiURL := range []string{qxRecordQueryURL, qxRecordQueryBackupURL, qxRecordQueryMuURL} {
		body, err := c.GetString(apiURL+"?token="+url.QueryEscape(token), nil)
		if err != nil {
			continue
		}
		resp, err := parseJSON(body)
		if err != nil {
			continue
		}
		dataURL := firstString(resp, "url")
		if dataURL == "" && strings.HasPrefix(firstString(resp, "urlMedia"), "http") {
			u := firstString(resp, "urlMedia")
			return qxMediaInfo{URL: u, URLs: []string{u}, Raw: resp}
		}
		if dataURL == "" {
			continue
		}
		body, err = c.GetString(dataURL, nil)
		if err != nil {
			continue
		}
		mediaResp, err := parseJSON(body)
		if err != nil {
			continue
		}
		if info := buildQXMediaInfo(extractItems(mediaResp["data"]), expectedDuration); info.URL != "" || len(info.URLs) > 0 {
			return info
		}
	}
	return qxMediaInfo{}
}

func getQXHLSURL(c *util.Client, token string) string {
	for _, apiURL := range []string{qxPlaybackQueryWebHLS, qxDataPlaybackQueryH5} {
		body, err := c.GetString(apiURL+"?token="+url.QueryEscape(token), nil)
		if err != nil {
			continue
		}
		resp, err := parseQXMaybeEncryptedJSON(body)
		if err != nil {
			continue
		}
		for _, u := range findURLs(resp, "cdn_url", "url", "playUrl", "hlsUrl", "address") {
			if strings.Contains(u, ".m3u8") && strings.HasPrefix(u, "http") {
				return u
			}
		}
		if address := findFirstStringDeep(resp, "cdn_url", "address", "playUrl", "url"); address != "" {
			if decrypted := decryptQXHLSAddress(c, token, address); decrypted != "" {
				return decrypted
			}
		}
	}
	_ = qxReplaySVRURL
	return ""
}

func decryptQXHLSAddress(c *util.Client, token, address string) string {
	if token == "" || address == "" {
		return ""
	}
	body, err := c.GetString(qxHLSEncryptURL+"?token="+url.QueryEscape(token), nil)
	if err != nil {
		return ""
	}
	servers := serverListFromQXEncrypt(body)
	if len(servers) == 0 {
		return ""
	}
	server := servers[0]
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		server = "https://" + server
	}
	payloadBytes, _ := json.Marshal(map[string]string{"address": address})
	payload := base64.StdEncoding.EncodeToString(payloadBytes)
	resp, err := c.Post(server+"/hls_address?token="+url.QueryEscape(token), strings.NewReader(qxJunkEncode(payload, 3, 1)), nil)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	decoded := decodeQXBase64JSON(qxJunkDecode(string(b), 3, 1))
	if decoded == nil {
		return ""
	}
	if content := asMap(decoded["content"]); len(content) > 0 {
		if u := firstString(content, "address", "url"); u != "" {
			return u
		}
	}
	return firstString(decoded, "address", "url")
}

func serverListFromQXEncrypt(body string) []string {
	decoded := decodeQXBase64JSON(qxJunkDecode(body, 3, 1))
	if decoded == nil {
		return nil
	}
	list := firstNonNil(decoded["serverlist"], digAny(decoded, "data", "serverlist"))
	var out []string
	switch t := list.(type) {
	case []any:
		for _, it := range t {
			if m := asMap(it); len(m) > 0 {
				if addr := firstString(m, "addr", "url"); addr != "" {
					out = append(out, addr)
				}
			} else if s := strings.TrimSpace(fmt.Sprint(it)); s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
	case map[string]any:
		if addr := firstString(t, "addr", "url"); addr != "" {
			out = append(out, addr)
		}
	}
	return out
}
