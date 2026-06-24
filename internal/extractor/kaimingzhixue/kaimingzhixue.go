// Package kaimingzhixue implements an extractor for lckmzx.com courses.
//
// API endpoints from decompiled Mooc/Courses/Kaimingzhixue/:
//
//	https://www.lckmzx.com/api/app/userInfo
//	https://www.lckmzx.com/api/app/myStudy/{course_type}
//	https://www.lckmzx.com/api/app/courseBasis
//	https://www.lckmzx.com/api/app/myStudy/course/{cid}
//	https://www.lckmzx.com/api/app/getPlayToken/chapter_id={chapter_id}/course_id={cid}
//	https://www.lckmzx.com/api/app/getPcRoomCode/course_id={cid}/chapter_id={chapter_id}
//	https://www.baijiayun.com/vod/video/getPlayUrl?vid={video_id}&render=jsonp&token={token}&use_encrypt=0
package kaimingzhixue

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/nichuanfang/medigo/internal/extractor"
	"github.com/nichuanfang/medigo/internal/extractor/shared"
	"github.com/nichuanfang/medigo/internal/util"
)

const (
	urlReferer          = "https://www.lckmzx.com"
	urlUserInfo         = "https://www.lckmzx.com/api/app/userInfo"
	urlCourseList       = "https://www.lckmzx.com/api/app/myStudy/%s"
	urlPublicCourse     = "https://www.lckmzx.com/api/app/courseBasis"
	urlDetail           = "https://www.lckmzx.com/api/app/myStudy/course/%s"
	urlPlayToken        = "https://www.lckmzx.com/api/app/getPlayToken/chapter_id=%s/course_id=%s"
	urlLiveRoomCode     = "https://www.lckmzx.com/api/app/getPcRoomCode/course_id=%s/chapter_id=%s"
	urlVideoPlay        = "https://www.baijiayun.com/vod/video/getPlayUrl?vid=%s&render=jsonp&token=%s&use_encrypt=0"
	urlBaijiayunReferer = "https://www.baijiayun.com"
)

var patterns = []string{`(?:[\w-]+\.)?lckmzx\.com/`}

func init() {
	extractor.Register(&Kaimingzhixue{}, extractor.SiteInfo{Name: "Kaimingzhixue", URL: "lckmzx.com", NeedAuth: true})
}

type Kaimingzhixue struct{}

func (s *Kaimingzhixue) Patterns() []string { return patterns }

var kzxIDRe = regexp.MustCompile(`(?i)(?:/(?:course|video|periods|classes)/(?:detail/)?|[?&](?:courseId|cid|id)=)(\d+)`)

func (s *Kaimingzhixue) Extract(rawURL string, opts *extractor.ExtractOpts) (*extractor.MediaInfo, error) {
	if opts == nil || opts.Cookies == nil {
		return nil, fmt.Errorf("kaimingzhixue requires login cookies")
	}
	c := util.NewClient()
	c.SetCookieJar(opts.Cookies)
	token := studentTokenFromJar(opts.Cookies)
	if token == "" {
		return nil, fmt.Errorf("kaimingzhixue requires studentToken cookie")
	}
	headers := kzxHeaders(opts.Cookies, token)
	if schoolID, err := checkKaimingLogin(c, headers); err != nil {
		return nil, err
	} else if schoolID != "" {
		headers["SchoolID"] = schoolID
	}

	cid := parseKaimingCID(rawURL)
	courses, err := fetchKaimingCourseList(c, headers)
	if err != nil {
		return nil, err
	}
	courseType, title := chooseKaimingCourse(courses, cid)
	if cid == "" && len(courses) == 1 {
		cid = courses[0].ID
		courseType = courses[0].CourseType
		title = courses[0].Title
	}
	if cid == "" {
		return nil, fmt.Errorf("cannot parse kaimingzhixue course id from URL: %s", rawURL)
	}

	detail, err := fetchKaimingDetail(c, cid, headers)
	if err != nil {
		return nil, err
	}
	if title == "" {
		title = firstText(findStringKey(detail, "title"), "kaimingzhixue_"+cid)
	}
	if courseType == "" {
		courseType = typeKey(firstText(findStringKey(detail, "course_type"), findStringKey(detail, "type")))
	}

	items := collectKaimingItems(detail, cid, courseType)
	entries := make([]*extractor.MediaInfo, 0, len(items))
	seen := map[string]bool{}
	for i, item := range items {
		key := item.Kind + ":" + item.ChapterID + ":" + item.VideoID + ":" + item.MeetingID
		if seen[key] {
			continue
		}
		seen[key] = true
		entry, err := buildKaimingEntry(c, headers, item, i+1)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("kaimingzhixue: no playable VOD/live entries for course=%s", cid)
	}
	return &extractor.MediaInfo{Site: "kaimingzhixue", Title: title, Entries: entries, Extra: map[string]any{"course_id": cid, "course_type": courseType}}, nil
}

func kzxHeaders(jar http.CookieJar, token string) map[string]string {
	cookie := cookieString(jar, "https", "www.lckmzx.com")
	if cookie == "" {
		cookie = "studentToken=" + token
	}
	return map[string]string{
		"SchoolID":      "2",
		"DeviceID":      newDeviceID(),
		"DeviceType":    "PC",
		"Content-Type":  "application/json",
		"Origin":        urlReferer,
		"Referer":       urlReferer,
		"cookie":        cookie,
		"Authorization": "Bearer " + token,
		"Accept":        "application/json, text/plain, */*",
	}
}

func checkKaimingLogin(c *util.Client, headers map[string]string) (string, error) {
	body, err := c.GetString(urlUserInfo, headers)
	if err != nil {
		return "", fmt.Errorf("kaimingzhixue userInfo: %w", err)
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			ID       any `json:"id"`
			SchoolID any `json:"school_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return "", fmt.Errorf("kaimingzhixue userInfo parse: %w", err)
	}
	if out.Code != 200 || firstText(out.Data.ID) == "" {
		return "", fmt.Errorf("kaimingzhixue requires valid logged-in studentToken (code=%d)", out.Code)
	}
	return firstText(out.Data.SchoolID), nil
}

type kzxEnvelope[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

func kzxAPIGet[T any](c *util.Client, apiURL string, params map[string]string, headers map[string]string) (T, error) {
	var zero T
	u, err := url.Parse(apiURL)
	if err != nil {
		return zero, err
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	body, err := c.GetString(u.String(), headers)
	if err != nil {
		return zero, err
	}
	var out kzxEnvelope[T]
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return zero, fmt.Errorf("kaimingzhixue parse %s: %w", u.String(), err)
	}
	if out.Code != 200 {
		return zero, fmt.Errorf("kaimingzhixue API code=%d msg=%s", out.Code, out.Msg)
	}
	return out.Data, nil
}

type kzxCourse struct {
	ID         string
	Title      string
	CourseType string
	Price      any
}

func fetchKaimingCourseList(c *util.Client, headers map[string]string) ([]kzxCourse, error) {
	var out []kzxCourse
	seen := map[string]bool{}
	for _, courseType := range []string{"1", "2", "3", "11"} {
		data, err := kzxAPIGet[any](c, fmt.Sprintf(urlCourseList, courseType), map[string]string{"type": "open"}, headers)
		if err != nil {
			return nil, fmt.Errorf("kaimingzhixue course list type=%s: %w", courseType, err)
		}
		for _, rec := range extractRecords(data) {
			id := firstText(mapLookup(rec, "course_id"), mapLookup(rec, "id"))
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, kzxCourse{ID: id, Title: firstText(mapLookup(rec, "title"), mapLookup(rec, "name")), CourseType: typeKey(firstText(mapLookup(rec, "course_type"), mapLookup(rec, "type"), courseType)), Price: mapLookup(rec, "price")})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("kaimingzhixue: empty myStudy course list")
	}
	return out, nil
}

func chooseKaimingCourse(courses []kzxCourse, cid string) (courseType, title string) {
	if cid == "" {
		return "", ""
	}
	for _, course := range courses {
		if course.ID == cid {
			return course.CourseType, course.Title
		}
	}
	return "", ""
}

func fetchKaimingDetail(c *util.Client, cid string, headers map[string]string) (map[string]any, error) {
	data, err := kzxAPIGet[any](c, fmt.Sprintf(urlDetail, url.PathEscape(cid)), nil, headers)
	if err != nil {
		return nil, fmt.Errorf("kaimingzhixue detail: %w", err)
	}
	if m, ok := data.(map[string]any); ok {
		return m, nil
	}
	return nil, fmt.Errorf("kaimingzhixue detail: data is not object")
}

type kzxItem struct {
	Kind       string
	CourseID   string
	ChapterID  string
	VideoID    string
	Title      string
	CourseType string
	PlayType   string
	PeriodID   string
	MeetingID  string
}

func collectKaimingItems(detail map[string]any, cid, selectedCourseType string) []kzxItem {
	var roots []any
	if v := detail["chapter"]; v != nil {
		roots = append(roots, v)
	}
	if v := detail["periods"]; v != nil {
		roots = append(roots, v)
	}
	if len(roots) == 0 {
		roots = append(roots, detail)
	}
	var items []kzxItem
	for _, root := range roots {
		walkKaimingNode(root, cid, selectedCourseType, nil, &items)
	}
	return items
}

func walkKaimingNode(v any, cid, selectedCourseType string, prefix []int, items *[]kzxItem) {
	switch x := v.(type) {
	case []any:
		for i, item := range x {
			walkKaimingNode(item, cid, selectedCourseType, append(prefix, i+1), items)
		}
	case map[string]any:
		chapterID := firstText(x["id"], x["course_chapter_id"], x["chapter_id"])
		videoID := firstText(x["video_id"], x["videoId"], x["vid"])
		nodeCourseType := typeKey(firstText(x["periods_type"], x["course_type"], selectedCourseType))
		title := firstText(x["periods_title"], x["title"], x["name"], x["chapter_name"], "未命名")
		if videoID != "" && chapterID != "" && isKaimingVOD(nodeCourseType) {
			*items = append(*items, kzxItem{Kind: "video", CourseID: cid, ChapterID: chapterID, VideoID: videoID, Title: formatIndexedTitle(prefix, title), CourseType: nodeCourseType})
		} else if chapterID != "" && isKaimingLiveNode(x, nodeCourseType) {
			*items = append(*items, kzxItem{Kind: "live_playback", CourseID: cid, ChapterID: chapterID, Title: formatIndexedTitle(prefix, title), CourseType: nodeCourseType, PlayType: firstText(x["type"], x["play_type"], "1"), PeriodID: firstText(x["periods_id"], x["bjy_period_id"]), MeetingID: firstText(x["meeting_id"])})
		}
		for _, key := range []string{"child", "children", "chapter", "periods", "list", "items"} {
			if child, ok := x[key]; ok {
				walkKaimingNode(child, cid, selectedCourseType, prefix, items)
			}
		}
	}
}

func isKaimingVOD(courseType string) bool {
	switch typeKey(courseType) {
	case "5", "8":
		return true
	default:
		return courseType == ""
	}
}

func isKaimingLiveNode(node map[string]any, courseType string) bool {
	if firstText(node["arrange_id"], node["meeting_id"], node["bjy_period_id"]) != "" {
		return true
	}
	switch typeKey(courseType) {
	case "2", "3", "4", "15", "16", "meeting":
		return true
	default:
		return false
	}
}

func buildKaimingEntry(c *util.Client, headers map[string]string, item kzxItem, index int) (*extractor.MediaInfo, error) {
	var mediaURL string
	var err error
	if item.Kind == "video" {
		mediaURL, err = resolveKaimingVOD(c, headers, item.CourseID, item.ChapterID, item.VideoID)
	} else {
		mediaURL, err = resolveKaimingLivePlayback(c, headers, item)
	}
	if err != nil || mediaURL == "" {
		if err == nil {
			err = fmt.Errorf("empty playback URL")
		}
		return nil, err
	}
	format := mediaExt(mediaURL)
	stream := extractor.Stream{Quality: "best", URLs: []string{mediaURL}, Format: format, Headers: map[string]string{"Referer": urlReferer}}
	if format == "m3u8" {
		stream.NeedMerge = true
	}
	return &extractor.MediaInfo{Site: "kaimingzhixue", Title: firstText(item.Title, fmt.Sprintf("[%d]--未命名", index)), Streams: map[string]extractor.Stream{"best": stream}, Extra: map[string]any{"course_id": item.CourseID, "chapter_id": item.ChapterID, "video_id": item.VideoID, "type": item.Kind}}, nil
}

func resolveKaimingVOD(c *util.Client, headers map[string]string, cid, chapterID, videoID string) (string, error) {
	data, err := kzxAPIGet[map[string]any](c, fmt.Sprintf(urlPlayToken, url.PathEscape(chapterID), url.PathEscape(cid)), nil, headers)
	if err != nil {
		return "", fmt.Errorf("kaimingzhixue play token: %w", err)
	}
	token := firstText(data["token"])
	vid := firstText(data["video_id"], videoID)
	if token == "" || vid == "" {
		return "", fmt.Errorf("kaimingzhixue play token missing token/video_id")
	}
	playURL, err := shared.BaijiayunResolveVOD(c, vid, token, map[string]string{"Referer": urlBaijiayunReferer})
	if err != nil {
		return "", err
	}
	return playURL, nil
}

func resolveKaimingLivePlayback(c *util.Client, headers map[string]string, item kzxItem) (string, error) {
	params := map[string]string{"type": "2"}
	if item.MeetingID != "" {
		params["meeting_id"] = item.MeetingID
	}
	if typeKey(item.CourseType) == "4" {
		params["repeat_times"] = "1"
	}
	data, err := kzxAPIGet[any](c, fmt.Sprintf(urlLiveRoomCode, url.PathEscape(item.CourseID), url.PathEscape(item.ChapterID)), params, headers)
	if err != nil {
		return "", fmt.Errorf("kaimingzhixue live room code: %w", err)
	}
	room := data
	if m, ok := data.(map[string]any); ok {
		if ci, ok := m["chapterInfo"].(map[string]any); ok {
			room = ci
		}
	}
	if direct := findPlayableURL(room); direct != "" {
		if vid, token, isVOD := parseBaijiayunQuery(direct); token != "" {
			if isVOD {
				return shared.BaijiayunResolveVOD(c, vid, token, map[string]string{"Referer": urlBaijiayunReferer})
			}
			return shared.BaijiayunResolvePlayback(c, vid, token, map[string]string{"Referer": urlBaijiayunReferer})
		}
		return direct, nil
	}
	roomID := firstText(findStringKey(room, "room_id"), findStringKey(room, "classid"), findStringKey(room, "roomId"), findStringKey(room, "bjy_room_id"))
	token := firstText(findStringKey(room, "token"), findStringKey(room, "playback_token"))
	vid := firstText(findStringKey(room, "vid"), findStringKey(room, "video_id"))
	if vid != "" && token != "" {
		return shared.BaijiayunResolveVOD(c, vid, token, map[string]string{"Referer": urlBaijiayunReferer})
	}
	if roomID != "" && token != "" {
		return shared.BaijiayunResolvePlayback(c, roomID, token, map[string]string{"Referer": urlBaijiayunReferer})
	}
	return "", fmt.Errorf("kaimingzhixue live playback missing room/token")
}

func parseBaijiayunQuery(raw string) (id, token string, isVOD bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", false
	}
	q := u.Query()
	token = q.Get("token")
	if vid := q.Get("vid"); vid != "" && token != "" {
		return vid, token, true
	}
	for _, key := range []string{"room_id", "classid"} {
		if roomID := q.Get(key); roomID != "" && token != "" {
			return roomID, token, false
		}
	}
	return "", token, false
}

func parseKaimingCID(rawURL string) string {
	if m := kzxIDRe.FindStringSubmatch(rawURL); len(m) > 1 {
		return m[1]
	}
	if u, err := url.Parse(rawURL); err == nil {
		q := u.Query()
		return firstText(q.Get("cid"), q.Get("courseId"), q.Get("id"))
	}
	return ""
}

func extractRecords(v any) []map[string]any {
	switch x := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, item := range x {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"list", "records", "rows", "items", "data"} {
			if recs := extractRecords(x[key]); len(recs) > 0 {
				return recs
			}
		}
	}
	return nil
}

func mapLookup(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		return v
	}
	return nil
}

func findStringKey(v any, key string) string {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if strings.EqualFold(k, key) {
				if s := firstText(val); s != "" {
					return s
				}
			}
			if s := findStringKey(val, key); s != "" {
				return s
			}
		}
	case []any:
		for _, item := range x {
			if s := findStringKey(item, key); s != "" {
				return s
			}
		}
	}
	return ""
}

func findPlayableURL(v any) string {
	switch x := v.(type) {
	case string:
		u := normalizeURL(x)
		lu := strings.ToLower(u)
		if strings.HasPrefix(u, "http") && (strings.Contains(lu, ".mp4") || strings.Contains(lu, ".m3u8") || strings.Contains(lu, ".ev1") || strings.Contains(lu, ".ev2") || strings.Contains(lu, "baijiayun")) {
			return u
		}
	case map[string]any:
		for _, key := range []string{"playback_url", "video_url", "url", "playUrl", "m3u8", "m3u8Url", "file_url", "fileUrl"} {
			if u := findPlayableURL(x[key]); u != "" {
				return u
			}
		}
		for _, item := range x {
			if u := findPlayableURL(item); u != "" {
				return u
			}
		}
	case []any:
		for _, item := range x {
			if u := findPlayableURL(item); u != "" {
				return u
			}
		}
	}
	return ""
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	return raw
}

func studentTokenFromJar(jar http.CookieJar) string {
	for _, host := range []string{"www.lckmzx.com", "lckmzx.com"} {
		for _, ck := range jar.Cookies(&url.URL{Scheme: "https", Host: host}) {
			if ck.Name == "studentToken" && ck.Value != "" {
				return ck.Value
			}
		}
	}
	return ""
}

func cookieString(jar http.CookieJar, scheme, host string) string {
	cookies := jar.Cookies(&url.URL{Scheme: scheme, Host: host})
	parts := make([]string, 0, len(cookies))
	for _, ck := range cookies {
		if ck.Value != "" {
			parts = append(parts, ck.Name+"="+ck.Value)
		}
	}
	return strings.Join(parts, "; ")
}

func newDeviceID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strings.ToUpper(hex.EncodeToString([]byte(strconv.FormatInt(int64(len(buf)), 10))))
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	hexed := strings.ToUpper(hex.EncodeToString(buf))
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexed[:8], hexed[8:12], hexed[12:16], hexed[16:20], hexed[20:])
}

func typeKey(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

func formatIndexedTitle(prefix []int, title string) string {
	if len(prefix) == 0 {
		return title
	}
	parts := make([]string, len(prefix))
	for i, n := range prefix {
		parts[i] = strconv.Itoa(n)
	}
	return fmt.Sprintf("[%s]--%s", strings.Join(parts, "."), title)
}

func mediaExt(u string) string {
	lu := strings.ToLower(u)
	switch {
	case strings.Contains(lu, ".m3u8"):
		return "m3u8"
	case strings.Contains(lu, ".flv"):
		return "flv"
	case strings.Contains(lu, ".mp3"):
		return "mp3"
	default:
		return "mp4"
	}
}

func firstText(values ...any) string {
	for _, v := range values {
		if s := stringValue(v); s != "" {
			return s
		}
	}
	return ""
}

func stringValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return strings.TrimSpace(x.String())
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		return strings.TrimSpace(fmt.Sprint(x))
	default:
		return ""
	}
}
