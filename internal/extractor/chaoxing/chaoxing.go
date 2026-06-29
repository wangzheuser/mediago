package chaoxing

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

var patterns = []string{
	`chaoxing\.com`,
	`xueyinonline\.com`,
}

var (
	objectIDRe     = regexp.MustCompile(`(?i)(?:objectId|objectid)=([a-z0-9_-]+)`)
	objectIDPageRe = regexp.MustCompile(`(?i)(?:objectid|objectId)\s*[:=]\s*["']([a-z0-9_-]+)["']`)
	uuidRe         = regexp.MustCompile(`(?i)(?:\?|&|&amp;)(?:uuid|liveid)=([a-z0-9_-]{8,})`)
)

func init() {
	extractor.Register(&Chaoxing{}, extractor.SiteInfo{
		Name:     "Chaoxing",
		URL:      "chaoxing.com",
		NeedAuth: true,
	})
}

type Chaoxing struct{}

func (c *Chaoxing) Patterns() []string { return patterns }

func (c *Chaoxing) Extract(rawURL string, opts *extractor.ExtractOpts) (*extractor.MediaInfo, error) {
	if opts == nil || opts.Cookies == nil {
		return nil, fmt.Errorf("chaoxing requires login cookies (use --cookies or --cookies-from-browser)")
	}

	client := util.NewClient()
	client.SetCookieJar(opts.Cookies)
	ctx := newChaoxingContext(client, opts.Cookies, rawURL)

	if objectID := extractObjectID(rawURL); objectID != "" {
		entry, err := ctx.resolveObjectResource(chaoxingResource{Title: "chaoxing_video", Kind: "video", ObjectID: objectID, Ext: "mp4"})
		if err != nil {
			return nil, err
		}
		return entry, nil
	}
	if uuid := extractChaoxingUUID(rawURL); uuid != "" && strings.Contains(strings.ToLower(rawURL), "k.chaoxing.com/res/look") {
		if entry := ctx.resolveResource(chaoxingResource{Title: "chaoxing_review_" + uuid, Kind: "review", UUID: uuid}); entry != nil {
			return entry, nil
		}
	}
	if liveID := extractZhiboLiveID(rawURL); liveID != "" {
		if entry := ctx.resolveLiveResource(chaoxingResource{Title: "chaoxing_live_" + liveID, Kind: "live", LiveID: liveID}); entry != nil {
			return entry, nil
		}
	}

	course, pageObjectID, err := ctx.resolveCourse(rawURL)
	if err == nil && len(course.Entries) > 0 {
		return course, nil
	}
	if pageObjectID != "" {
		entry, derr := ctx.resolveObjectResource(chaoxingResource{Title: "chaoxing_video", Kind: "video", ObjectID: pageObjectID, Ext: "mp4"})
		if derr == nil {
			return entry, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("chaoxing: no playable course resources found")
}

func extractObjectID(raw string) string {
	if m := objectIDRe.FindStringSubmatch(raw); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractObjectIDFromPage(text string) string {
	if m := objectIDPageRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractChaoxingUUID(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		for _, key := range []string{"uuid", "liveid"} {
			if v := strings.TrimSpace(u.Query().Get(key)); v != "" {
				return v
			}
		}
	}
	if m := uuidRe.FindStringSubmatch(raw); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractZhiboLiveID(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || !strings.Contains(strings.ToLower(u.Host), "zhibo.chaoxing.com") {
		return ""
	}
	return strings.Trim(strings.TrimSpace(u.Path), "/")
}

const (
	defaultCourseHost = "https://mooc1.chaoxing.com"
	defaultNewHost    = "https://mooc2-ans.chaoxing.com"
	defaultPublicHost = "https://mooc1.xueyinonline.com"
	audioListURL      = "https://appswh.chaoxing.com/vclass/page/viewlist/data?uuid=%s"
	audioUpdateURL    = "https://appswh.chaoxing.com/vclass/page/update/data?pageId=%s&objectId=%s"
	meetReviewURL     = "https://k.chaoxing.com/apis/chapter/getMeetReview4Job?crossOrigin=true&uuid=%s"
	yunFileURL        = "https://k.chaoxing.com/apis/file/getYunFile?crossOrigin=true&objectId=%s&key="

	portalNewHeaderURL = "https://www.xueyinonline.com/portal/new-header?cur=1"
)

type chaoxingContext struct {
	c               *util.Client
	jar             http.CookieJar
	courseURL       string
	newCourseURL    string
	publicCourseURL string
	pathPrefix      string
	newCourse       bool
	courseID        string
	clazzID         string
	enc             string
	oldEnc          string
	cpi             string
	openc           string
	portalEnc       string
	portalCourseEnc string
	portalT         string
	downpath        string
	title           string
	headers         map[string]string
}

func newChaoxingContext(c *util.Client, jar http.CookieJar, rawURL string) *chaoxingContext {
	ctx := &chaoxingContext{
		c:               c,
		jar:             jar,
		courseURL:       defaultCourseHost,
		newCourseURL:    defaultNewHost,
		publicCourseURL: defaultPublicHost,
		downpath:        "https://cs-ans.chaoxing.com",
		headers: map[string]string{
			"Accept":  "text/html,application/xhtml+xml,application/xml;q=0.9,application/json,*/*;q=0.8",
			"Referer": defaultCourseHost + "/",
			"Origin":  defaultCourseHost,
		},
	}
	if u, err := url.Parse(rawURL); err == nil && u.Scheme != "" && u.Host != "" {
		host := strings.ToLower(u.Host)
		for _, marker := range []string{"/mycourse/stu", "/mycourse/studentcourse"} {
			if idx := strings.Index(u.Path, marker); idx >= 0 {
				ctx.pathPrefix = u.Path[:idx]
				break
			}
		}
		if ctx.pathPrefix != "" && strings.Contains(u.Path, "/course/") {
			ctx.pathPrefix = u.Path[:strings.Index(u.Path, "/course/")]
		}
		if strings.Contains(u.Path, "/mooc2-ans/") || strings.EqualFold(u.Query().Get("mooc2"), "1") {
			ctx.newCourse = true
		}
		if strings.Contains(host, "chaoxing.com") && !strings.Contains(host, "mooc2-ans") {
			ctx.courseURL = u.Scheme + "://" + u.Host
			ctx.headers["Referer"] = ctx.courseURL + "/"
			ctx.headers["Origin"] = ctx.courseURL
		}
		if strings.Contains(host, "mooc2-ans") {
			ctx.newCourseURL = u.Scheme + "://" + u.Host
			ctx.newCourse = true
		}
	}
	ctx.extractAccessFromURL(rawURL)
	return ctx
}

func (x *chaoxingContext) abs(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(x.courseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func (x *chaoxingContext) getString(rawURL string) (string, error) {
	return x.c.GetString(rawURL, x.headers)
}
