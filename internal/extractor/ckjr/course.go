package ckjr

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

const (
	ckjrListPageSize = 50
	ckjrDirPageSize  = 100
)

var typeRouteMap = map[int]string{
	0:   "video",
	1:   "voice",
	2:   "imgText",
	8:   "datum",
	9:   "column",
	38:  "column",
	48:  "package",
	51:  "live",
	61:  "package",
	62:  "package",
	70:  "package",
	72:  "live",
	110: "video",
	111: "voice",
	112: "imgText",
	124: "live",
	125: "testPaper",
	173: "live",
	174: "livePersonal",
	180: "livePersonal",
}

type ckjrCourse struct {
	ID          string
	Title       string
	Kind        string
	ProdType    string
	CourseType  string
	URL         string
	Price       float64
	Purchased   bool
	DisplayType string
	Raw         map[string]any
}

type ckjrLessonNode struct {
	Node    map[string]any
	Prefix  []int
	Chapter string
	Index   int
}

func fetchCourseList(c *util.Client, headers map[string]string, base routeInfo) ([]ckjrCourse, error) {
	merged := map[string]ckjrCourse{}
	for page := 1; page <= 99; page++ {
		payload, err := requestAPI(c, "/api/marketingAward/getMarketingAwardList", map[string]string{
			"name":     "",
			"page":     fmt.Sprint(page),
			"limit":    fmt.Sprint(ckjrListPageSize),
			"prodType": "0",
		}, headers)
		if err != nil {
			return nil, err
		}
		if !apiResponseOK(payload) {
			return nil, fmt.Errorf("ckjr course list failed: code=%s", responseCode(payload))
		}
		rows := extractCourseListItems(payload, "", base)
		for _, course := range rows {
			if course.ID == "" {
				continue
			}
			key := course.Kind + ":" + course.ID
			merged[key] = mergeCourse(merged[key], course)
		}
		pageRows := extractPageRows(payload)
		if len(pageRows) == 0 || !pageHasMore(payload, page, ckjrListPageSize, len(pageRows)) {
			break
		}
	}
	out := make([]ckjrCourse, 0, len(merged))
	for _, course := range merged {
		out = append(out, course)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return routeSortRank(out[i].Kind) < routeSortRank(out[j].Kind)
		}
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	return out, nil
}

func extractCourseListItems(payload any, routeKind string, base routeInfo) []ckjrCourse {
	rows := extractPageRows(payload)
	out := make([]ckjrCourse, 0, len(rows))
	for _, row := range rows {
		if !courseListItemHasPermission(row) {
			continue
		}
		course := courseFromNode(row, routeKind, base)
		if course.ID == "" || course.Title == "" {
			continue
		}
		out = append(out, course)
	}
	return out
}

func courseFromNode(node map[string]any, routeKind string, base routeInfo) ckjrCourse {
	kind := resolveCourseListKind(node, routeKind)
	if _, ok := routeCfg[kind]; !ok {
		return ckjrCourse{}
	}
	id := courseListResourceID(kind, node, "")
	if id == "" {
		return ckjrCourse{}
	}
	cfg := routeCfg[kind]
	courseType := firstNonEmpty(textValue(node, "courseType", "childType", "type", "detailType"), cfg.CourseTyp)
	if kind == "video" || kind == "voice" || kind == "imgText" || kind == "column" {
		courseType = firstNonEmpty(textValue(node, "courseType", "childType", "type", "detailType"), cfg.CourseTyp)
	} else {
		courseType = cfg.CourseTyp
	}
	prodType := firstNonEmpty(cfg.ProdType, textValue(node, "prodType", "productType", "ckFrom"))
	course := ckjrCourse{
		ID:          id,
		Title:       util.SanitizeFilename(firstNonEmpty(textValue(node, "prodName", "productName", "name", "title", "courseName", "courseTitle", "columnName", "liveName", "datumName", "paperName", "testName", "testTitle"), id)),
		Kind:        kind,
		ProdType:    prodType,
		CourseType:  courseType,
		Price:       normalizePrice(firstNonEmpty(textValue(node, "preferentialPrice", "price", "salePrice", "sellPrice", "originalPrice", "amount"))),
		Purchased:   true,
		DisplayType: routeDisplayName(kind),
		Raw:         node,
	}
	if v := firstBoolText(node, "hasPermission", "has_permission", "isBuy", "isPaid", "paid", "isPay", "bought", "isPurchased"); v != "" {
		course.Purchased = coerceBool(v, true)
	}
	course.URL = buildCourseURL(base, course)
	return course
}

func mergeCourse(current, incoming ckjrCourse) ckjrCourse {
	if current.ID == "" {
		return incoming
	}
	if incoming.Title != "" {
		current.Title = incoming.Title
	}
	if incoming.URL != "" {
		current.URL = incoming.URL
	}
	if incoming.ProdType != "" {
		current.ProdType = incoming.ProdType
	}
	if incoming.CourseType != "" {
		current.CourseType = incoming.CourseType
	}
	if incoming.Price > 0 {
		current.Price = incoming.Price
	}
	if incoming.Purchased {
		current.Purchased = true
	}
	if len(incoming.Raw) > 0 {
		current.Raw = incoming.Raw
	}
	return current
}

func courseListMedia(base routeInfo, courses []ckjrCourse) *extractor.MediaInfo {
	entries := make([]*extractor.MediaInfo, 0, len(courses))
	for _, course := range courses {
		entries = append(entries, &extractor.MediaInfo{
			Site:  "ckjr",
			Title: util.SanitizeFilename(firstNonEmpty(course.Title, course.ID)),
			Extra: map[string]any{
				"url":          course.URL,
				"course_id":    course.ID,
				"route_kind":   course.Kind,
				"prod_type":    course.ProdType,
				"course_type":  course.CourseType,
				"display_type": course.DisplayType,
				"price":        course.Price,
				"purchased":    course.Purchased,
				"course":       course.Raw,
			},
		})
	}
	title := "ckjr_courses"
	if base.Company != "" {
		title = "ckjr_" + base.Company + "_courses"
	}
	return &extractor.MediaInfo{Site: "ckjr", Title: util.SanitizeFilename(title), Entries: entries}
}

func buildCourseURL(base routeInfo, course ckjrCourse) string {
	if u := firstNonEmpty(textValue(course.Raw, "url", "courseUrl", "courseURL", "shareUrl", "link")); u != "" {
		return normalizeRouteURL(u, base)
	}
	fragment := map[string]string{
		"testPaper":    "/homePage/testPaper/testDetail?testId=%s&ckFrom=125",
		"livePersonal": "/homePage/live/livePersonalDetail?liveId=%s&ckFrom=180",
		"live":         "/homePage/live/liveDetail?liveId=%s&ckFrom=51",
		"datum":        "/homePage/datum/datumDetail?datumId=%s&ckFrom=8",
		"package":      "/homePage/package/packageDetail?combosId=%s",
		"column":       "/homePage/column/columnDetail?cId=-1&ckFrom=9&extId=%s",
		"imgText":      "/homePage/course/imgText?courseId=%s&ckFrom=5&extId=-1",
		"voice":        "/homePage/course/voice?courseId=%s&ckFrom=5&extId=-1",
		"video":        "/homePage/course/video?courseId=%s&ckFrom=5&extId=-1",
	}[course.Kind]
	if fragment == "" || course.ID == "" {
		return ""
	}
	root := base.BaseURL
	if root == "" && base.Company != "" {
		root = url0 + "/kpv2p/" + base.Company + "/"
	}
	if root == "" {
		root = url0 + "/kpv2p/"
	}
	return strings.TrimRight(root, "/") + "#" + fmt.Sprintf(fragment, url.QueryEscape(course.ID))
}

func normalizeRouteURL(raw string, base routeInfo) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(raw), "http") {
		return raw
	}
	root := base.BaseURL
	if root == "" && base.Company != "" {
		root = url0 + "/kpv2p/" + base.Company + "/"
	}
	if root == "" {
		root = url0
	}
	if strings.HasPrefix(raw, "#") {
		return strings.TrimRight(root, "/") + raw
	}
	if strings.HasPrefix(raw, "/") && strings.Contains(raw, "homePage/") {
		return strings.TrimRight(root, "/") + "#" + raw
	}
	return ckjrJoinURL(root, raw)
}

func courseListItemHasPermission(item map[string]any) bool {
	if len(item) == 0 {
		return false
	}
	if intLike(item["prodStatus"], 1) == 0 {
		return false
	}
	if st := intLike(item["status"], 1); st == -3 || st == -6 {
		return false
	}
	for _, key := range []string{"hasPermission", "has_permission", "isBuy", "isPaid", "paid", "isPay", "bought", "isPurchased"} {
		if v, ok := item[key]; ok && !coerceBool(fmt.Sprint(v), false) {
			return false
		}
	}
	return true
}

func resolveCourseListKind(item map[string]any, routeKind string) string {
	if routeKind != "" {
		if _, ok := routeCfg[routeKind]; ok {
			return routeKind
		}
	}
	prodType := intLike(firstNonEmpty(textValue(item, "prodType"), textValue(item, "ckFrom")), 0)
	if prodType == 5 {
		return firstNonEmpty(typeRouteMap[intLike(firstNonEmpty(textValue(item, "courseType"), textValue(item, "childType")), 0)], "video")
	}
	if kind := typeRouteMap[prodType]; kind != "" {
		return kind
	}
	typ := intLike(item["type"], 0)
	switch {
	case typ == 4 && textValue(item, "columnId", "extId") != "":
		return "column"
	case (typ == 0 || typ == 1 || typ == 2) && textValue(item, "courseId", "prodId", "productId") != "":
		return firstNonEmpty(typeRouteMap[typ], "video")
	case typ == 5 && textValue(item, "datumId") != "":
		return "datum"
	case typ == 8 && textValue(item, "liveId", "id") != "":
		return "live"
	}
	return ""
}
