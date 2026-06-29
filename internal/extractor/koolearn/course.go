package koolearn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

const (
	urlStudyCoursePage    = "https://study.koolearn.com/%s/course/%s/%s"
	urlStudyCategory      = "https://study.koolearn.com/%s/course_kc_data/%s/%s?pathId=%s&nodeId=-1&level=1"
	urlStudyLesson        = "https://study.koolearn.com/%s/course_kc_data/%s/%s?pathId=%s&nodeId=%s&level=%d&learningSubjectId=%s"
	urlStudyChuguoIndex   = "https://study.koolearn.com/chuguo/index/%s?orderNo=%s"
	urlStudyChuguoNew     = "https://study.koolearn.com/chuguo/index/new/index?productId=%s&orderNo=%s"
	urlStudyChuguoModules = "https://study.koolearn.com/chuguo/index/course-module/%s/%s"
	urlStudyChuguoSubject = "https://study.koolearn.com/chuguo/lesson/%s?orderNo=%s&currentModuleId=%s"
	urlStudyChuguoNodes   = "https://study.koolearn.com/chuguo/lesson/nodes?productId=%s&moduleId=%s&learningSubjectId=%s&courseId=%s&nodeId=%s&level=%d&isPushed=%s"
	urlStudyTinyTitle     = "https://study.koolearn.com/chuguo/small-class/index/top-msg?orderNo=%s&productId=%s"
	urlStudyTinyModules   = "https://study.koolearn.com/chuguo/small-class/index/course-module?orderNo=%s&productId=%s"
	urlStudyTinyNodes     = "https://study.koolearn.com/chuguo/small-class/lesson/nodes?itemId=%s&orderNo=%s&productId=%s&level=%d&type=%s&learningSubjectId=%s"
	urlStudyFindUser      = "https://study.koolearn.com/common/find/user"
)

var (
	reStudyDirect    = regexp.MustCompile(`(?i)study\.koolearn\.com/(?P<ctype>tongyong|ky|fer|schedule)/.*?/(?P<cid>\d+)/(?P<order>[\w_]+)(?P<param>\?ct=\d+&courseId=\d+)?`)
	reStudyVIP       = regexp.MustCompile(`(?i)vip\.koolearn\.com/home/(?P<cid>\d+)/(?P<order>[\w_]+)`)
	reChuguoDirect   = regexp.MustCompile(`(?i)study\.koolearn\.com/(?:chuguo/index|chuguo/lesson)/(?P<cid>\d+).*?orderNo=(?P<order>[\w_]+)`)
	reChuguoZhixin   = regexp.MustCompile(`(?i)study\.koolearn\.com/fer/.*?zhixin/(?P<cid>\d+).*?orderNo=(?P<order>[\w_]+)`)
	reTinyDirect     = regexp.MustCompile(`(?i)study\.koolearn\.com.*?/tiny-class/.*?/(?P<order>[\w_]+)/(?P<cid>\d+)`)
	reLearningCourse = regexp.MustCompile(`(?i)study\.koolearn\.com/(?P<ctype>tongyong|ky|chuguo)/learning/\d+/\d+/\d+`)
)

type studyRoute struct {
	brand    string
	ctype    string
	cid      string
	order    string
	paramURL string
	rawURL   string
}

func parseStudyRoute(rawURL string) studyRoute {
	route := studyRoute{rawURL: rawURL, ctype: "tongyong"}
	if m := reTinyDirect.FindStringSubmatch(rawURL); len(m) > 0 {
		return studyRoute{brand: "tiny", ctype: "chuguo", cid: namedSubmatch(reTinyDirect, m, "cid"), order: namedSubmatch(reTinyDirect, m, "order"), rawURL: rawURL}
	}
	for _, re := range []*regexp.Regexp{reChuguoDirect, reChuguoZhixin} {
		if m := re.FindStringSubmatch(rawURL); len(m) > 0 {
			return studyRoute{brand: "chuguo", ctype: "chuguo", cid: namedSubmatch(re, m, "cid"), order: namedSubmatch(re, m, "order"), rawURL: rawURL}
		}
	}
	if m := reStudyVIP.FindStringSubmatch(rawURL); len(m) > 0 {
		return studyRoute{brand: "course", ctype: "tongyong", cid: namedSubmatch(reStudyVIP, m, "cid"), order: namedSubmatch(reStudyVIP, m, "order"), rawURL: rawURL}
	}
	if m := reStudyDirect.FindStringSubmatch(rawURL); len(m) > 0 {
		ctype := namedSubmatch(reStudyDirect, m, "ctype")
		brand := "course"
		if ctype == "fer" {
			brand, ctype = "chuguo", "chuguo"
		}
		return studyRoute{brand: brand, ctype: firstNonEmpty(ctype, "tongyong"), cid: namedSubmatch(reStudyDirect, m, "cid"), order: namedSubmatch(reStudyDirect, m, "order"), paramURL: namedSubmatch(reStudyDirect, m, "param"), rawURL: rawURL}
	}
	if m := reLearningCourse.FindStringSubmatch(rawURL); len(m) > 0 {
		ctype := namedSubmatch(reLearningCourse, m, "ctype")
		route.brand = map[bool]string{true: "chuguo", false: "course"}[ctype == "chuguo"]
		route.ctype = ctype
	}
	return route
}

func namedSubmatch(re *regexp.Regexp, match []string, name string) string {
	for i, n := range re.SubexpNames() {
		if n == name && i < len(match) {
			return strings.TrimSpace(match[i])
		}
	}
	return ""
}

func isStudyCourseURL(rawURL string) bool {
	r := parseStudyRoute(rawURL)
	return r.brand != "" && (r.cid != "" || reLearningCourse.MatchString(rawURL))
}

func extractStudyCourse(c *util.Client, jar http.CookieJar, rawURL string) (*extractor.MediaInfo, error) {
	if !studyLogined(c) {
		return nil, fmt.Errorf("koolearn study course requires login cookies")
	}
	route := parseStudyRoute(rawURL)
	sc := &studyContext{c: c, header: studyHeaders(jar), cid: route.cid, order: route.order, ctype: firstNonEmpty(route.ctype, "tongyong"), brand: firstNonEmpty(route.brand, "course"), paramURL: route.paramURL}
	if err := sc.resolveStudyIDs(route); err != nil {
		return nil, err
	}
	if sc.cid == "" || sc.order == "" {
		return nil, fmt.Errorf("koolearn study course: cannot parse product/order from %s", rawURL)
	}
	title := sc.resolveStudyTitle()
	entries := sc.collectStudyEntries()
	if len(entries) == 0 {
		return nil, fmt.Errorf("koolearn study course %s/%s: no playable lessons found", sc.cid, sc.order)
	}
	return &extractor.MediaInfo{Site: "koolearn", Title: firstNonEmpty(title, "koolearn_"+sc.cid), Entries: entries, Extra: map[string]any{"product_id": sc.cid, "order_no": sc.order, "ctype": sc.ctype, "brand": sc.brand, "user_course_id": sc.userCID, "user_id": sc.userID}}, nil
}

func (sc *studyContext) resolveStudyIDs(route studyRoute) error {
	if sc.cid != "" && sc.order != "" {
		return nil
	}
	body, err := sc.getString(route.rawURL)
	if err != nil {
		return err
	}
	sc.order = firstNonEmpty(sc.order, match1Local(body, `"orderNo"\s*:\s*"([\w_]+)"`), match1Local(body, `orderNo\s*:\s*"([\w_]+)"`))
	sc.cid = firstNonEmpty(sc.cid, match1Local(body, `"productId"\s*:\s*(\d+)`), match1Local(body, `productId\s*:\s*(\d+)`))
	sc.paramURL = firstNonEmpty(sc.paramURL, match1Local(body, `(\?ct=\d+&courseId=\d+)`))
	return nil
}

func (sc *studyContext) resolveStudyTitle() string {
	switch sc.brand {
	case "chuguo":
		body, _ := sc.getString(fmt.Sprintf(urlStudyChuguoIndex, url.QueryEscape(sc.cid), url.QueryEscape(sc.order)))
		title := firstNonEmpty(match1Local(body, `className\s*:\s*'(.+)'`), match1Local(body, `"productName"\s*:\s*"(.+?)"`))
		sc.userCID = firstNonEmpty(sc.userCID, match1Local(body, `'userCourseId'\s*:\s*'?(\d+)'?`), match1Local(body, `"userCourseId"\s*:\s*(\d+)`))
		if title == "" || sc.userCID == "" {
			body, _ = sc.getString(fmt.Sprintf(urlStudyChuguoNew, url.QueryEscape(sc.cid), url.QueryEscape(sc.order)))
			title = firstNonEmpty(title, match1Local(body, `"productName"\s*:\s*"(.+?)"`))
			sc.userCID = firstNonEmpty(sc.userCID, match1Local(body, `"userCourseId"\s*:\s*(\d+)`))
		}
		sc.userID = firstNonEmpty(sc.userID, sc.fetchStudyUserID())
		return sanitizeStudyTitle(title)
	case "tiny":
		body, _ := sc.getString(fmt.Sprintf(urlStudyTinyTitle, url.QueryEscape(sc.order), url.QueryEscape(sc.cid)))
		sc.userID = firstNonEmpty(sc.userID, sc.fetchStudyUserID())
		return sanitizeStudyTitle(match1Local(body, `"?productName"?\s*:\s*"(.+?)"`))
	default:
		body, _ := sc.getString(fmt.Sprintf(urlStudyCoursePage, url.QueryEscape(sc.ctype), url.QueryEscape(sc.cid), url.QueryEscape(sc.order)) + sc.paramURL)
		title := firstNonEmpty(match1Local(body, `<title>(.+)-新东方在线网络课堂</title>`), match1Local(body, `className\s*:\s*'(.+)'`))
		sc.userCID = firstNonEmpty(sc.userCID, match1Local(body, `'userCourseId'\s*:\s*'?(\d+)'?`))
		sc.userID = firstNonEmpty(sc.userID, match1Local(body, `'user_id'\s*:\s*(\d+)`))
		return sanitizeStudyTitle(title)
	}
}

func (sc *studyContext) fetchStudyUserID() string {
	body, err := sc.getString(urlStudyFindUser)
	if err != nil {
		return ""
	}
	return match1Local(body, `"userId"\s*:\s*(\d+)`)
}

func (sc *studyContext) collectStudyEntries() []*extractor.MediaInfo {
	switch sc.brand {
	case "chuguo":
		return sc.collectChuguoEntries()
	case "tiny":
		return sc.collectTinyEntries()
	default:
		return sc.collectCourseEntries()
	}
}

func (sc *studyContext) collectCourseEntries() []*extractor.MediaInfo {
	page, err := sc.getString(fmt.Sprintf(urlStudyCoursePage, url.QueryEscape(sc.ctype), url.QueryEscape(sc.cid), url.QueryEscape(sc.order)) + sc.paramURL)
	if err != nil {
		return nil
	}
	stages := parseLessonStages(page)
	var out []*extractor.MediaInfo
	for si, stage := range stages {
		cats := sc.studyCategories(stage.nodeID)
		if len(cats) == 0 {
			cats = []studyNode{{nodeID: stage.nodeID, name: stage.name}}
		}
		for ci, cat := range cats {
			out = append(out, sc.collectCourseLessonEntries(stage.nodeID, cat.nodeID, cat.nodeID, 2, []int{si + 1, ci + 1})...)
		}
	}
	return dedupeKoolearnEntries(out)
}

func parseLessonStages(page string) []studyNode {
	block := match1Local(page, `lessonStage\s*:\s*(\[[\s\S]*?\])`)
	if block == "" {
		return nil
	}
	matches := regexp.MustCompile(`id\s*:\s*(\d+),?[\s\S]*?name\s*:\s*'(.+?)',?[\s\S]*?learnModel\s*:\s*(\d+)`).FindAllStringSubmatch(block, -1)
	out := make([]studyNode, 0, len(matches))
	for _, m := range matches {
		out = append(out, studyNode{nodeID: m[1], name: sanitizeStudyTitle(m[2]), nodeType: toIntLocal(m[3])})
	}
	return out
}

func (sc *studyContext) studyCategories(pathID string) []studyNode {
	order := sc.order
	if sc.ctype == "ky" {
		order += "/1/0"
	}
	api := fmt.Sprintf(urlStudyCategory, url.QueryEscape(sc.ctype), url.QueryEscape(sc.cid), url.QueryEscape(order), url.QueryEscape(pathID)) + strings.Replace(sc.paramURL, "?", "&", 1)
	var root map[string]any
	if ok, _ := sc.getJSON(api, &root); !ok {
		return nil
	}
	return nodesFromAny(root["data"])
}

func (sc *studyContext) studyLessons(pathID, nodeID, innerNodeID string, level int) []studyNode {
	if level > maxTreeDepth {
		return nil
	}
	order := sc.order
	if sc.ctype == "ky" {
		order += "/1/0"
	}
	api := fmt.Sprintf(urlStudyLesson, url.QueryEscape(sc.ctype), url.QueryEscape(sc.cid), url.QueryEscape(order), url.QueryEscape(pathID), url.QueryEscape(innerNodeID), level, url.QueryEscape(nodeID)) + strings.Replace(sc.paramURL, "?", "&", 1)
	var root map[string]any
	if ok, _ := sc.getJSON(api, &root); !ok {
		return nil
	}
	return studyNodesFromAny(root["data"])
}

func (sc *studyContext) collectCourseLessonEntries(pathID, subjectID, innerNodeID string, level int, prefix []int) []*extractor.MediaInfo {
	nodes := sc.studyLessons(pathID, subjectID, innerNodeID, level)
	var out []*extractor.MediaInfo
	for i, node := range nodes {
		idx := append(append([]int{}, prefix...), i+1)
		if !node.leaf && level < maxTreeDepth {
			children := sc.collectCourseLessonEntries(pathID, subjectID, node.nodeID, level+1, idx)
			if len(children) > 0 {
				out = append(out, children...)
				continue
			}
		}
		if entry := sc.leafNodeEntry(node, prefix, i+1, -1); entry != nil {
			out = append(out, entry)
		}
	}
	return out
}

func (sc *studyContext) collectChuguoEntries() []*extractor.MediaInfo {
	var root map[string]any
	if ok, _ := sc.getJSON(fmt.Sprintf(urlStudyChuguoModules, url.QueryEscape(sc.order), url.QueryEscape(sc.cid)), &root); !ok {
		return nil
	}
	modules := nodesFromAny(root["data"])
	var out []*extractor.MediaInfo
	for mi, mod := range modules {
		subID := sc.chuguoSubjectID(mod.nodeID)
		cats := sc.chuguoNodes(mod.nodeID, subID, "", 1, false)
		if len(cats) == 0 {
			cats = []studyNode{{nodeID: mod.nodeID, name: mod.name}}
		}
		for ci, cat := range cats {
			out = append(out, sc.collectChuguoLessonEntries(mod.nodeID, subID, cat.nodeID, 2, cat.isPushed, []int{mi + 1, ci + 1})...)
		}
	}
	return dedupeKoolearnEntries(out)
}

func (sc *studyContext) chuguoSubjectID(moduleID string) string {
	body, err := sc.getString(fmt.Sprintf(urlStudyChuguoSubject, url.QueryEscape(sc.cid), url.QueryEscape(sc.order), url.QueryEscape(moduleID)))
	if err != nil {
		return ""
	}
	sc.userCID = firstNonEmpty(sc.userCID, match1Local(body, `'userCourseId'\s*:\s*'?(\d+)'?`))
	return match1Local(body, `learningSubjectId\s*=\s*(\d+)`)
}

func (sc *studyContext) chuguoNodes(moduleID, subjectID, nodeID string, level int, pushed bool) []studyNode {
	push := "false"
	if pushed {
		push = "true"
	}
	if sc.userCID == "" {
		sc.userCID = sc.cid
	}
	api := fmt.Sprintf(urlStudyChuguoNodes, url.QueryEscape(sc.cid), url.QueryEscape(moduleID), url.QueryEscape(subjectID), url.QueryEscape(sc.userCID), url.QueryEscape(nodeID), level, push)
	var root map[string]any
	if ok, _ := sc.getJSON(api, &root); !ok {
		return nil
	}
	nodes := studyNodesFromAny(root["data"])
	for i := range nodes {
		nodes[i].userCID = firstNonEmpty(nodes[i].userCID, sc.userCID)
	}
	return nodes
}

func (sc *studyContext) collectChuguoLessonEntries(moduleID, subjectID, nodeID string, level int, pushed bool, prefix []int) []*extractor.MediaInfo {
	nodes := sc.chuguoNodes(moduleID, subjectID, nodeID, level, pushed)
	var out []*extractor.MediaInfo
	for i, node := range nodes {
		idx := append(append([]int{}, prefix...), i+1)
		if !node.leaf && level < maxTreeDepth {
			children := sc.collectChuguoLessonEntries(moduleID, subjectID, node.nodeID, level+1, node.isPushed, idx)
			if len(children) > 0 {
				out = append(out, children...)
				continue
			}
		}
		if entry := sc.leafNodeEntry(node, prefix, i+1, 0); entry != nil {
			out = append(out, entry)
		}
	}
	return out
}

func (sc *studyContext) collectTinyEntries() []*extractor.MediaInfo {
	var root map[string]any
	if ok, _ := sc.getJSON(fmt.Sprintf(urlStudyTinyModules, url.QueryEscape(sc.order), url.QueryEscape(sc.cid)), &root); !ok {
		return nil
	}
	modules := nodesFromAny(smartData(root, "courseModuleVos"))
	var out []*extractor.MediaInfo
	for mi, mod := range modules {
		cats := sc.tinyNodes(mod.nodeID, mod.nodeType, "", 1)
		if len(cats) == 0 {
			cats = []studyNode{{nodeID: mod.nodeID, name: mod.name, nodeType: mod.nodeType}}
		}
		for ci, cat := range cats {
			out = append(out, sc.collectTinyLessonEntries(mod.nodeID, mod.nodeType, cat.nodeID, 2, []int{mi + 1, ci + 1})...)
		}
	}
	return dedupeKoolearnEntries(out)
}

func (sc *studyContext) tinyNodes(moduleID string, moduleType int, nodeID string, level int) []studyNode {
	api := fmt.Sprintf(urlStudyTinyNodes, url.QueryEscape(firstNonEmpty(nodeID, moduleID)), url.QueryEscape(sc.order), url.QueryEscape(sc.cid), level, url.QueryEscape(fmt.Sprint(moduleType)), url.QueryEscape(moduleID))
	var root map[string]any
	if ok, _ := sc.getJSON(api, &root); !ok {
		return nil
	}
	return studyNodesFromAny(root["data"])
}

func (sc *studyContext) collectTinyLessonEntries(moduleID string, moduleType int, nodeID string, level int, prefix []int) []*extractor.MediaInfo {
	nodes := sc.tinyNodes(moduleID, moduleType, nodeID, level)
	var out []*extractor.MediaInfo
	for i, node := range nodes {
		idx := append(append([]int{}, prefix...), i+1)
		if !node.leaf && level < maxTreeDepth {
			children := sc.collectTinyLessonEntries(moduleID, moduleType, node.nodeID, level+1, idx)
			if len(children) > 0 {
				out = append(out, children...)
				continue
			}
		}
		if entry := sc.leafNodeEntry(node, prefix, i+1, 0); entry != nil {
			out = append(out, entry)
		}
	}
	return out
}

func (sc *studyContext) leafNodeEntry(node studyNode, prefix []int, counter, isRecommend int) *extractor.MediaInfo {
	stream, err := sc.resolveLeafStream(node, "", isRecommend)
	if err != nil || stream == nil {
		return nil
	}
	return leafEntry(indexPrefix(prefix, counter)+sanitizeStudyTitle(firstNonEmpty(node.name, node.nodeID)), stream, map[string]any{"node_id": node.nodeID, "ctype": sc.ctype, "brand": sc.brand, "user_course_id": firstNonEmpty(node.userCID, sc.userCID), "user_id": sc.userID})
}

func studyNodesFromAny(v any) []studyNode {
	var out []studyNode
	for _, m := range mapsFromJSON(v) {
		id := firstNonEmpty(valueString(m["id"]), valueString(m["nodeId"]), valueString(m["node_id"]))
		if id == "" {
			continue
		}
		nodeType := toIntLocal(firstNonEmpty(valueString(m["type"]), valueString(m["nodeType"])))
		name := sanitizeStudyTitle(firstNonEmpty(valueString(m["name"]), valueString(m["title"]), valueString(m["nodeName"])))
		leaf := nodeType == 2 || nodeType == 10 || nodeType == 12 || nodeType == 13 || truthy(m["isLeaf"]) || truthy(m["leaf"])
		isLive := truthy(m["is_live"]) || truthy(m["isLive"]) || strings.EqualFold(valueString(m["type"]), "live")
		out = append(out, studyNode{nodeID: id, name: firstNonEmpty(name, id), leaf: leaf, nodeType: nodeType, isLive: isLive, jumpURL: firstNonEmpty(valueString(m["jump_url"]), valueString(m["jumpUrl"])), userCID: firstNonEmpty(valueString(m["user_cid"]), valueString(m["userCourseId"])), isPushed: truthy(m["isPushed"]) || truthy(m["is_pushed"])})
	}
	return out
}

func nodesFromAny(v any) []studyNode {
	var out []studyNode
	for _, m := range mapsFromJSON(v) {
		id := firstNonEmpty(valueString(m["moduleId"]), valueString(m["nodeId"]), valueString(m["id"]), valueString(m["itemId"]))
		if id == "" {
			continue
		}
		out = append(out, studyNode{nodeID: id, name: sanitizeStudyTitle(firstNonEmpty(valueString(m["moduleName"]), valueString(m["name"]), valueString(m["title"]), id)), nodeType: toIntLocal(firstNonEmpty(valueString(m["moduleType"]), valueString(m["type"])))})
	}
	return out
}

func smartData(root map[string]any, key string) any {
	if d, ok := root["data"].(map[string]any); ok {
		return d[key]
	}
	return root[key]
}

func mapsFromJSON(v any) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			out = append(out, t)
			for _, child := range t {
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		}
	}
	walk(v)
	return out
}

func valueString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%v", x)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func truthy(v any) bool {
	s := strings.ToLower(valueString(v))
	return s == "1" || s == "true" || s == "yes" || s == "live"
}

func match1Local(s, pat string) string {
	if m := regexp.MustCompile(pat).FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func sanitizeStudyTitle(s string) string {
	return strings.TrimSpace(regexp.MustCompile(`[\\/:*?"<>|\r\n\t]+`).ReplaceAllString(s, "_"))
}

func toIntLocal(s string) int {
	var n int
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n
}

func dedupeKoolearnEntries(in []*extractor.MediaInfo) []*extractor.MediaInfo {
	seen := map[string]bool{}
	out := make([]*extractor.MediaInfo, 0, len(in))
	for _, mi := range in {
		if mi == nil {
			continue
		}
		key := mi.Title
		for _, st := range mi.Streams {
			if len(st.URLs) > 0 {
				key = st.URLs[0]
				break
			}
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, mi)
	}
	return out
}
