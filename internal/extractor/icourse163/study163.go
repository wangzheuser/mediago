package icourse163

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

// Study163 / 网易云课堂 flows ported from the restored Study163_* Python
// sources. The package name stays icourse163 because Mooc163 is mapped to this
// existing Go extractor directory.
const (
	studyReferer = "https://study.163.com"
	studyMainURL = "https://study.163.com/course/courseMain.htm?courseId=%s"
	studyListURL = "https://study.163.com/j/my/courseListV2.json?pageSize=99&pageIndex=%d&filterType=1"

	studyPlanURL   = "https://study.163.com/dwr/call/plaincall/PlanNewBean.getPlanCourseDetail.dwr"
	studyVideoURL  = "https://study.163.com/dwr/call/plaincall/LessonLearnBean.getVideoLearnInfo.dwr"
	studyTextURL   = "https://study.163.com/dwr/call/plaincall/LessonLearnBean.getTextLearnInfo.dwr"
	studyAudioURL  = "https://study.163.com/dwr/call/plaincall/LessonLearnBean.getAudioLearnInfo.dwr"
	studyAttachURL = "https://study.163.com/dwr/call/plaincall/LessonReferenceBean.getLessonReferenceVoByLessonId.dwr"

	studyMoocCourseURL = "https://mooc.study.163.com/course/%s"
	studyMoocInfoURL   = "https://mooc.study.163.com/dwr/call/plaincall/CourseBean.getLastLearnedMocTermDto.dwr"
	studyMoocStartURL  = "https://mooc.study.163.com/dwr/call/plaincall/CourseBean.startTermLearn.dwr"
	studyMoocJoinedURL = "https://mooc.study.163.com/dwr/call/plaincall/CourseBean.checkTermLearn.dwr"
	studyMoocParseURL  = "https://mooc.study.163.com/dwr/call/plaincall/CourseBean.getLessonUnitLearnVo.dwr"
	studyMoocAttachURL = "https://mooc.study.163.com/course/attachment.htm"
	studyMoocPriceURL  = "https://cps.study.163.com/j/cpsShare/getSharePosterInfo.json"

	studyCompositeURL      = "https://course.study.163.com/%s"
	studyCompositeTermsURL = "https://course.study.163.com/j/cp/getCompositeRelList.json"
	studyCompositeListURL  = "https://course.study.163.com/j/cp/lecture/front/getList.json"
	studyCompositeParseURL = "https://course.study.163.com/p/cp/lecture/front/getLectureResource.json"
	studyCompositePriceURL = "https://course.study.163.com/j/cp/getTermIndexVos.json"

	studySpecURL      = "https://mooc.study.163.com/smartSpec/detail/%s.htm"
	studySpecLearnURL = "https://course.study.163.com/%s/learning"
	studySpecPriceURL = "https://mooc.study.163.com/smartSpec/price.htm?specId=%s"

	kaoyanPackageURL  = "https://kaoyan.icourse163.org/course/packages/%s.htm"
	kaoyanPackageInfo = "https://kaoyan.icourse163.org/web/j/kaoyanCourseBean.getMocCoursePackageIncludeTerm.rpc?csrfKey="
)

var study163Patterns = []string{
	`(?:[\w-]+\.)?study\.163\.com/(?:course/(?:courseMain|introduction)|j/my/courseListV2|mycourse|.*?[?&]course[Ii]d=\d+)`,
	`mooc\.study\.163\.com/(?:course|learn|smartSpec/detail)/`,
	`course\.study\.163\.com/\d+(?:/learning)?`,
	`ke\.study\.163\.com/.*?(?:detail/\d+|course[Ii]d=\d+)`,
	`kaoyan\.icourse163\.org/course/packages/\d+\.htm`,
}

var (
	studyCourseIDRe     = regexp.MustCompile(`(?i)(?:courseId|course_id)=([0-9]+)|/course/(?:courseMain\.htm\?courseId=|introduction/)?([0-9]+)(?:\.htm)?`)
	studyMoocURLRe      = regexp.MustCompile(`(?i)^https?://mooc\.study\.163\.com/(?:course|learn)/(?P<cid>\d+)(?:.*?[?&]tid=(?P<tid>\d+))?`)
	studyCompositeURLRe = regexp.MustCompile(`(?i)^https?://course\.study\.163\.com/(?P<cid>\d+)`)
	studySpecURLRe      = regexp.MustCompile(`(?i)^https?://mooc\.study\.163\.com/smartSpec/detail/(?P<sid>\d+)\.htm|^https?://course\.study\.163\.com/(?P<cid>\d+)(?:/learning)?`)
	kaoyanPackageRe     = regexp.MustCompile(`(?i)^https?://kaoyan\.icourse163\.org/course/packages/(?P<pid>\d+)\.htm`)
)

func init() {
	extractor.Register(&Study163{}, extractor.SiteInfo{Name: "study163", URL: "study.163.com", NeedAuth: true})
}

type Study163 struct{}

func (s *Study163) Patterns() []string { return study163Patterns }

type studyEntry struct {
	title, url, format, quality string
	size                        int64
	extra                       map[string]any
}

type studySource struct {
	id, contentID, contentType, name string
}

func (s *Study163) Extract(rawURL string, opts *extractor.ExtractOpts) (*extractor.MediaInfo, error) {
	if opts == nil || opts.Cookies == nil {
		return nil, fmt.Errorf("study163 requires login cookies (use --cookies or --cookies-from-browser)")
	}
	c := util.NewClient()
	c.SetCookieJar(opts.Cookies)

	if _, ok := parseYoudaoURL(rawURL); ok || strings.Contains(rawURL, "ke.study.163.com") || strings.Contains(rawURL, "ke.study.youdao.com") {
		return (&Youdao163{}).Extract(rawURL, opts)
	}
	if strings.Contains(rawURL, "/j/my/courseListV2") || strings.Contains(rawURL, "study.163.com/mycourse") {
		return extractStudyCourseList(c)
	}
	if pkg, ok := parseStudyPackage(rawURL); ok {
		return extractKaoyanPackage(c, pkg)
	}
	if spec, ok := parseStudySpec(rawURL); ok {
		return extractStudySpec(c, spec)
	}
	if mooc, ok := parseStudyMooc(rawURL); ok {
		return extractStudyMooc(c, mooc)
	}
	if comp, ok := parseStudyComposite(rawURL); ok {
		return extractStudyComposite(c, comp)
	}
	if cid := firstGroup(studyCourseIDRe, rawURL); cid != "" {
		return extractStudyMain(c, cid)
	}
	return nil, fmt.Errorf("cannot parse study163 URL: %s", rawURL)
}

func studyHeaders(ref string) map[string]string {
	return map[string]string{"Referer": firstNonEmpty(ref, studyReferer), "Origin": studyReferer}
}

func extractStudyCourseList(c *util.Client) (*extractor.MediaInfo, error) {
	var entries []*extractor.MediaInfo
	for page := 1; page < 10; page++ {
		body, err := c.GetString(fmt.Sprintf(studyListURL, page), studyHeaders(studyReferer))
		if err != nil {
			return nil, err
		}
		var out struct {
			Result struct {
				List []struct {
					Type     int    `json:"type"`
					TermID   any    `json:"termId"`
					CourseID any    `json:"courseId"`
					Name     string `json:"name"`
					EndTime  int64  `json:"endTime"`
				} `json:"list"`
			} `json:"result"`
		}
		if err := decodeJSON(body, &out); err != nil || len(out.Result.List) == 0 {
			break
		}
		for _, item := range out.Result.List {
			cid := valueString(item.CourseID)
			termID := valueString(item.TermID)
			if cid == "" {
				continue
			}
			courseURL := studyListCourseURL(item.Type, cid, termID)
			entries = append(entries, &extractor.MediaInfo{
				Site:  "study163",
				Title: sanitize(firstNonEmpty(item.Name, cid)),
				Extra: map[string]any{"type": item.Type, "course_id": cid, "term_id": termID, "course_url": courseURL, "source_api": "courseListV2"},
			})
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("study163: no enrolled courses found")
	}
	return &extractor.MediaInfo{Site: "study163", Title: "网易云课堂", Entries: entries, Extra: map[string]any{"source_api": "courseListV2"}}, nil
}

func studyListCourseURL(typ int, cid, termID string) string {
	switch typ {
	case 100:
		return fmt.Sprintf("https://ke.study.163.com/course/detail/%s", cid)
	case 3:
		if termID != "" {
			return fmt.Sprintf("https://mooc.study.163.com/course/%s?tid=%s", cid, termID)
		}
		return fmt.Sprintf("https://mooc.study.163.com/course/%s", cid)
	default:
		return fmt.Sprintf(studyMainURL, cid)
	}
}

func extractStudyMain(c *util.Client, cid string) (*extractor.MediaInfo, error) {
	pageURL := fmt.Sprintf(studyMainURL, cid)
	page, _ := c.GetString(pageURL, studyHeaders(pageURL))
	title := studyTitle(page, "study163_"+cid)
	body, err := c.PostForm(studyPlanURL, studyDWRData("PlanNewBean", "getPlanCourseDetail", map[string]string{
		"c0-param0": "string:" + cid,
		"c0-param1": "number:0",
		"c0-param2": "null:null",
	}), studyHeaders(pageURL))
	if err != nil {
		return nil, err
	}
	lessons := parseStudyPlanLessons(body)
	var entries []studyEntry
	for _, ls := range lessons {
		entries = append(entries, studyMainEntriesForLesson(c, cid, ls)...)
	}
	entries = append(entries, studyEntriesFromText(c, body, title)...)
	return studyMediaInfo(title, entries, "study163", map[string]any{"course_id": cid, "source_api": "PlanNewBean.getPlanCourseDetail"})
}

func studyMainEntriesForLesson(c *util.Client, cid string, src studySource) []studyEntry {
	var entries []studyEntry
	switch src.contentType {
	case "2", "50":
		body, err := c.PostForm(studyVideoURL, studyDWRData("LessonLearnBean", "getVideoLearnInfo", map[string]string{"c0-param0": "string:" + src.id, "c0-param1": "string:" + cid}), studyHeaders(studyReferer))
		if err == nil {
			entries = append(entries, studyEntriesFromText(c, body, src.name)...)
		}
	case "70":
		body, err := c.PostForm(studyAudioURL, studyDWRData("LessonLearnBean", "getAudioLearnInfo", map[string]string{"c0-param0": "string:" + src.id, "c0-param1": "string:" + cid}), studyHeaders(studyReferer))
		if err == nil {
			entries = append(entries, studyEntriesFromText(c, body, src.name)...)
		}
	case "3":
		body, err := c.PostForm(studyTextURL, studyDWRData("LessonLearnBean", "getTextLearnInfo", map[string]string{"c0-param0": "string:" + src.id, "c0-param1": "string:" + cid}), studyHeaders(studyReferer))
		if err == nil {
			entries = append(entries, studyEntriesFromText(c, body, src.name)...)
		}
	}
	body, err := c.PostForm(studyAttachURL, studyDWRData("LessonReferenceBean", "getLessonReferenceVoByLessonId", map[string]string{"c0-param0": "number:" + src.id}), studyHeaders(studyReferer))
	if err == nil {
		entries = append(entries, studyEntriesFromText(c, body, src.name)...)
	}
	return entries
}

type studyMoocTarget struct{ cid, termID string }

func parseStudyMooc(raw string) (studyMoocTarget, bool) {
	m := studyMoocURLRe.FindStringSubmatch(raw)
	if m == nil {
		return studyMoocTarget{}, false
	}
	return studyMoocTarget{cid: m[studyMoocURLRe.SubexpIndex("cid")], termID: m[studyMoocURLRe.SubexpIndex("tid")]}, true
}

func extractStudyMooc(c *util.Client, tgt studyMoocTarget) (*extractor.MediaInfo, error) {
	pageURL := fmt.Sprintf(studyMoocCourseURL, tgt.cid)
	if tgt.termID != "" {
		pageURL += "?tid=" + tgt.termID
	}
	page, _ := c.GetString(pageURL, studyHeaders(pageURL))
	if tgt.termID == "" {
		tgt.termID = firstNonEmpty(match1(page, `termId\s*:\s*"(\d+)"`), match1(page, `currentTermId\s*:\s*"?(\d+)"?`))
	}
	if tgt.termID == "" {
		return nil, fmt.Errorf("study163 mooc: cannot find termId for %s", tgt.cid)
	}
	_, _ = c.PostForm(studyMoocStartURL, studyDWRData("CourseBean", "startTermLearn", map[string]string{"c0-param0": "string:" + tgt.termID, "c0-param1": "null:null"}), studyHeaders(pageURL))
	_, _ = c.PostForm(studyMoocJoinedURL, studyDWRData("CourseBean", "checkTermLearn", map[string]string{"c0-param0": "string:" + tgt.termID}), studyHeaders(pageURL))
	body, err := c.PostForm(studyMoocInfoURL, studyDWRData("CourseBean", "getLastLearnedMocTermDto", map[string]string{"c0-param0": "number:" + tgt.termID}), studyHeaders(pageURL))
	if err != nil {
		return nil, err
	}
	units := parseStudyMoocUnits(body)
	var entries []studyEntry
	for _, u := range units {
		entries = append(entries, studyMoocEntriesForUnit(c, tgt.termID, u)...)
	}
	entries = append(entries, studyEntriesFromText(c, body, studyTitle(page, "study163_mooc_"+tgt.cid))...)
	return studyMediaInfo(studyTitle(page, "study163_mooc_"+tgt.cid), entries, "study163", map[string]any{"course_id": tgt.cid, "term_id": tgt.termID, "source_api": "CourseBean.getLastLearnedMocTermDto"})
}

func studyMoocEntriesForUnit(c *util.Client, termID string, src studySource) []studyEntry {
	body, err := c.PostForm(studyMoocParseURL, studyDWRData("CourseBean", "getLessonUnitLearnVo", map[string]string{
		"c0-param0": "number:" + termID,
		"c0-param1": "number:" + src.contentID,
		"c0-param2": "number:" + src.contentType,
		"c0-param3": "number:0",
		"c0-param4": "number:" + src.id,
	}), studyHeaders(studyReferer))
	if err != nil {
		return nil
	}
	entries := studyEntriesFromText(c, body, src.name)
	if src.contentType == "4" {
		if html := match1(body, `htmlContent\s*:\s*"([\s\S]*?)",\s*id`); html != "" {
			entries = append(entries, studyEntry{title: src.name + " 富文本", url: "data:text/html;charset=utf-8," + url.PathEscape(strings.ReplaceAll(html, `\n`, "<br/>")), format: "html", quality: "document"})
		}
	}
	return entries
}

type studyCompositeTarget struct{ cid string }

func parseStudyComposite(raw string) (studyCompositeTarget, bool) {
	m := studyCompositeURLRe.FindStringSubmatch(raw)
	if m == nil {
		return studyCompositeTarget{}, false
	}
	return studyCompositeTarget{cid: m[studyCompositeURLRe.SubexpIndex("cid")]}, true
}

func extractStudyComposite(c *util.Client, tgt studyCompositeTarget) (*extractor.MediaInfo, error) {
	pageURL := fmt.Sprintf(studyCompositeURL, tgt.cid)
	page, _ := c.GetString(pageURL, studyHeaders(pageURL))
	termID := firstNonEmpty(match1(page, `"termId"\s*:\s*(\d+)`), tgt.cid)
	title := studyTitle(page, "study163_course_"+termID)
	_, _ = c.PostForm(studyCompositePriceURL, map[string]string{"courseId": tgt.cid, "preview": "false"}, studyHeaders(pageURL))
	_, _ = c.PostForm(studyCompositeTermsURL, map[string]string{"termId": termID, "preview": "0"}, studyHeaders(pageURL))
	body, err := c.PostForm(studyCompositeListURL, map[string]string{"termId": termID, "preview": "0"}, studyHeaders(pageURL))
	if err != nil {
		return nil, err
	}
	var payload any
	_ = decodeJSON(body, &payload)
	sources := studySourcesFromAny(payload)
	var entries []studyEntry
	for _, src := range sources {
		entries = append(entries, studyCompositeEntriesForSource(c, termID, src)...)
	}
	entries = append(entries, studyEntriesFromAny(c, payload, title)...)
	return studyMediaInfo(title, entries, "study163", map[string]any{"term_id": termID, "source_api": "cp.lecture.front"})
}

func studyCompositeEntriesForSource(c *util.Client, termID string, src studySource) []studyEntry {
	body, err := c.PostForm(studyCompositeParseURL, map[string]string{
		"termId":        termID,
		"scene-type-id": "front-3-" + src.id,
		"preview":       "0",
		"nodeType":      "3",
		"productType":   "study",
		"id":            src.id,
		"contentType":   src.contentType,
		"contentId":     src.contentID,
	}, studyHeaders(studyReferer))
	if err != nil {
		return nil
	}
	var payload any
	if decodeJSON(body, &payload) == nil {
		return studyEntriesFromAny(c, payload, src.name)
	}
	return studyEntriesFromText(c, body, src.name)
}

type studySpecTarget struct{ specID, cid string }

func parseStudySpec(raw string) (studySpecTarget, bool) {
	m := studySpecURLRe.FindStringSubmatch(raw)
	if m == nil {
		return studySpecTarget{}, false
	}
	return studySpecTarget{specID: firstNonEmpty(m[studySpecURLRe.SubexpIndex("sid")]), cid: firstNonEmpty(m[studySpecURLRe.SubexpIndex("cid")])}, true
}

func extractStudySpec(c *util.Client, tgt studySpecTarget) (*extractor.MediaInfo, error) {
	if tgt.specID != "" {
		page, err := c.GetString(fmt.Sprintf(studySpecURL, tgt.specID), studyHeaders(studyReferer))
		if err != nil {
			return nil, err
		}
		tgt.cid = firstNonEmpty(match1(page, `enrolledS2MicroTermId\s*=\s*(\d+);`), match1(page, `mappingS2TermId\s*:\s*(\d+)`))
		_, _ = c.GetString(fmt.Sprintf(studySpecPriceURL, tgt.specID), studyHeaders(studyReferer))
	}
	if tgt.cid == "" {
		return nil, fmt.Errorf("study163 spec: cannot find term id")
	}
	_, _ = c.GetString(fmt.Sprintf(studySpecLearnURL, tgt.cid), studyHeaders(studyReferer))
	return extractStudyComposite(c, studyCompositeTarget{cid: tgt.cid})
}

type kaoyanPackageTarget struct{ packageID string }

func parseStudyPackage(raw string) (kaoyanPackageTarget, bool) {
	m := kaoyanPackageRe.FindStringSubmatch(raw)
	if m == nil {
		return kaoyanPackageTarget{}, false
	}
	return kaoyanPackageTarget{packageID: m[kaoyanPackageRe.SubexpIndex("pid")]}, true
}

func extractKaoyanPackage(c *util.Client, tgt kaoyanPackageTarget) (*extractor.MediaInfo, error) {
	pageURL := fmt.Sprintf(kaoyanPackageURL, tgt.packageID)
	page, _ := c.GetString(pageURL, headers())
	title := sanitize(firstNonEmpty(match1(page, `name\s*:\s*"(.*?)"`), "kaoyan_package_"+tgt.packageID))
	body, err := c.PostForm(kaoyanPackageInfo+srckey, map[string]string{"coursePackageId": tgt.packageID}, headers())
	if err != nil {
		return nil, err
	}
	var out struct {
		Result []struct {
			CourseName string `json:"courseName"`
			TermID     any    `json:"termId"`
		} `json:"result"`
	}
	if err := decodeJSON(body, &out); err != nil {
		return nil, err
	}
	if len(out.Result) == 0 {
		return nil, fmt.Errorf("kaoyan package %s has no terms", tgt.packageID)
	}
	var entries []*extractor.MediaInfo
	for i, term := range out.Result {
		termID := valueString(term.TermID)
		if termID == "" {
			continue
		}
		mi, err := extractKaoyan(c, kaoyanURLInfo{termID: termID})
		if err != nil {
			entries = append(entries, &extractor.MediaInfo{Site: "icourse163", Title: sanitize(firstNonEmpty(term.CourseName, termID)), Extra: map[string]any{"term_id": termID, "source_url": fmt.Sprintf("https://kaoyan.icourse163.org/course/terms/%s.htm", termID)}})
			continue
		}
		mi.Title = fmt.Sprintf("%02d %s", i+1, sanitize(firstNonEmpty(term.CourseName, mi.Title)))
		entries = append(entries, mi)
	}
	return &extractor.MediaInfo{Site: "icourse163", Title: title, Entries: entries, Extra: map[string]any{"package_id": tgt.packageID, "source_api": "kaoyanCourseBean.getMocCoursePackageIncludeTerm"}}, nil
}

func studyMediaInfo(title string, entries []studyEntry, site string, extra map[string]any) (*extractor.MediaInfo, error) {
	entries = dedupeStudyEntries(entries)
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s: no downloadable resource found", site)
	}
	if len(entries) == 1 {
		mi := studyEntryMedia(site, firstNonEmpty(entries[0].title, title), entries[0])
		mi.Extra = mergeExtra(mi.Extra, extra)
		return mi, nil
	}
	children := make([]*extractor.MediaInfo, 0, len(entries))
	for _, e := range entries {
		children = append(children, studyEntryMedia(site, e.title, e))
	}
	return &extractor.MediaInfo{Site: site, Title: sanitize(title), Entries: children, Extra: extra}, nil
}

func studyEntryMedia(site, title string, e studyEntry) *extractor.MediaInfo {
	format := firstNonEmpty(e.format, formatFromURL(e.url, "mp4"))
	quality := firstNonEmpty(e.quality, format)
	stream := extractor.Stream{Quality: quality, URLs: []string{e.url}, Format: format, Size: e.size, Headers: studyHeaders(studyReferer)}
	if format == "m3u8" {
		stream.NeedMerge = true
	}
	return &extractor.MediaInfo{Site: site, Title: sanitize(firstNonEmpty(title, e.title, "study163")), Streams: map[string]extractor.Stream{quality: stream}, Extra: e.extra}
}

func mergeExtra(base, extra map[string]any) map[string]any {
	if len(extra) == 0 {
		return base
	}
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func dedupeStudyEntries(in []studyEntry) []studyEntry {
	seen := map[string]bool{}
	var out []studyEntry
	for _, e := range in {
		if e.url == "" || seen[e.url] {
			continue
		}
		seen[e.url] = true
		out = append(out, e)
	}
	return out
}

func studyEntriesFromAny(c *util.Client, v any, fallbackTitle string) []studyEntry {
	var out []studyEntry
	var walk func(any, string)
	walk = func(x any, title string) {
		switch t := x.(type) {
		case map[string]any:
			itemTitle := firstNonEmpty(valueString(t["name"]), valueString(t["title"]), valueString(t["lessonName"]), title)
			if sig := firstNonEmpty(valueString(t["signature"]), valueString(t["videoSignature"])); sig != "" {
				vid := firstNonEmpty(valueString(t["videoId"]), valueString(t["videoID"]))
				if e, err := studyVideoBySignature(c, sig, vid, t); err == nil && e.url != "" {
					e.title = firstNonEmpty(itemTitle, e.title)
					out = append(out, e)
				}
			}
			for _, key := range []string{"videoUrl", "videoURL", "downloadUrl", "download_url", "nosDownloadUrl", "pdfUrl", "url", "audioUrl", "fileUrl", "content"} {
				if s := valueString(t[key]); s != "" {
					out = append(out, studyEntriesFromText(c, s, itemTitle)...)
				}
			}
			for _, vv := range t {
				walk(vv, itemTitle)
			}
		case []any:
			for _, vv := range t {
				walk(vv, title)
			}
		case string:
			out = append(out, studyEntriesFromText(c, t, title)...)
		}
	}
	walk(v, fallbackTitle)
	return out
}

var (
	studyURLRe       = regexp.MustCompile(`(?i)https?:\\?/\\?/[^"'<>\s]+?\.(?:m3u8|mp4|flv|mp3|m4a|aac|pdf|pptx?|docx?|xlsx?|zip|rar|7z|txt|html)(?:\?[^"'<>\s]*)?`)
	studySignatureRe = regexp.MustCompile(`(?is)signature\s*=\s*"(\w+)"[\s\S]+?videoId\s*=\s*(\d+)`)
	studyPDFRe       = regexp.MustCompile(`(?is)pdfUrl\s*:?\s*"(https?://.+?)"`)
	studyAudioRe     = regexp.MustCompile(`(?is)url\s*=\s*"(https?://[^";]+)"`)
)

func studyEntriesFromText(c *util.Client, text, fallbackTitle string) []studyEntry {
	text = strings.ReplaceAll(text, `\/`, `/`)
	var out []studyEntry
	for _, m := range studySignatureRe.FindAllStringSubmatch(text, -1) {
		if e, err := studyVideoBySignature(c, m[1], m[2], nil); err == nil && e.url != "" {
			e.title = fallbackTitle
			out = append(out, e)
		}
	}
	for _, m := range studyPDFRe.FindAllStringSubmatch(text, -1) {
		out = append(out, studyURLToEntry(stripStudyDownloadParam(m[1]), fallbackTitle))
	}
	for _, m := range studyURLRe.FindAllString(text, -1) {
		out = append(out, studyURLToEntry(m, fallbackTitle))
	}
	for _, m := range studyAudioRe.FindAllStringSubmatch(text, -1) {
		out = append(out, studyURLToEntry(m[1], fallbackTitle))
	}
	return out
}

var studyDownloadParamRe = regexp.MustCompile(`(?i)([?&])download=[^&]*&?`)

func stripStudyDownloadParam(raw string) string {
	out := studyDownloadParamRe.ReplaceAllString(raw, "$1")
	out = strings.Replace(out, "?&", "?", 1)
	return strings.TrimRight(out, "?&")
}

func studyURLToEntry(raw, title string) studyEntry {
	raw = strings.Trim(strings.ReplaceAll(raw, `\/`, `/`), `"' ;,`)
	format := formatFromURL(raw, "mp4")
	quality := "source"
	if format == "pdf" || format == "html" || format == "zip" || format == "rar" || format == "7z" || format == "doc" || format == "docx" || format == "ppt" || format == "pptx" {
		quality = "document"
	} else if format == "mp3" || format == "m4a" || format == "aac" {
		quality = "audio"
	}
	return studyEntry{title: title, url: raw, format: format, quality: quality}
}

func studyVideoBySignature(c *util.Client, signature, videoID string, raw map[string]any) (studyEntry, error) {
	if signature == "" || videoID == "" {
		return studyEntry{}, fmt.Errorf("missing video signature")
	}
	body, err := c.PostForm(videoInfoURL, map[string]string{"clientType": "3", "signature": signature, "videoId": videoID}, studyHeaders(studyReferer))
	if err != nil {
		return studyEntry{}, err
	}
	var out struct {
		Result struct {
			Videos []struct {
				Format   string `json:"format"`
				Quality  int    `json:"quality"`
				VideoURL string `json:"videoUrl"`
				Size     int64  `json:"size"`
				E        bool   `json:"e"`
			} `json:"videos"`
		} `json:"result"`
	}
	if err := decodeJSON(body, &out); err != nil {
		return studyEntry{}, err
	}
	best := studyEntry{quality: "-1", extra: map[string]any{"video_id": videoID}}
	for _, pref := range []string{"mp4", "hls", "flv"} {
		for _, v := range out.Result.Videos {
			if v.Format != pref || v.VideoURL == "" {
				continue
			}
			if v.E && pref == "flv" {
				continue
			}
			q, _ := strconv.Atoi(best.quality)
			if v.Quality <= q {
				continue
			}
			format := v.Format
			if format == "hls" {
				format = "m3u8"
			}
			best.url, best.format, best.quality, best.size = v.VideoURL, format, strconv.Itoa(v.Quality), v.Size
		}
		if best.url != "" {
			break
		}
	}
	if best.url == "" {
		return studyEntry{}, fmt.Errorf("no playable video in vod result")
	}
	if raw != nil {
		best.title = firstNonEmpty(valueString(raw["name"]), valueString(raw["title"]))
	}
	return best, nil
}

func studySourcesFromAny(v any) []studySource {
	var out []studySource
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			data := t
			if dm, ok := t["data"].(map[string]any); ok {
				data = dm
			}
			src := studySource{
				id:          firstNonEmpty(valueString(t["id"]), valueString(data["id"]), valueString(t["lessonUnitId"])),
				contentID:   firstNonEmpty(valueString(data["contentId"]), valueString(t["contentId"])),
				contentType: firstNonEmpty(valueString(data["contentType"]), valueString(t["contentType"]), valueString(t["type"])),
				name:        firstNonEmpty(valueString(t["name"]), valueString(t["title"]), valueString(data["name"])),
			}
			if src.contentID != "" && src.contentType != "" && src.id != "" {
				out = append(out, src)
			}
			for _, vv := range t {
				walk(vv)
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(v)
	return dedupeStudySources(out)
}

func dedupeStudySources(in []studySource) []studySource {
	seen := map[string]bool{}
	var out []studySource
	for _, s := range in {
		key := s.id + ":" + s.contentID + ":" + s.contentType
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

func parseStudyPlanLessons(body string) []studySource {
	chapters := regexp.MustCompile(`(?is)courseId=\d+;.+?id=(\d+);.+?name="([\s\S]*?)";`).FindAllStringSubmatch(body, -1)
	var out []studySource
	for _, ch := range chapters {
		re := regexp.MustCompile(`(?is)chapterId=` + regexp.QuoteMeta(ch[1]) + `;[\s\S]*?id=(\d+);[\s\S]*?lessonName="([\s\S]*?)";[\s\S]*?type=(\d+);`)
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			out = append(out, studySource{id: m[1], contentType: m[3], name: m[2]})
		}
	}
	return out
}

func parseStudyMoocUnits(body string) []studySource {
	lessonIDs := regexp.MustCompile(`(?is)contentType=1.+?id=(\d+).+?isTestChecked=false.+?name="([\s\S]+?)".+?test`).FindAllStringSubmatch(body, -1)
	var out []studySource
	for _, ls := range lessonIDs {
		re := regexp.MustCompile(`(?is)contentId=(\d+).+?contentType=(1|3|4|7).+?id=(\d+).+?lessonId=` + regexp.QuoteMeta(ls[1]) + `.+?name="(.+)"`)
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			out = append(out, studySource{contentID: m[1], contentType: m[2], id: m[3], name: m[4]})
		}
	}
	return out
}

func studyDWRData(script, method string, override map[string]string) map[string]string {
	d := map[string]string{
		"batchId":         strconv.FormatInt(time.Now().UnixMilli(), 10),
		"callCount":       "1",
		"scriptSessionId": "${scriptSessionId}190",
		"c0-id":           "0",
		"c0-scriptName":   script,
		"c0-methodName":   method,
	}
	for k, v := range override {
		d[k] = v
	}
	return d
}

func studyTitle(page, fallback string) string {
	for _, pat := range []string{`<title>(.+?)(?:\s*-\s*网易云课堂)?</title>`, `window\.course[\s\S]*?name\s*:\s*"(.*?)"`, `courseName\s*:\s*['"]([^'"]+)['"]`} {
		if t := match1(page, pat); t != "" {
			return sanitize(t)
		}
	}
	return fallback
}

func firstGroup(re *regexp.Regexp, raw string) string {
	m := re.FindStringSubmatch(raw)
	if len(m) == 0 {
		return ""
	}
	for _, g := range m[1:] {
		if strings.TrimSpace(g) != "" {
			return strings.TrimSpace(g)
		}
	}
	return ""
}

func sortStudyEntries(entries []studyEntry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].title < entries[j].title })
}

var _ = sortStudyEntries
var _ http.CookieJar
