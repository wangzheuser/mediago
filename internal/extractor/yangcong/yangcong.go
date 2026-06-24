// Package yangcong implements an extractor for yangcongxueyuan.com / yangcong345.com.
package yangcong

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/nichuanfang/medigo/internal/extractor"
	"github.com/nichuanfang/medigo/internal/util"
)

const (
	refererURL        = "https://school.yangcongxueyuan.com/"
	originURL         = "https://school.yangcongxueyuan.com"
	apiHost           = "https://school-api.yangcong345.com"
	subjectsURL       = apiHost + "/course/subjects"
	chaptersURL       = apiHost + "/course/chapters-with-section/scene"
	specialCoursesURL = apiHost + "/course-tree/special-courses"
	specialCourseURL  = apiHost + "/course/special-course/%s"
	topicDetailsURL   = apiHost + "/course-business/courseTree/getAnyTopicDetailsByIds"
	videoAddressesURL = apiHost + "/videos/addresses"
	orderAuthURL      = apiHost + "/user-auths/order/auth"
	meURL             = apiHost + "/me"
)

var (
	patterns     = []string{`(?:[\w-]+\.)?(?:yangcong345|yangcongxueyuan)\.com/`}
	specialIDRe  = regexp.MustCompile(`(?:special-course/|special-)([\w-]+)`)
	titleCleanRe = regexp.MustCompile(`[\\/:*?"<>|\r\n\t]+`)
)

func init() {
	extractor.Register(&Yangcong{}, extractor.SiteInfo{Name: "Yangcong", URL: "yangcongxueyuan.com", NeedAuth: true})
}

type Yangcong struct{}

func (y *Yangcong) Patterns() []string { return patterns }

type courseRequest struct {
	subjectID, stageID, publisherID, semesterID, semesterName string
	specialID, title                                          string
}

type ycVideo struct {
	VideoID string
	TopicID string
	Title   string
	Path    string
}

func (y *Yangcong) Extract(rawURL string, opts *extractor.ExtractOpts) (*extractor.MediaInfo, error) {
	if opts == nil || opts.Cookies == nil {
		return nil, fmt.Errorf("yangcong requires login cookies")
	}
	req := parseCourseRequest(rawURL)
	c := util.NewClient()
	c.SetCookieJar(opts.Cookies)
	headers := buildHeaders(opts.Cookies)
	if err := checkCookie(c, headers); err != nil {
		return nil, err
	}
	_, _ = getJSON(c, subjectsURL, headers)  // source warms course subject list
	_, _ = getJSON(c, orderAuthURL, headers) // source loads order/auth before price

	root, title, err := fetchCoursePayload(c, headers, req)
	if err != nil {
		return nil, err
	}
	if title == "" {
		title = firstNonEmpty(req.title, req.specialID, req.semesterName, "yangcong")
	}
	videos := collectVideos(root)
	if len(videos) == 0 {
		return nil, fmt.Errorf("yangcong: no video topics found")
	}
	entries := make([]*extractor.MediaInfo, 0, len(videos))
	seen := map[string]bool{}
	for _, v := range videos {
		if v.VideoID == "" || seen[v.VideoID+":"+v.TopicID] {
			continue
		}
		seen[v.VideoID+":"+v.TopicID] = true
		entry, err := resolveVideo(c, headers, v)
		if err == nil {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("yangcong: no playable video address resolved")
	}
	return &extractor.MediaInfo{Site: "yangcong", Title: cleanTitle(title), Entries: entries}, nil
}

func parseCourseRequest(raw string) courseRequest {
	u, _ := url.Parse(raw)
	q := url.Values{}
	if u != nil {
		for k, vs := range u.Query() {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		if strings.Contains(u.Fragment, "?") {
			if fq, err := url.ParseQuery(strings.SplitN(u.Fragment, "?", 2)[1]); err == nil {
				for k, vs := range fq {
					for _, v := range vs {
						q.Add(k, v)
					}
				}
			}
		}
	}
	get := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(q.Get(k)); v != "" {
				return v
			}
		}
		return ""
	}
	r := courseRequest{subjectID: get("subjectId", "subject_id"), stageID: get("stageId", "stage_id"), publisherID: get("publisherId", "publisher_id"), semesterID: get("semesterId", "semester_id"), semesterName: get("semesterName", "semester_name"), specialID: get("specialCourseId", "special_course_id", "cid", "id"), title: get("title", "name")}
	if m := specialIDRe.FindStringSubmatch(raw); len(m) > 1 {
		r.specialID = m[1]
	}
	if strings.Contains(raw, "courseType=special") || strings.Contains(raw, "course_type=special") {
		return r
	}
	if r.subjectID != "" && r.stageID != "" {
		r.specialID = ""
	}
	return r
}

func buildHeaders(jar http.CookieJar) map[string]string {
	h := map[string]string{"Accept": "application/json, text/plain, */*", "Origin": originURL, "Referer": refererURL}
	for _, raw := range []string{refererURL, apiHost + "/"} {
		u, _ := url.Parse(raw)
		for _, ck := range jar.Cookies(u) {
			if strings.EqualFold(ck.Name, "authorization") || strings.EqualFold(ck.Name, "token") {
				h["authorization"] = normalizeAuth(ck.Value)
			}
		}
	}
	return h
}

func normalizeAuth(v string) string {
	v = strings.TrimSpace(v)
	if v != "" && !strings.HasPrefix(strings.ToLower(v), "bearer ") {
		return "Bearer " + v
	}
	return v
}

func checkCookie(c *util.Client, headers map[string]string) error {
	body, err := c.GetString(meURL, headers)
	if err != nil {
		return fmt.Errorf("yangcong me: %w", err)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return fmt.Errorf("yangcong me parse: %w", err)
	}
	if firstString(data, "id") == "" || firstString(data, "role") == "" {
		return fmt.Errorf("yangcong login check failed")
	}
	return nil
}

func fetchCoursePayload(c *util.Client, headers map[string]string, req courseRequest) (map[string]any, string, error) {
	if req.specialID != "" {
		data, err := getJSON(c, fmt.Sprintf(specialCourseURL, url.PathEscape(req.specialID)), headers)
		if err != nil {
			return nil, "", err
		}
		return data, firstString(data, "name", "title"), nil
	}
	if req.subjectID == "" || req.stageID == "" || req.publisherID == "" || req.semesterID == "" {
		return nil, "", fmt.Errorf("yangcong: subjectId/stageId/publisherId/semesterId are required for sync course URLs")
	}
	q := url.Values{"filterPublished": {"false"}, "subjectId": {req.subjectID}, "stageId": {req.stageID}, "publisherId": {req.publisherID}, "semesterId": {req.semesterID}}
	if req.semesterName != "" {
		q.Set("semesterName", req.semesterName)
	}
	data, err := getJSON(c, chaptersURL+"?"+q.Encode(), headers)
	if err != nil {
		return nil, "", err
	}
	book := asMap(data["defaultBook"])
	return data, firstNonEmpty(firstString(book, "name", "title"), req.title), nil
}

func getJSON(c *util.Client, apiURL string, headers map[string]string) (map[string]any, error) {
	body, err := c.GetString(apiURL, headers)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		var arr []any
		if e := json.Unmarshal([]byte(body), &arr); e != nil {
			return nil, err
		}
		data = map[string]any{"list": arr}
	}
	return data, nil
}

func postJSON(c *util.Client, apiURL string, payload any, headers map[string]string) (map[string]any, error) {
	b, _ := json.Marshal(payload)
	h := map[string]string{"Content-Type": "application/json"}
	for k, v := range headers {
		h[k] = v
	}
	resp, err := c.Post(apiURL, strings.NewReader(string(b)), h)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func collectVideos(root map[string]any) []ycVideo {
	var out []ycVideo
	var walk func(any, []string)
	walk = func(v any, path []string) {
		m := asMap(v)
		if len(m) > 0 {
			name := cleanTitle(firstString(m, "name", "title"))
			if name != "" {
				path = append(path, name)
			}
			if vid := videoID(m); vid != "" {
				out = append(out, ycVideo{VideoID: vid, TopicID: firstString(m, "id", "topic_id", "topicId"), Title: firstNonEmpty(name, vid), Path: strings.Join(path, " / ")})
			}
			for _, k := range []string{"children", "childrens", "levels", "sections", "subsections", "themes", "topics", "items", "list"} {
				walk(m[k], path)
			}
			return
		}
		if arr, ok := v.([]any); ok {
			for _, it := range arr {
				walk(it, append([]string{}, path...))
			}
		}
	}
	walk(root, nil)
	return out
}

func videoID(m map[string]any) string {
	if v := firstString(m, "videoId", "video_id"); v != "" {
		return v
	}
	return firstString(asMap(m["video"]), "id", "videoId", "video_id")
}

func resolveVideo(c *util.Client, headers map[string]string, v ycVideo) (*extractor.MediaInfo, error) {
	payload := map[string]any{"videoList": []map[string]any{{"refinedExerciseId": v.TopicID, "topicId": v.TopicID, "videoId": v.VideoID, "custom": map[string]string{"videoId": v.VideoID}}}}
	resp, err := postJSON(c, videoAddressesURL, payload, headers)
	if err != nil {
		return nil, fmt.Errorf("yangcong video address: %w", err)
	}
	addr := pickAddress(resp)
	if addr == "" {
		return nil, fmt.Errorf("yangcong: no address for video %s", v.VideoID)
	}
	return &extractor.MediaInfo{Site: "yangcong", Title: cleanTitle(firstNonEmpty(v.Path, v.Title, v.VideoID)), Streams: map[string]extractor.Stream{"default": {Quality: "best", URLs: []string{addr}, Format: pickFormat(addr), Headers: map[string]string{"Referer": refererURL}}}, Extra: map[string]any{"video_id": v.VideoID, "topic_id": v.TopicID}}, nil
}

func pickAddress(v any) string {
	bestURL, bestScore := "", -1
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case []any:
			for _, it := range t {
				walk(it)
			}
		case map[string]any:
			if u := firstString(t, "url"); strings.HasPrefix(u, "http") && !strings.HasSuffix(u, ".ycm") {
				score := qualityScore(firstString(t, "format"), firstString(t, "clarity"), firstString(t, "platform"))
				if score > bestScore {
					bestURL, bestScore = u, score
				}
			}
			for _, k := range []string{"address", "data", "list", "videoList", "videos"} {
				walk(t[k])
			}
		}
	}
	walk(v)
	return bestURL
}

func qualityScore(vals ...string) int {
	score := 0
	joined := strings.ToLower(strings.Join(vals, " "))
	for i, key := range []string{"low", "middle", "high", "fullhigh"} {
		if strings.Contains(joined, key) {
			score = i + 1
		}
	}
	if strings.Contains(joined, "mp4") {
		score += 10
	} else if strings.Contains(joined, "hls") {
		score += 5
	}
	if strings.Contains(joined, "pc") {
		score++
	}
	return score
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := strings.TrimSpace(fmt.Sprint(m[k])); s != "" && s != "<nil>" {
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
func cleanTitle(s string) string { return titleCleanRe.ReplaceAllString(strings.TrimSpace(s), "_") }
func pickFormat(u string) string {
	if strings.Contains(strings.ToLower(u), ".m3u8") {
		return "m3u8"
	}
	return "mp4"
}
