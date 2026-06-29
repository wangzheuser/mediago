// Package qihang implements an extractor for iqihang.com courses.
package qihang

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/extractor/shared"
	"github.com/Sophomoresty/mediago/internal/util"
)

const (
	referer        = "https://www.iqihang.com"
	course_url     = "https://www.iqihang.com/api/ark/web/v1/user/course/course-list?isMarketingCourse=&status=&type=1"
	info_url       = "https://www.iqihang.com/api/ark/web/v1/course/catalog/%s"
	video_play_url = "https://p.bokecc.com/servlet/getvideofile?vid=%s&siteid=A183AC83A2983CCC"
	live_url       = "https://www.iqihang.com/api/ark/web/v1/user/course/live/replay?liveId=%s"
	live_login_url = "https://view.csslcloud.net/api/room/replay/login?roomid=%s&userid=%s&recordid=%s&viewertoken=%s%%3A%s"
	live_play_url  = "https://view.csslcloud.net/api/record/vod?accountId=%s&recordId=%s&terminal=3&token=%s"
	source_url     = "https://www.iqihang.com/api/ark/web/v1/lecture/curriculum/node?curriculumId=%s"
	price_url      = "https://iqihang.com/api/ark/web/v1/product/%s"
	user_info_url  = "https://www.iqihang.com/api/ark/web/v1/user/info"
	bokeccSiteID   = "A183AC83A2983CCC"
)

var patterns = []string{`(?:[\w-]+\.)?iqihang\.com/`}

func init() {
	extractor.Register(&Qihang{}, extractor.SiteInfo{Name: "Qihang", URL: "iqihang.com", NeedAuth: true})
}

type Qihang struct{}

func (s *Qihang) Patterns() []string { return patterns }

func (s *Qihang) Extract(rawURL string, opts *extractor.ExtractOpts) (*extractor.MediaInfo, error) {
	if opts == nil || opts.Cookies == nil {
		return nil, fmt.Errorf("qihang requires login cookies")
	}
	c := util.NewClient()
	c.SetCookieJar(opts.Cookies)
	h := qihangHeaders(opts.Cookies)
	uid, _ := fetchUID(c, h)
	cid, learnID, productID := parseIDs(rawURL)

	courses, _ := fetchCourseList(c, h)
	if cid == "" {
		for _, it := range courses {
			if (productID != "" && it.ProductID == productID) || (learnID != "" && it.LearnID == learnID) {
				cid, productID, learnID = it.CourseID, it.ProductID, it.LearnID
				break
			}
		}
	}
	if cid == "" {
		return nil, fmt.Errorf("qihang: cannot map URL to productCurriculumId")
	}
	title := titleFromCourse(c, h, productID, courses, cid)
	if title == "" {
		title = "qihang_" + cid
	}

	nodes, err := fetchNodes(c, fmt.Sprintf(info_url, cid), true, h)
	if err != nil {
		return nil, fmt.Errorf("qihang course/catalog: %w", err)
	}
	sourceNodes, _ := fetchNodes(c, fmt.Sprintf(source_url, cid), false, h)
	nodes = append(nodes, sourceNodes...)

	seen := map[string]bool{}
	var entries []*extractor.MediaInfo
	collectEntries(c, h, nodes, nil, uid, learnID, seen, &entries)
	if len(entries) == 0 {
		return nil, fmt.Errorf("qihang: no downloadable entries found from courseNodes/resourceList")
	}
	return &extractor.MediaInfo{Site: "qihang", Title: title, Entries: entries}, nil
}

type qCourse struct{ LearnID, ProductID, CourseID, Title string }

func fetchCourseList(c *util.Client, h map[string]string) ([]qCourse, error) {
	body, err := c.GetString(course_url, h)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []struct {
			ID                  any    `json:"id"`
			ProductID           any    `json:"productId"`
			ProductCurriculumID any    `json:"productCurriculumId"`
			ProductName         string `json:"productName"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, err
	}
	out := make([]qCourse, 0, len(resp.Data))
	for _, it := range resp.Data {
		out = append(out, qCourse{LearnID: jstr(it.ID), ProductID: jstr(it.ProductID), CourseID: jstr(it.ProductCurriculumID), Title: it.ProductName})
	}
	return out, nil
}

func titleFromCourse(c *util.Client, h map[string]string, productID string, courses []qCourse, cid string) string {
	if productID != "" {
		body, err := c.GetString(fmt.Sprintf(price_url, productID), h)
		if err == nil {
			var resp struct {
				Data struct {
					Name      string `json:"name"`
					SellPrice any    `json:"sellPrice"`
				} `json:"data"`
			}
			if json.Unmarshal([]byte(body), &resp) == nil && strings.TrimSpace(resp.Data.Name) != "" {
				return sanitize(resp.Data.Name)
			}
		}
	}
	for _, it := range courses {
		if it.CourseID == cid && it.Title != "" {
			return sanitize(it.Title)
		}
	}
	return ""
}

type qNode struct {
	Name              string      `json:"name"`
	Children          []qNode     `json:"children"`
	StudyResourceType int         `json:"studyResourceType"`
	ResourceList      []qResource `json:"resourceList"`
}
type qResource struct {
	Vid        any    `json:"vid"`
	ResourceID any    `json:"resourceId"`
	LectureURL string `json:"lectureUrl"`
}

func fetchNodes(c *util.Client, api string, catalog bool, h map[string]string) ([]qNode, error) {
	body, err := c.GetString(api, h)
	if err != nil {
		return nil, err
	}
	if catalog {
		var resp struct {
			Data struct {
				CourseNodes []qNode `json:"courseNodes"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			return nil, err
		}
		return resp.Data.CourseNodes, nil
	}
	var resp struct {
		Data []qNode `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func collectEntries(c *util.Client, h map[string]string, nodes []qNode, prefix []int, uid, learnID string, seen map[string]bool, entries *[]*extractor.MediaInfo) {
	for i, n := range nodes {
		idx := append(append([]int{}, prefix...), i+1)
		if len(n.Children) > 0 {
			collectEntries(c, h, n.Children, idx, uid, learnID, seen, entries)
		}
		if len(n.ResourceList) == 0 {
			continue
		}
		r := n.ResourceList[0]
		switch {
		case n.StudyResourceType == 2 || n.StudyResourceType == 3:
			// Video / live replay
			vid, liveID := jstr(r.Vid), jstr(r.ResourceID)
			key := vid + ":" + liveID + ":" + n.Name
			if key == "::" || seen[key] {
				continue
			}
			seen[key] = true
			mi := resolveVideo(c, h, idx, n.Name, vid, liveID, uid, learnID)
			if mi != nil {
				*entries = append(*entries, mi)
			}
		case n.StudyResourceType == 4:
			// File node (PDF, PPT, DOC, attachment, etc.)
			mi := resolveFile(idx, n.Name, r.LectureURL)
			if mi != nil {
				key := "file:" + r.LectureURL + ":" + n.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				*entries = append(*entries, mi)
			}
		}
	}
}

func resolveVideo(c *util.Client, h map[string]string, idx []int, name, vid, liveID, uid, learnID string) *extractor.MediaInfo {
	title := sanitize(fmt.Sprintf("[%s]-%s", joinInts(idx), strings.TrimSuffix(name, ".mp4")))
	if vid != "" {
		if u, err := shared.BokeCCResolve(c, vid, bokeccSiteID, h); err == nil && u != "" {
			return mediaWithPreparedM3U8(c, h, title, u, map[string]any{"video_id": vid})
		}
	}
	if liveID != "" {
		if u, audio, err := resolveLive(c, h, liveID, uid, learnID); err == nil && u != "" {
			m := mediaWithPreparedM3U8(c, h, title, u, map[string]any{"live_id": liveID})
			if audio != "" {
				st := m.Streams["best"]
				st.AudioURL = audio
				m.Streams["best"] = st
			}
			return m
		}
	}
	return nil
}

func resolveLive(c *util.Client, h map[string]string, liveID, uid, learnID string) (string, string, error) {
	body, err := c.GetString(fmt.Sprintf(live_url, liveID), h)
	if err != nil {
		return "", "", err
	}
	var resp struct {
		Data struct {
			ReplayURL string `json:"replayUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return "", "", err
	}
	roomID, userID, recordID := replayArgs(resp.Data.ReplayURL)
	if roomID == "" || userID == "" || recordID == "" {
		return "", "", fmt.Errorf("qihang live: replayUrl lacks roomid/userid/recordid")
	}
	pi, err := shared.CssLcloudResolvePlayInfo(c, shared.CssLcloudPayload{
		LiveRoomID: roomID, UserID: userID, AccessID: userID, RecordID: recordID,
		ViewerToken: uid + ":" + learnID, Referer: referer,
	})
	if err != nil {
		return "", "", err
	}
	return pi.VideoURL, pi.AudioURL, nil
}

// resolveFile builds a MediaInfo for a file node (studyResourceType == 4).
// Mirrors _parse_file_info in the source: takes lectureUrl, derives format from
// the URL path extension, and surfaces the file as a downloadable entry.
func resolveFile(idx []int, name, lectureURL string) *extractor.MediaInfo {
	if lectureURL == "" {
		return nil
	}
	title := sanitize(fmt.Sprintf("(%s)-%s", joinInts(idx), name))
	// Extract file extension from the URL path (before query string), as the
	// source does: lectureUrl.split('?')[0].rsplit('.', 1)[-1]
	pathPart := lectureURL
	if qIdx := strings.IndexByte(pathPart, '?'); qIdx >= 0 {
		pathPart = pathPart[:qIdx]
	}
	ext := ""
	if dotIdx := strings.LastIndexByte(pathPart, '.'); dotIdx >= 0 {
		ext = strings.ToLower(pathPart[dotIdx+1:])
	}
	// Determine stream format: mp4 → video, everything else → attachment file
	format := ext
	if format == "" {
		format = "bin"
	}
	return &extractor.MediaInfo{
		Site:  "qihang",
		Title: title,
		Streams: map[string]extractor.Stream{
			"best": {
				Quality: "best",
				URLs:    []string{lectureURL},
				Format:  format,
				Headers: map[string]string{"Referer": referer},
			},
		},
		Extra: map[string]any{"type": "file", "file_fmt": ext},
	}
}

func media(title, u string, extra map[string]any) *extractor.MediaInfo {
	format := pickFormat(u)
	return &extractor.MediaInfo{Site: "qihang", Title: title, Streams: map[string]extractor.Stream{"best": {Quality: "best", URLs: []string{u}, Format: format, NeedMerge: format == "m3u8", Headers: map[string]string{"Referer": referer}}}, Extra: extra}
}

func mediaWithPreparedM3U8(c *util.Client, h map[string]string, title, streamURL string, extra map[string]any) *extractor.MediaInfo {
	if preparedURL, m3u8Text, ok := prepareQihangM3U8(c, h, streamURL); ok {
		streamURL = preparedURL
		if extra == nil {
			extra = map[string]any{}
		}
		extra["m3u8_text"] = m3u8Text
		extra["m3u8_prepared"] = true
	}
	return media(title, streamURL, extra)
}

func prepareQihangM3U8(c *util.Client, h map[string]string, m3u8URL string) (string, string, bool) {
	if c == nil || !strings.Contains(strings.ToLower(m3u8URL), ".m3u8") {
		return "", "", false
	}
	headers := qihangM3U8Headers(h)
	text, err := c.GetString(m3u8URL, headers)
	if err != nil || !strings.Contains(text, "#EXTM3U") {
		return "", "", false
	}
	rewritten := rewriteQihangM3U8(c, headers, text, m3u8URL)
	return qihangM3U8DataURL(rewritten), rewritten, true
}

func rewriteQihangM3U8(c *util.Client, headers map[string]string, text, baseURL string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			if strings.HasPrefix(trimmed, "#EXT-X-KEY") {
				line = rewriteQihangM3U8KeyLine(c, headers, line, baseURL)
			}
			out = append(out, line)
			continue
		}
		out = append(out, resolveQihangM3U8Line(baseURL, trimmed))
	}
	return strings.Join(out, "\n")
}

func rewriteQihangM3U8KeyLine(c *util.Client, headers map[string]string, line, baseURL string) string {
	uri := extractQihangM3U8URI(line)
	if uri == "" {
		return line
	}
	resolvedURI := resolveQihangM3U8Line(baseURL, uri)
	replacement := resolvedURI
	if info := qihangM3U8InfoParam(resolvedURI); info != "" && c != nil {
		if keyBytes, err := c.GetBytes(resolvedURI, headers); err == nil {
			if keyHex := decryptQihangKey(keyBytes, info); keyHex != "" {
				replacement = "0x" + keyHex
			}
		}
	}
	return replaceQihangM3U8URI(line, replacement)
}

func qihangM3U8Headers(base map[string]string) map[string]string {
	headers := map[string]string{"Referer": referer}
	for k, v := range base {
		if strings.EqualFold(k, "authorization") || strings.EqualFold(k, "cookie") || strings.EqualFold(k, "user-agent") {
			headers[k] = v
		}
	}
	return headers
}

func extractQihangM3U8URI(line string) string {
	for _, marker := range []string{`URI="`, `URI='`} {
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(marker):]
		if end := strings.IndexAny(rest, `"'`); end >= 0 {
			return rest[:end]
		}
		return rest
	}
	return ""
}

func replaceQihangM3U8URI(line, uri string) string {
	for _, marker := range []string{`URI="`, `URI='`} {
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(marker):]
		if end := strings.IndexAny(rest, `"'`); end >= 0 {
			return line[:idx+len(marker)] + uri + rest[end:]
		}
		return line[:idx+len(marker)] + uri
	}
	return line
}

func qihangM3U8InfoParam(raw string) string {
	normalized := strings.ReplaceAll(raw, "&amp;", "&")
	if u, err := url.Parse(normalized); err == nil {
		if info := u.Query().Get("info"); info != "" {
			return info
		}
	}
	return match1(normalized, `(?:[?&])info=(\w+)`)
}

func resolveQihangM3U8Line(baseURL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "0x") {
		return ref
	}
	if strings.HasPrefix(ref, "//") {
		return "https:" + ref
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ref
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(parsed).String()
}

func qihangM3U8DataURL(text string) string {
	return "data:application/vnd.apple.mpegurl;charset=utf-8," + url.PathEscape(text)
}

var qihangKeySeeds = decodeQihangKeySeeds()

func decodeQihangKeySeeds() [][]byte {
	encoded := []string{
		"Uglq1TA2pTi/QKOegfPX+3zjOYKbL/+HNI5DRMTe6ctUe5QypsIjPe5MlQtC+sNOCC6hZijZJLJ2W6JJbYvRJXL49mSGaJgW1KRczF1ltpJscEhQ/e252l4VRlenjZ2EkNirAIy80wr35FgFuLNFBtAsHo/KPw8Cwa+9AwETims6kRFBT2fc6pfyz87wtOZzlqx0IuetNYXi+TfoHHXfbkfxGnEdKcWJb7diDqoYvhv8Vj5LxtJ5IJrbwP54zVr0H92oM4gHxzGxEhBZJ4DsX2BRf6kZtUoNLeV6n5PJnO+g4DtNrir1sMjruzyDU5lhFysEfrp31ibhaRRjVSEMfQ==",
		"Y1UhDH1SCWrVMDalOL9Ao56B89f7fOM5gpsv/4c0jkNExN7py1R7lDKmwiM97kyVC0L6w04ILqFmKNkksnZboklti9Elcvj2ZIZomBbUpFzMXWW2kmxwSFD97bnaXhVGV6eNnYSQ2KsAjLzTCvfkWAW4s0UG0Cwej8o/DwLBr70DAROKazqREUFPZ9zql/LPzvC05nOWrHQi5601heL5N+gcdd9uR/EacR0pxYlvt2IOqhi+G/xWPkvG0nkgmtvA/njNWvQf3agziAfHMbESEFkngOxfYFF/qRm1Sg0t5Xqfk8mc76DgO02uKvWwyOu7PINTmWEXKwR+unfWJuFpFA==",
		"c5asdCLnrTWF4vk36Bx1325H8RpxHSnFiW+3Yg6qGL4b/FY+S8bSeSCa28D+eM1a9B/dqDOIB8cxsRIQWSeA7F9gUX+pGbVKDS3lep+TyZzvoOA7Ta4q9bDI67s8g1OZYRcrBH66d9Ym4WkUY1UhDH1SCWrVMDalOL9Ao56B89f7fOM5gpsv/4c0jkNExN7py1R7lDKmwiM97kyVC0L6w04ILqFmKNkksnZboklti9Elcvj2ZIZomBbUpFzMXWW2kmxwSFD97bnaXhVGV6eNnYSQ2KsAjLzTCvfkWAW4s0UG0Cwej8o/DwLBr70DAROKazqREUFPZ9zql/LPzvC05g==",
	}
	out := make([][]byte, 0, len(encoded))
	for _, item := range encoded {
		if decoded, err := base64.StdEncoding.DecodeString(item); err == nil {
			out = append(out, decoded)
		}
	}
	return out
}

func decryptQihangKey(encKey []byte, info string) string {
	if len(qihangKeySeeds) != 3 || len(encKey) < 21 || len(info) < 32 {
		return ""
	}
	seedIdx := int(encKey[0])
	if seedIdx < 0 || seedIdx >= len(qihangKeySeeds) {
		return ""
	}
	seed := qihangKeySeeds[seedIdx]
	if len(seed) < 256 {
		return ""
	}
	infoBytes := []byte(info)
	out := make([]byte, 20)
	for i := 0; i < 20; i++ {
		keyIndex := int(encKey[i+1]) ^ int(infoBytes[i%32])
		if keyIndex < 0 || keyIndex >= len(seed) {
			return ""
		}
		out[i] = seed[keyIndex]
	}
	return strings.ToUpper(hex.EncodeToString(out[:16]))
}

func qihangHeaders(j http.CookieJar) map[string]string {
	h := map[string]string{"Referer": referer}
	if token := cookieVal(j, []string{"https://www.iqihang.com/", "https://iqihang.com/"}, "accessToken"); token != "" {
		h["Authorization"] = "Bearer " + token
	}
	return h
}
func fetchUID(c *util.Client, h map[string]string) (string, error) {
	if h["Authorization"] == "" {
		return "", nil
	}
	body, err := c.GetString(user_info_url, h)
	if err != nil {
		return "", err
	}
	var resp struct {
		Data struct {
			ID any `json:"id"`
		} `json:"data"`
		ID any `json:"id"`
	}
	if json.Unmarshal([]byte(body), &resp) == nil {
		if id := first(jstr(resp.Data.ID), jstr(resp.ID)); id != "" {
			return id, nil
		}
	}
	if m := regexp.MustCompile(`"code"\s*:\s*0[\s\S]*?"id"\s*:\s*(\d+)`).FindStringSubmatch(body); len(m) > 1 {
		return m[1], nil
	}
	return "", nil
}

var (
	learnRe    = regexp.MustCompile(`/learn/(\d+)`)
	recordRe   = regexp.MustCompile(`/record/\d+/\d+/(\d+)`)
	playbackRe = regexp.MustCompile(`/playback/\d+/.*?/\d+/(\d+)`)
	catalogRe  = regexp.MustCompile(`/course/catalog/(\d+)`)
	productRe  = regexp.MustCompile(`[?&]courseId=(\d+)`)
)

func parseIDs(raw string) (cid, learnID, productID string) {
	if u, err := url.Parse(raw); err == nil {
		q := u.Query()
		cid = first(q.Get("productCurriculumId"), q.Get("curriculumId"), q.Get("cid"))
		learnID = first(q.Get("learnId"), q.Get("lid"), q.Get("id"))
		productID = first(q.Get("courseId"), q.Get("productId"))
	}
	if m := learnRe.FindStringSubmatch(raw); len(m) > 1 {
		learnID = m[1]
	}
	if m := recordRe.FindStringSubmatch(raw); len(m) > 1 {
		learnID = m[1]
	}
	if m := playbackRe.FindStringSubmatch(raw); len(m) > 1 {
		learnID = m[1]
	}
	if m := productRe.FindStringSubmatch(raw); len(m) > 1 {
		productID = m[1]
	}
	if m := catalogRe.FindStringSubmatch(raw); len(m) > 1 && cid == "" {
		cid = m[1]
	}
	return cid, learnID, productID
}
func replayArgs(raw string) (roomID, userID, recordID string) {
	u, err := url.Parse(strings.ReplaceAll(raw, "&amp;", "&"))
	if err == nil {
		q := u.Query()
		roomID, userID, recordID = q.Get("roomid"), q.Get("userid"), q.Get("recordid")
	}
	if roomID == "" {
		roomID = match1(raw, `roomid=(\w+)`)
	}
	if userID == "" {
		userID = match1(raw, `userid=(\w+)`)
	}
	if recordID == "" {
		recordID = match1(raw, `recordid=(\w+)`)
	}
	return
}
func cookieVal(j http.CookieJar, hosts []string, names ...string) string {
	if j == nil {
		return ""
	}
	for _, host := range hosts {
		u, _ := url.Parse(host)
		for _, ck := range j.Cookies(u) {
			for _, n := range names {
				if strings.EqualFold(ck.Name, n) {
					return strings.TrimSpace(ck.Value)
				}
			}
		}
	}
	return ""
}
func jstr(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" && v != "<nil>" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func match1(s, pat string) string {
	if m := regexp.MustCompile(pat).FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return ""
}
func joinInts(v []int) string {
	parts := make([]string, len(v))
	for i, n := range v {
		parts[i] = fmt.Sprint(n)
	}
	return strings.Join(parts, ".")
}

var badName = regexp.MustCompile(`[\\/:*?"<>|\r\n\t]+`)

func sanitize(s string) string {
	s = badName.ReplaceAllString(strings.TrimSpace(s), "_")
	if s == "" {
		return "未命名视频"
	}
	return s
}
func pickFormat(u string) string {
	lower := strings.ToLower(u)
	if strings.Contains(lower, ".m3u8") || strings.Contains(lower, "mpegurl") || strings.HasPrefix(strings.TrimSpace(u), "#EXTM3U") {
		return "m3u8"
	}
	return "mp4"
}
