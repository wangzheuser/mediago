package baijiayunxiao

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

const (
	yunduanEntryURL = "https://www.baijiayun.com/entry"

	yunduanAccountURL      = "https://%s/org/account/getUserInfo"
	yunduanCourseListURL   = "https://%s/org/course_playback/getCourseList"
	yunduanCourseRecentURL = "https://%s/org/course_playback/getRecentList"
	yunduanCourseLessonURL = "https://%s/org/course_playback/getLessonList"
	yunduanAPILessonURL    = "https://%s/org/course_playback/getApiLessonList"
	yunduanLongRoomURL     = "https://%s/org/class_playback/getLongTermRoomList"
	yunduanClassRecentURL  = "https://%s/org/class_playback/getRecentList"
	yunduanLongLessonURL   = "https://%s/org/class_playback/getLongTermList"
	yunduanShortLessonURL  = "https://%s/org/class_playback/getShortTermList"
)

var (
	yunduanDomainRe       = regexp.MustCompile(`(?i)^https?://([a-z0-9][a-z0-9-]*\.at\.baijiayun\.com)(?:[/?#]|$)`)
	yunduanCookieDomainRe = regexp.MustCompile(`(?i)(?:^|;)\s*(?:YUNDUN_DOMAIN|YUNDUAN_DOMAIN|domain)=([^;]+)`)
	yunduanDomainTextRe   = regexp.MustCompile(`(?i)([a-z0-9][a-z0-9-]*\.at\.baijiayun\.com)`)
)

type yunduanTarget struct {
	domain   string
	courseID string
	roomID   string
	source   string
}

type yunduanCourse struct {
	ID      string
	Title   string
	Source  string
	IDName  string
	Payload map[string]any
	Lessons []map[string]any
}

type yunduanLesson struct {
	ID        string
	LessonID  string
	RoomID    string
	Token     string
	SessionID string
	Title     string
	Payload   map[string]any
}

func parseYunduanTarget(raw string) (yunduanTarget, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return yunduanTarget{}, false
	}
	host := strings.ToLower(parsed.Host)
	q := parsed.Query()
	target := yunduanTarget{
		courseID: firstNonEmpty(q.Get("course_id"), q.Get("courseId")),
		roomID:   firstNonEmpty(q.Get("room_id"), q.Get("roomId"), q.Get("classid"), q.Get("class_id"), q.Get("classId")),
	}
	if m := yunduanDomainRe.FindStringSubmatch(raw); m != nil {
		target.domain = normalizeYunduanDomain(m[1])
	}
	if target.domain == "" && (host == "www.baijiayun.com" || host == "baijiayun.com") && strings.EqualFold(parsed.Path, "/entry") {
		return target, true
	}
	switch strings.ToLower(parsed.Path) {
	case "/org/course_playback/getlessonlist":
		target.source = "course"
	case "/org/course_playback/getapilessonlist":
		target.source = "api_course"
	case "/org/course_playback/getrecentlist":
		target.source = "recent_course"
	case "/org/class_playback/getlongtermlist", "/org/class_playback/getshorttermlist":
		target.source = "long_term"
	case "/org/class_playback/getrecentlist":
		target.source = "recent_class"
	case "/org/class_playback/getlongtermroomlist":
		target.source = "long_room"
	}
	if target.domain != "" {
		return target, true
	}
	return yunduanTarget{}, false
}

func resolveYunduan(c *util.Client, target yunduanTarget, jar http.CookieJar, headers map[string]string) (*extractor.MediaInfo, error) {
	domain := firstNonEmpty(target.domain, yunduanDomainFromCookies(jar), yunduanDomainFromHeader(headers["Cookie"]), yunduanDomainFromHeader(headers["cookie"]))
	if domain == "" {
		if discovered := discoverYunduanDomain(c, headers); discovered != "" {
			domain = discovered
		}
	}
	domain = normalizeYunduanDomain(domain)
	if domain == "" {
		return nil, fmt.Errorf("baijiayunxiao yunduan: missing *.at.baijiayun.com domain")
	}

	headers = cloneHeaders(headers)
	headers["Referer"] = "https://" + domain + "/web/course/index"
	headers["Origin"] = "https://" + domain
	if cookie := yunduanCookieHeader(jar, domain, headers["Cookie"], headers["cookie"]); cookie != "" {
		headers["Cookie"] = cookie
		headers["cookie"] = cookie
	}
	if !validateYunduanLogin(c, domain, headers) {
		return nil, fmt.Errorf("baijiayunxiao yunduan: invalid ORGSUPERSESSID cookie for %s", domain)
	}

	courses := fetchYunduanCourses(c, domain, headers)
	if target.courseID != "" || target.roomID != "" {
		courses = selectYunduanCourses(courses, target)
	}
	if len(courses) == 0 {
		if direct := buildDirectYunduanCourse(c, domain, headers, target); len(direct.Lessons) > 0 {
			courses = []yunduanCourse{direct}
		}
	}
	if len(courses) == 0 {
		return nil, fmt.Errorf("baijiayunxiao yunduan: no downloadable courses from org playback APIs")
	}

	entries := make([]*extractor.MediaInfo, 0, len(courses))
	for _, course := range courses {
		courseEntries := resolveYunduanCourseEntries(c, domain, course, headers)
		if len(courseEntries) == 0 {
			continue
		}
		if len(courses) > 1 {
			prefixYunduanEntryTitles(courseEntries, firstNonEmpty(course.Title, course.ID))
		}
		entries = append(entries, courseEntries...)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("baijiayunxiao yunduan: parsed courses but no baijiayun stream resolved")
	}
	title := "云端课堂课程"
	if len(courses) == 1 {
		title = firstNonEmpty(courses[0].Title, title)
	}
	return &extractor.MediaInfo{
		Site:    "baijiayunxiao",
		Title:   util.SanitizeFilename(title),
		Entries: entries,
		Extra:   map[string]any{"platform": "yunduan", "domain": domain},
	}, nil
}

func prefixYunduanEntryTitles(entries []*extractor.MediaInfo, prefix string) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return
	}
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if strings.HasPrefix(entry.Title, prefix+" - ") {
			continue
		}
		entry.Title = util.SanitizeFilename(prefix + " - " + entry.Title)
	}
}

func fetchYunduanCourses(c *util.Client, domain string, headers map[string]string) []yunduanCourse {
	seen := map[string]bool{}
	var out []yunduanCourse

	for _, item := range fetchYunduanPagedList(c, fmt.Sprintf(yunduanCourseListURL, domain), nil, headers, 100, 20) {
		id := firstNonEmpty(anyString(item["course_id"]), anyString(item["courseId"]), anyString(item["id"]))
		title := firstNonEmpty(anyString(item["title"]), anyString(item["name"]), anyString(item["courseName"]))
		if id == "" {
			continue
		}
		course := buildYunduanCourse(item, "course", id, "course_id", title)
		if appendYunduanDownloadableCourse(c, domain, headers, &out, seen, "course:"+id, &course) {
			continue
		}
		course.Source = "api_course"
		course.Lessons = filterYunduanPlayableLessons(fetchYunduanPagedList(c, fmt.Sprintf(yunduanAPILessonURL, domain), map[string]string{"course_id": id}, headers, 100, 20))
		appendYunduanCourse(&out, seen, "api_course:"+id, course)
	}

	for _, item := range fetchYunduanPagedList(c, fmt.Sprintf(yunduanLongRoomURL, domain), nil, headers, 100, 20) {
		id := firstNonEmpty(anyString(item["room_id"]), anyString(item["roomId"]), anyString(item["id"]))
		title := firstNonEmpty(anyString(item["title"]), anyString(item["name"]), anyString(item["roomName"]))
		if id == "" {
			continue
		}
		course := buildYunduanCourse(item, "long_term", id, "room_id", title)
		appendYunduanDownloadableCourse(c, domain, headers, &out, seen, "long:"+id, &course)
	}

	if len(out) == 0 {
		if lessons := filterYunduanPlayableLessons(fetchYunduanPagedList(c, fmt.Sprintf(yunduanCourseRecentURL, domain), nil, headers, 100, 20)); len(lessons) > 0 {
			appendYunduanCourse(&out, seen, "recent_course", yunduanCourse{ID: "recent_course", Title: "近期小班课回放", Source: "recent_course", IDName: "id", Payload: map[string]any{"id": "recent_course", "source": "recent_course"}, Lessons: lessons})
		}
		if lessons := filterYunduanPlayableLessons(fetchYunduanPagedList(c, fmt.Sprintf(yunduanClassRecentURL, domain), nil, headers, 100, 20)); len(lessons) > 0 {
			appendYunduanCourse(&out, seen, "recent_class", yunduanCourse{ID: "recent_class", Title: "近期班课回放", Source: "recent_class", IDName: "id", Payload: map[string]any{"id": "recent_class", "source": "recent_class"}, Lessons: lessons})
		}
	}
	return out
}

func validateYunduanLogin(c *util.Client, domain string, headers map[string]string) bool {
	if domain == "" {
		return false
	}
	cookie := firstNonEmpty(headers["Cookie"], headers["cookie"])
	if !strings.Contains(strings.ToUpper(cookie), "ORGSUPERSESSID=") {
		return false
	}
	payload := requestYunduanJSON(c, fmt.Sprintf(yunduanAccountURL, domain), nil, headers)
	code := anyString(payload["code"])
	return code == "0" || code == "200"
}

func buildYunduanCourse(item map[string]any, source, id, idName, title string) yunduanCourse {
	payload := cloneMap(item)
	payload["title"] = firstNonEmpty(title, source+"_"+id)
	payload["source"] = source
	payload[idName] = id
	payload["id"] = id
	return yunduanCourse{ID: id, Title: anyString(payload["title"]), Source: source, IDName: idName, Payload: payload}
}

func appendYunduanDownloadableCourse(c *util.Client, domain string, headers map[string]string, out *[]yunduanCourse, seen map[string]bool, key string, course *yunduanCourse) bool {
	if course == nil || seen[key] {
		return false
	}
	if !mayHaveYunduanPlayback(course.Payload) {
		return false
	}
	lessons := filterYunduanPlayableLessons(getYunduanCourseLessons(c, domain, headers, course))
	if len(lessons) == 0 {
		return false
	}
	course.Lessons = lessons
	if course.Payload != nil {
		course.Payload["_yunduan_lessons_resolved"] = true
	}
	return appendYunduanCourse(out, seen, key, *course)
}

func appendYunduanCourse(out *[]yunduanCourse, seen map[string]bool, key string, course yunduanCourse) bool {
	if key == "" || seen[key] || len(course.Lessons) == 0 {
		return false
	}
	seen[key] = true
	*out = append(*out, course)
	return true
}

func getYunduanCourseLessons(c *util.Client, domain string, headers map[string]string, course *yunduanCourse) []map[string]any {
	if course == nil {
		return nil
	}
	if len(course.Lessons) > 0 {
		if resolved, _ := course.Payload["_yunduan_lessons_resolved"].(bool); resolved {
			return append([]map[string]any{}, course.Lessons...)
		}
	}
	var lessons []map[string]any
	countHint := yunduanPlaybackCountHint(course.Payload)
	switch course.Source {
	case "recent_course", "recent_class":
		lessons = append([]map[string]any{}, course.Lessons...)
	case "long_term":
		roomID := firstNonEmpty(anyString(course.Payload["room_id"]), anyString(course.Payload["roomId"]), course.ID)
		lessons = mergeYunduanLessonLists(
			fetchYunduanPagedList(c, fmt.Sprintf(yunduanLongLessonURL, domain), map[string]string{"room_id": roomID}, headers, 100, 20),
			fetchYunduanPagedList(c, fmt.Sprintf(yunduanShortLessonURL, domain), map[string]string{"room_id": roomID}, headers, 100, 20),
		)
		lessons = appendYunduanRecentLessonsIfIncomplete(c, fmt.Sprintf(yunduanClassRecentURL, domain), "room_id", roomID, countHint, lessons, headers)
	case "api_course":
		courseID := firstNonEmpty(anyString(course.Payload["course_id"]), anyString(course.Payload["courseId"]), course.ID)
		lessons = fetchYunduanPagedList(c, fmt.Sprintf(yunduanAPILessonURL, domain), map[string]string{"course_id": courseID}, headers, 100, 20)
	default:
		courseID := firstNonEmpty(anyString(course.Payload["course_id"]), anyString(course.Payload["courseId"]), course.ID)
		lessons = mergeYunduanLessonLists(
			fetchYunduanPagedList(c, fmt.Sprintf(yunduanCourseLessonURL, domain), map[string]string{"course_id": courseID}, headers, 100, 20),
			fetchYunduanPagedList(c, fmt.Sprintf(yunduanAPILessonURL, domain), map[string]string{"course_id": courseID}, headers, 100, 20),
		)
		lessons = appendYunduanRecentLessonsIfIncomplete(c, fmt.Sprintf(yunduanCourseRecentURL, domain), "course_id", courseID, countHint, lessons, headers)
	}
	course.Lessons = lessons
	return append([]map[string]any{}, lessons...)
}

func appendYunduanRecentLessonsIfIncomplete(c *util.Client, apiURL, matchKey, matchValue string, countHint int, lessons []map[string]any, headers map[string]string) []map[string]any {
	if matchValue == "" || countHint <= 0 || len(lessons) >= countHint {
		return lessons
	}
	var matched []map[string]any
	for _, item := range fetchYunduanPagedList(c, apiURL, nil, headers, 100, 20) {
		value := firstNonEmpty(anyString(item[matchKey]), anyString(item[camelAlias(matchKey)]))
		if value == matchValue {
			matched = append(matched, item)
		}
	}
	if len(matched) == 0 {
		return lessons
	}
	return mergeYunduanLessonLists(lessons, matched)
}

func resolveYunduanCourseEntries(c *util.Client, domain string, course yunduanCourse, headers map[string]string) []*extractor.MediaInfo {
	lessons := filterYunduanPlayableLessons(getYunduanCourseLessons(c, domain, headers, &course))
	entries := make([]*extractor.MediaInfo, 0, len(lessons))
	seenMaterials := map[string]bool{}
	appendMaterialEntries(c, domain, &entries, seenMaterials, extractMaterials(course.Payload, firstNonEmpty(course.Title, "资料")), headers)

	for i, payload := range lessons {
		lesson := buildYunduanLesson(i+1, payload)
		if lesson.ID == "" && (lesson.RoomID == "" || lesson.Token == "") {
			continue
		}
		appendMaterialEntries(c, domain, &entries, seenMaterials, extractMaterials(payload, firstNonEmpty(lesson.Title, course.Title, "资料")), headers)

		if lesson.RoomID == "" || lesson.Token == "" {
			if p, ok := parseYunduanPlaybackParamsFromLesson(payload); ok {
				lesson.RoomID = firstNonEmpty(lesson.RoomID, p.roomID, p.vid)
				lesson.Token = firstNonEmpty(lesson.Token, p.token)
			}
		}
		if lesson.RoomID == "" || lesson.Token == "" {
			token, roomID, _, err := fetchLessonToken(c, courseURL{domain: domain, cid: course.ID}, firstNonEmpty(lesson.LessonID, lesson.ID), headers)
			if err == nil {
				lesson.Token = firstNonEmpty(lesson.Token, token)
				lesson.RoomID = firstNonEmpty(lesson.RoomID, roomID)
			}
		}
		if lesson.RoomID == "" || lesson.Token == "" {
			continue
		}

		entry, err := resolvePlayback(c, playbackParams{roomID: lesson.RoomID, token: lesson.Token}, headers, lesson.Title)
		if err != nil {
			continue
		}
		mergeExtra(entry, map[string]any{
			"platform":   "yunduan",
			"source":     course.Source,
			"course_id":  course.ID,
			"lesson_id":  lesson.LessonID,
			"video_id":   lesson.ID,
			"room_id":    lesson.RoomID,
			"session_id": lesson.SessionID,
		})
		entries = append(entries, entry)
		if docURL := firstNonEmpty(anyString(entry.Extra["doc_url"]), anyString(entry.Extra["package_url"])); docURL != "" {
			appendMaterialEntries(c, domain, &entries, seenMaterials, fetchDocMaterials(c, docURL, lesson.Title, headers), headers)
		}
	}
	return entries
}

func buildYunduanLesson(index int, item map[string]any) yunduanLesson {
	roomID, token := extractYunduanRoomToken(item)
	id := firstNonEmpty(
		anyString(item["video_id"]), anyString(item["videoId"]), anyString(item["id"]),
		anyString(item["lesson_id"]), anyString(item["lessonId"]), anyString(item["playback_id"]),
		anyString(item["playbackId"]), anyString(item["record_id"]), anyString(item["recordId"]),
		roomID,
	)
	lessonID := firstNonEmpty(anyString(item["lesson_id"]), anyString(item["lessonId"]), anyString(item["id"]))
	return yunduanLesson{
		ID:        id,
		LessonID:  lessonID,
		RoomID:    roomID,
		Token:     token,
		SessionID: firstNonEmpty(anyString(item["session_id"]), anyString(item["sessionId"])),
		Title:     buildYunduanVideoName(index, item),
		Payload:   item,
	}
}
