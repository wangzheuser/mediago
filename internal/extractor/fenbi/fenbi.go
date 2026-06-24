// Package fenbi implements the ke.fenbi.com lecture / episode extractor.
package fenbi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"

	"github.com/nichuanfang/medigo/internal/extractor"
	"github.com/nichuanfang/medigo/internal/util"
)

const (
	referer                          = "https://pc.fenbi.com/pwa"
	origin                           = "https://pc.fenbi.com"
	login_check_url                  = "https://login.fenbi.com/api/users/current"
	ke_check_url                     = "https://ke.fenbi.com/win/v3/users/current?nickname=true"
	course_list_url                  = "https://ke.fenbi.com/win/v3/courses"
	visible_lectures_url             = "https://ke.fenbi.com/win/%s/v3/my/lectures/visible?start=%s&len=%s"
	lecture_detail_url               = "https://ke.fenbi.com/win/%s/v3/lectures/%s"
	lecture_detail_api_url           = "https://ke.fenbi.com/api/%s/v3/lectures/%s"
	lecture_summary_url              = "https://ke.fenbi.com/win/%s/v3/my/lectures/%s/summary"
	lecture_set_contents_url         = "https://ke.fenbi.com/win/%s/v3/lecturesets/%s/contents?start=%s&len=%s"
	lecture_episode_nodes_url        = "https://ke.fenbi.com/win/%s/v3/lectures/%s/episode_nodes"
	my_lecture_episode_set_nodes_url = "https://ke.fenbi.com/win/%s/v3/my/lectures/%s/episode_sets/%s/episode_nodes"
	episode_detail_url               = "https://ke.fenbi.com/win/%s/v3/episodes/%s"
	episode_detail_api_url           = "https://ke.fenbi.com/api/%s/v3/episodes/%s"
	media_meta_url                   = "https://ke.fenbi.com/win/%s/v3/episodes/%s/mediafile/meta"
	material_url                     = "https://live.fenbi.com/win/%s/v3/livereplay/materials/%s/path"
	vertical_material_url            = "https://ke.fenbi.com/win/v3/vertical_feed/material/path"
	page_size                        = 200
)

var patterns = []string{`(?:pc|ke|live|www)\.fenbi\.com/|(?:^|\s)(?:fenbi|粉笔)(?:\s|$)`}

func init() {
	extractor.Register(&Fenbi{}, extractor.SiteInfo{Name: "Fenbi", URL: "fenbi.com", NeedAuth: true})
}

type Fenbi struct{}

func (f *Fenbi) Patterns() []string { return patterns }

var (
	winLectureRe = regexp.MustCompile(`(?i)/(?:win|api)/([A-Za-z0-9_]+)/v3/lectures/(\d+)`)
	winEpisodeRe = regexp.MustCompile(`(?i)/(?:win|api)/([A-Za-z0-9_]+)/v3/episodes/(\d+)`)
	prefixRe     = regexp.MustCompile(`(?i)/(?:win|api)/([A-Za-z0-9_]+)/v3/`)
	lectureRe    = regexp.MustCompile(`(?i)(?:lecture_id|lectureId|lectures?|biz_id|content_id)[=/](\d+)|[?&](?:lecture_id|biz_id|content_id)=(\d+)`)
	episodeRe    = regexp.MustCompile(`(?i)(?:episode_id|episodeId|episodes?)[=/](\d+)|[?&]episode_id=(\d+)`)
)

type requestIDs struct {
	Prefix    string
	LectureID string
	EpisodeID string
}

type episodeNode struct {
	ID    string
	Title string
	Raw   map[string]any
}

func (f *Fenbi) Extract(rawURL string, opts *extractor.ExtractOpts) (*extractor.MediaInfo, error) {
	if opts == nil || opts.Cookies == nil {
		return nil, fmt.Errorf("fenbi requires login cookies (use --cookies or --cookies-from-browser)")
	}
	ids := parseIDs(rawURL)
	if ids.Prefix == "" {
		ids.Prefix = "gwy"
	}
	if ids.LectureID == "" && ids.EpisodeID == "" {
		return nil, fmt.Errorf("cannot parse fenbi lecture/episode id from URL: %s", rawURL)
	}

	c := util.NewClient()
	c.SetCookieJar(opts.Cookies)
	headers := map[string]string{
		"Accept":           "application/json, text/plain, */*",
		"Origin":           origin,
		"Referer":          referer,
		"X-Requested-With": "XMLHttpRequest",
	}
	_ = checkLogin(c, headers)

	if ids.EpisodeID != "" && ids.LectureID == "" {
		entry, err := resolveEpisode(c, headers, ids.Prefix, ids.EpisodeID, "fenbi_"+ids.EpisodeID)
		if err != nil {
			return nil, err
		}
		return entry, nil
	}

	entries, title, err := resolveLecture(c, headers, ids)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("fenbi: no playable episodes found for lecture %s", ids.LectureID)
	}
	return &extractor.MediaInfo{Site: "fenbi", Title: util.SanitizeFilename(firstNonEmpty(title, "fenbi_"+ids.LectureID)), Entries: entries}, nil
}

func checkLogin(c *util.Client, headers map[string]string) error {
	for _, api := range []string{login_check_url, ke_check_url} {
		body, err := c.GetString(api, headers)
		if err != nil {
			continue
		}
		var payload any
		if json.Unmarshal([]byte(body), &payload) == nil {
			return nil
		}
	}
	return nil
}

func resolveLecture(c *util.Client, headers map[string]string, ids requestIDs) ([]*extractor.MediaInfo, string, error) {
	payloads := []any{}
	for _, api := range []string{
		fmt.Sprintf(lecture_detail_url, url.PathEscape(ids.Prefix), url.PathEscape(ids.LectureID)),
		fmt.Sprintf(lecture_detail_api_url, url.PathEscape(ids.Prefix), url.PathEscape(ids.LectureID)),
		fmt.Sprintf(lecture_summary_url, url.PathEscape(ids.Prefix), url.PathEscape(ids.LectureID)),
		fmt.Sprintf(lecture_episode_nodes_url, url.PathEscape(ids.Prefix), url.PathEscape(ids.LectureID)),
	} {
		if p, err := requestJSON(c, api, headers); err == nil {
			payloads = append(payloads, p)
		}
	}
	var nodes []episodeNode
	var title string
	var direct []*extractor.MediaInfo
	for _, payload := range payloads {
		title = firstNonEmpty(pickTitle(payload), title)
		if media := findMediaURL(payload); media != "" {
			direct = append(direct, mediaInfo(firstNonEmpty(pickTitle(payload), ids.LectureID), media, headers))
		}
		nodes = append(nodes, collectEpisodes(payload)...)
	}
	if ids.EpisodeID != "" {
		nodes = append([]episodeNode{{ID: ids.EpisodeID, Title: title}}, nodes...)
	}
	if len(nodes) == 0 && len(direct) > 0 {
		return direct, title, nil
	}
	seen := map[string]bool{}
	entries := make([]*extractor.MediaInfo, 0, len(nodes))
	for _, node := range nodes {
		if node.ID == "" || seen[node.ID] {
			continue
		}
		seen[node.ID] = true
		entry, err := resolveEpisode(c, headers, ids.Prefix, node.ID, node.Title)
		if err == nil {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 && len(direct) > 0 {
		return direct, title, nil
	}
	return entries, title, nil
}

func resolveEpisode(c *util.Client, headers map[string]string, prefix, episodeID, fallbackTitle string) (*extractor.MediaInfo, error) {
	payloads := []any{}
	for _, api := range []string{
		fmt.Sprintf(episode_detail_url, url.PathEscape(prefix), url.PathEscape(episodeID)),
		fmt.Sprintf(episode_detail_api_url, url.PathEscape(prefix), url.PathEscape(episodeID)),
		fmt.Sprintf(media_meta_url, url.PathEscape(prefix), url.PathEscape(episodeID)),
	} {
		if p, err := requestJSON(c, api, headers); err == nil {
			payloads = append(payloads, p)
		}
	}
	for _, payload := range payloads {
		if media := findMediaURL(payload); media != "" {
			title := util.SanitizeFilename(firstNonEmpty(pickTitle(payload), fallbackTitle, episodeID))
			return mediaInfo(title, media, headers), nil
		}
	}
	return nil, fmt.Errorf("fenbi: no media meta URL for episode %s", episodeID)
}

func requestJSON(c *util.Client, api string, headers map[string]string) (any, error) {
	body, err := c.GetString(api, headers)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func parseIDs(raw string) requestIDs {
	out := requestIDs{Prefix: "gwy"}
	if u, err := url.Parse(raw); err == nil {
		q := u.Query()
		out.Prefix = firstNonEmpty(q.Get("prefix"), out.Prefix)
		out.LectureID = firstNonEmpty(q.Get("lecture_id"), q.Get("lectureId"), q.Get("biz_id"), q.Get("content_id"))
		out.EpisodeID = firstNonEmpty(q.Get("episode_id"), q.Get("episodeId"))
	}
	if m := winLectureRe.FindStringSubmatch(raw); m != nil {
		out.Prefix, out.LectureID = m[1], m[2]
	}
	if m := winEpisodeRe.FindStringSubmatch(raw); m != nil {
		out.Prefix, out.EpisodeID = m[1], m[2]
	}
	if m := prefixRe.FindStringSubmatch(raw); m != nil && out.Prefix == "gwy" {
		out.Prefix = m[1]
	}
	out.LectureID = firstNonEmpty(out.LectureID, rx(lectureRe, raw))
	out.EpisodeID = firstNonEmpty(out.EpisodeID, rx(episodeRe, raw))
	return out
}

func rx(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	for i := 1; i < len(m); i++ {
		if m[i] != "" {
			return m[i]
		}
	}
	return ""
}
