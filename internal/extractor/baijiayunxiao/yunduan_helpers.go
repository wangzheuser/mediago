package baijiayunxiao

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/Sophomoresty/mediago/internal/util"
)

func fetchYunduanPagedList(c *util.Client, apiURL string, params map[string]string, headers map[string]string, pageSize int, maxPages int) []map[string]any {
	if pageSize <= 0 {
		pageSize = 100
	}
	if maxPages <= 0 {
		maxPages = 20
	}
	seen := map[string]bool{}
	var out []map[string]any
	for page := 1; page <= maxPages; page++ {
		query := map[string]string{}
		for k, v := range params {
			if strings.TrimSpace(v) != "" {
				query[k] = v
			}
		}
		query["page"] = strconv.Itoa(page)
		query["page_size"] = strconv.Itoa(pageSize)
		query["pageSize"] = strconv.Itoa(pageSize)
		payload := requestYunduanJSON(c, apiURL, query, headers)
		items, total := extractYunduanListAndTotal(payload)
		if len(items) == 0 {
			if page == 1 {
				items, total = extractYunduanListAndTotal(valueMap(payload["data"]))
			}
			if len(items) == 0 {
				break
			}
		}
		before := len(out)
		for _, item := range items {
			key := yunduanListItemKey(item)
			if key != "" && seen[key] {
				continue
			}
			if key != "" {
				seen[key] = true
			}
			out = append(out, item)
		}
		if total > 0 && len(out) >= total {
			break
		}
		if len(out) == before || len(items) < pageSize {
			break
		}
	}
	return out
}

func requestYunduanJSON(c *util.Client, apiURL string, params map[string]string, headers map[string]string) map[string]any {
	if apiURL == "" {
		return map[string]any{}
	}
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return map[string]any{}
	}
	q := parsed.Query()
	for k, v := range params {
		if strings.TrimSpace(v) != "" {
			q.Set(k, v)
		}
	}
	parsed.RawQuery = q.Encode()
	body, err := c.GetString(parsed.String(), headers)
	if err != nil {
		return map[string]any{}
	}
	return decodeJSONMap(body)
}

func extractYunduanListAndTotal(payload map[string]any) ([]map[string]any, int) {
	var candidates []any
	data := valueMap(payload["data"])
	if len(data) > 0 {
		candidates = append(candidates, data["list"], data["rows"], data["records"], data["items"])
	} else {
		candidates = append(candidates, payload["data"])
	}
	candidates = append(candidates, payload["list"], payload["rows"], payload["records"], payload["items"])
	var items []map[string]any
	for _, candidate := range candidates {
		items = recordsValue(candidate)
		if len(items) > 0 {
			break
		}
	}
	if len(items) == 0 && len(data) > 0 && yunduanMapLooksLikeItem(data) {
		items = []map[string]any{data}
	}
	if len(items) == 0 && yunduanMapLooksLikeItem(payload) {
		items = []map[string]any{payload}
	}
	total := firstPositiveInt(payload["total"], payload["total_count"], payload["totalCount"], payload["count"])
	if data := valueMap(payload["data"]); total == 0 && len(data) > 0 {
		total = firstPositiveInt(data["total"], data["total_count"], data["totalCount"], data["count"])
	}
	return items, total
}

func yunduanMapLooksLikeItem(item map[string]any) bool {
	if len(item) == 0 {
		return false
	}
	for _, key := range []string{"list", "rows", "records", "items"} {
		if _, ok := item[key]; ok {
			return false
		}
	}
	return firstFromAliases(item,
		"lesson_id", "lessonId", "video_id", "videoId", "room_id", "roomId",
		"classid", "class_id", "classId", "course_id", "courseId",
		"playback_id", "playbackId", "record_id", "recordId", "id",
	) != ""
}

func filterYunduanPlayableLessons(lessons []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(lessons))
	for _, item := range lessons {
		if len(item) == 0 {
			continue
		}
		roomID, token := extractYunduanRoomToken(item)
		id := firstNonEmpty(anyString(item["video_id"]), anyString(item["videoId"]), anyString(item["id"]), anyString(item["lesson_id"]), anyString(item["lessonId"]), anyString(item["playback_id"]), anyString(item["playbackId"]), anyString(item["record_id"]), anyString(item["recordId"]))
		if roomID != "" || token != "" || id != "" {
			out = append(out, item)
		}
	}
	return out
}

func extractYunduanRoomToken(lesson map[string]any) (string, string) {
	playURL := normalizeMediaURL(firstNonEmpty(anyString(lesson["play_url"]), anyString(lesson["playUrl"]), anyString(lesson["playback_url"]), anyString(lesson["playbackUrl"]), anyString(lesson["url"])))
	roomID := firstNonEmpty(anyString(lesson["room_id"]), anyString(lesson["roomId"]), anyString(lesson["classid"]), anyString(lesson["class_id"]), anyString(lesson["classId"]))
	token := firstNonEmpty(anyString(lesson["player_token"]), anyString(lesson["playerToken"]), anyString(lesson["token"]), anyString(lesson["play_token"]), anyString(lesson["playToken"]), anyString(lesson["access_token"]), anyString(lesson["accessToken"]))
	if playURL != "" {
		if p, ok := parseYunduanPlaybackParamsFromURL(playURL); ok {
			roomID = firstNonEmpty(roomID, p.roomID, p.vid)
			token = firstNonEmpty(token, p.token)
		}
	}
	return roomID, token
}

func parseYunduanPlaybackParamsFromLesson(lesson map[string]any) (playbackParams, bool) {
	for _, key := range []string{"play_url", "playUrl", "playback_url", "playbackUrl", "url"} {
		if p, ok := parseYunduanPlaybackParamsFromURL(anyString(lesson[key])); ok {
			return p, true
		}
	}
	return playbackParams{}, false
}

func parseYunduanPlaybackParamsFromURL(raw string) (playbackParams, bool) {
	if p, ok := parsePlaybackParams(raw); ok {
		return p, true
	}
	return playbackParams{}, false
}

func mergeYunduanLessonLists(lists ...[]map[string]any) []map[string]any {
	seen := map[string]bool{}
	var out []map[string]any
	for _, list := range lists {
		for _, item := range list {
			keys := yunduanLessonIdentityKeys(item)
			duplicate := false
			for _, key := range keys {
				if seen[key] {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			for _, key := range keys {
				seen[key] = true
			}
			out = append(out, item)
		}
	}
	return out
}

func yunduanLessonIdentityKeys(item map[string]any) []string {
	var keys []string
	for _, group := range []struct {
		name    string
		aliases []string
	}{
		{"lesson_id", []string{"lesson_id", "lessonId"}},
		{"video_id", []string{"video_id", "videoId"}},
		{"playback_id", []string{"playback_id", "playbackId"}},
		{"record_id", []string{"record_id", "recordId"}},
		{"id", []string{"id"}},
	} {
		if v := firstFromAliases(item, group.aliases...); v != "" {
			keys = append(keys, group.name+"="+v)
		}
	}
	roomID := firstFromAliases(item, "room_id", "roomId", "classid", "class_id", "classId")
	contextParts := []string{}
	for _, group := range []struct {
		name    string
		aliases []string
	}{
		{"title", []string{"title", "name"}},
		{"start_time", []string{"start_time", "startTime", "date", "upload_date", "uploadDate"}},
		{"length", []string{"length", "duration"}},
	} {
		if v := firstFromAliases(item, group.aliases...); v != "" {
			contextParts = append(contextParts, group.name+"="+v)
		}
	}
	if roomID != "" && len(contextParts) > 0 {
		keys = append(keys, "room_context="+roomID+"|"+strings.Join(contextParts, "|"))
	}
	if len(keys) == 0 {
		keys = append(keys, yunduanListItemKey(item))
	}
	return keys
}

func yunduanListItemKey(item map[string]any) string {
	parts := yunduanItemKeyParts(item,
		[][2][]string{
			{{"lesson_id"}, {"lesson_id", "lessonId"}},
			{{"video_id"}, {"video_id", "videoId"}},
			{{"room_id"}, {"room_id", "roomId", "classid", "class_id", "classId"}},
			{{"course_id"}, {"course_id", "courseId"}},
			{{"playback_id"}, {"playback_id", "playbackId"}},
			{{"record_id"}, {"record_id", "recordId"}},
			{{"id"}, {"id"}},
		},
		[][2][]string{
			{{"title"}, {"title", "name"}},
			{{"start_time"}, {"start_time", "startTime", "date", "upload_date", "uploadDate"}},
			{{"length"}, {"length", "duration"}},
		},
		[][2][]string{
			{{"play_url"}, {"play_url", "playUrl"}},
			{{"token"}, {"player_token", "playerToken", "token", "play_token", "playToken", "access_token", "accessToken"}},
		},
	)
	if len(parts) > 0 {
		return strings.Join(parts, "|")
	}
	return stableMapKey(item)
}

func yunduanItemKeyParts(item map[string]any, primary, context, fallback [][2][]string) []string {
	var parts []string
	for _, group := range primary {
		if v := firstFromAliases(item, group[1]...); v != "" {
			parts = append(parts, group[0][0]+"="+v)
		}
	}
	for _, group := range context {
		if v := firstFromAliases(item, group[1]...); v != "" {
			parts = append(parts, group[0][0]+"="+v)
		}
	}
	if len(parts) == 0 {
		for _, group := range fallback {
			if v := firstFromAliases(item, group[1]...); v != "" {
				parts = append(parts, group[0][0]+"="+v)
			}
		}
	}
	return parts
}

func mayHaveYunduanPlayback(item map[string]any) bool {
	count := yunduanPlaybackCountHint(item)
	if count >= 0 {
		return count > 0
	}
	return true
}

func yunduanPlaybackCountHint(item map[string]any) int {
	for _, key := range []string{"playback_count", "playbackCount", "lesson_count", "lessonCount", "total", "total_count", "totalCount"} {
		if v, ok := item[key]; ok {
			s := anyString(v)
			if s == "" {
				continue
			}
			n, err := strconv.Atoi(strings.Split(s, ".")[0])
			if err != nil {
				return -1
			}
			return n
		}
	}
	return -1
}

func buildYunduanVideoName(index int, lesson map[string]any) string {
	title := firstNonEmpty(anyString(lesson["title"]), anyString(lesson["name"]), "回放")
	date := firstNonEmpty(anyString(lesson["upload_date"]), anyString(lesson["uploadDate"]), anyString(lesson["date"]), anyString(lesson["start_time"]), anyString(lesson["startTime"]))
	if len(date) > 10 {
		date = date[:10]
	}
	duration := formatYunduanDuration(firstNonEmpty(anyString(lesson["length"]), anyString(lesson["duration"])))
	parts := []string{}
	if date != "" {
		parts = append(parts, date)
	}
	parts = append(parts, title)
	if duration != "" {
		parts = append(parts, duration)
	}
	return util.SanitizeFilename(fmt.Sprintf("[%d]--%s", index, strings.Join(parts, "--")))
}

func formatYunduanDuration(raw string) string {
	if raw == "" {
		return ""
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
	if err != nil {
		return ""
	}
	seconds := int(f)
	if seconds <= 0 {
		return ""
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%02d-%02d-%02d", h, m, s)
	}
	return fmt.Sprintf("%02d-%02d", m, s)
}

func selectYunduanCourses(courses []yunduanCourse, target yunduanTarget) []yunduanCourse {
	var selected []yunduanCourse
	for _, course := range courses {
		if target.source != "" && target.source != "long_room" {
			switch target.source {
			case "recent_course", "recent_class":
				if course.Source != target.source {
					continue
				}
			case "course", "api_course":
				if course.Source != "course" && course.Source != "api_course" {
					continue
				}
			case "long_term":
				if course.Source != "long_term" {
					continue
				}
			}
		}
		if target.courseID != "" {
			cid := firstNonEmpty(anyString(course.Payload["course_id"]), anyString(course.Payload["courseId"]), course.ID)
			if cid == target.courseID {
				selected = append(selected, course)
				continue
			}
		}
		if target.roomID != "" {
			roomID := firstNonEmpty(anyString(course.Payload["room_id"]), anyString(course.Payload["roomId"]), course.ID)
			if roomID == target.roomID {
				selected = append(selected, course)
			}
		}
	}
	return selected
}

func buildDirectYunduanCourse(c *util.Client, domain string, headers map[string]string, target yunduanTarget) yunduanCourse {
	switch target.source {
	case "recent_course":
		lessons := filterYunduanPlayableLessons(fetchYunduanPagedList(c, fmt.Sprintf(yunduanCourseRecentURL, domain), nil, headers, 100, 20))
		if target.courseID != "" {
			lessons = filterYunduanLessonsByField(lessons, "course_id", target.courseID)
		}
		return yunduanCourse{ID: firstNonEmpty(target.courseID, "recent_course"), Title: "近期小班课回放", Source: "recent_course", IDName: "id", Payload: map[string]any{"id": "recent_course", "source": "recent_course"}, Lessons: lessons}
	case "recent_class":
		lessons := filterYunduanPlayableLessons(fetchYunduanPagedList(c, fmt.Sprintf(yunduanClassRecentURL, domain), nil, headers, 100, 20))
		if target.roomID != "" {
			lessons = filterYunduanLessonsByField(lessons, "room_id", target.roomID)
		}
		return yunduanCourse{ID: firstNonEmpty(target.roomID, "recent_class"), Title: "近期班课回放", Source: "recent_class", IDName: "id", Payload: map[string]any{"id": "recent_class", "source": "recent_class"}, Lessons: lessons}
	case "long_term", "long_room":
		roomID := target.roomID
		if roomID == "" {
			break
		}
		lessons := filterYunduanPlayableLessons(mergeYunduanLessonLists(
			fetchYunduanPagedList(c, fmt.Sprintf(yunduanLongLessonURL, domain), map[string]string{"room_id": roomID}, headers, 100, 20),
			fetchYunduanPagedList(c, fmt.Sprintf(yunduanShortLessonURL, domain), map[string]string{"room_id": roomID}, headers, 100, 20),
		))
		return yunduanCourse{ID: roomID, Title: "云端课堂班课回放", Source: "long_term", IDName: "room_id", Payload: map[string]any{"id": roomID, "room_id": roomID, "source": "long_term"}, Lessons: lessons}
	default:
		if target.courseID != "" {
			lessons := filterYunduanPlayableLessons(mergeYunduanLessonLists(
				fetchYunduanPagedList(c, fmt.Sprintf(yunduanCourseLessonURL, domain), map[string]string{"course_id": target.courseID}, headers, 100, 20),
				fetchYunduanPagedList(c, fmt.Sprintf(yunduanAPILessonURL, domain), map[string]string{"course_id": target.courseID}, headers, 100, 20),
			))
			return yunduanCourse{ID: target.courseID, Title: "云端课堂课程", Source: "course", IDName: "course_id", Payload: map[string]any{"id": target.courseID, "course_id": target.courseID, "source": "course"}, Lessons: lessons}
		}
	}
	return yunduanCourse{}
}

func filterYunduanLessonsByField(lessons []map[string]any, field string, value string) []map[string]any {
	if value == "" {
		return lessons
	}
	out := make([]map[string]any, 0, len(lessons))
	alt := camelAlias(field)
	for _, lesson := range lessons {
		if firstNonEmpty(anyString(lesson[field]), anyString(lesson[alt])) == value {
			out = append(out, lesson)
		}
	}
	return out
}

func camelAlias(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) <= 1 {
		return s
	}
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

func normalizeYunduanDomain(domain string) string {
	domain = strings.TrimSpace(strings.Trim(domain, "/"))
	domain = strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(domain), "https://"), "http://")
	if i := strings.IndexAny(domain, "/?#"); i >= 0 {
		domain = domain[:i]
	}
	if yunduanDomainTextRe.MatchString(domain) && yunduanDomainTextRe.FindString(domain) == domain {
		return domain
	}
	return ""
}

func yunduanDomainFromCookies(jar http.CookieJar) string {
	if jar == nil {
		return ""
	}
	hosts := []string{"www.baijiayun.com", "baijiayun.com"}
	for _, host := range hosts {
		for _, ck := range jar.Cookies(&url.URL{Scheme: "https", Host: host, Path: "/"}) {
			if strings.EqualFold(ck.Name, "YUNDUN_DOMAIN") || strings.EqualFold(ck.Name, "YUNDUAN_DOMAIN") || strings.EqualFold(ck.Name, "domain") {
				if domain := normalizeYunduanDomain(ck.Value); domain != "" {
					return domain
				}
			}
		}
	}
	return ""
}

func yunduanDomainFromHeader(cookie string) string {
	if m := yunduanCookieDomainRe.FindStringSubmatch(cookie); m != nil {
		return normalizeYunduanDomain(m[1])
	}
	return ""
}

func discoverYunduanDomain(c *util.Client, headers map[string]string) string {
	for _, rawURL := range []string{yunduanEntryURL, "https://www.baijiayun.com/entry/"} {
		body, err := c.GetString(rawURL, headers)
		if err != nil {
			continue
		}
		if domain := normalizeYunduanDomain(yunduanDomainTextRe.FindString(body)); domain != "" {
			return domain
		}
	}
	return ""
}

func yunduanCookieHeader(jar http.CookieJar, domain string, existing ...string) string {
	seen := map[string]bool{}
	var parts []string
	for _, cookie := range existing {
		for _, part := range strings.Split(cookie, ";") {
			part = strings.TrimSpace(part)
			if part == "" || !strings.Contains(part, "=") {
				continue
			}
			name := strings.TrimSpace(strings.SplitN(part, "=", 2)[0])
			if name == "" || seen[strings.ToLower(name)] {
				continue
			}
			seen[strings.ToLower(name)] = true
			parts = append(parts, part)
		}
	}
	if jar != nil {
		for _, host := range []string{domain, "www.baijiayun.com", "baijiayun.com"} {
			for _, ck := range jar.Cookies(&url.URL{Scheme: "https", Host: host, Path: "/"}) {
				if ck.Value == "" || seen[strings.ToLower(ck.Name)] {
					continue
				}
				seen[strings.ToLower(ck.Name)] = true
				parts = append(parts, ck.Name+"="+ck.Value)
			}
		}
	}
	return strings.Join(parts, "; ")
}

func firstFromAliases(m map[string]any, aliases ...string) string {
	for _, key := range aliases {
		if v := anyString(m[key]); v != "" {
			return v
		}
	}
	return ""
}

func firstPositiveInt(values ...any) int {
	for _, value := range values {
		s := anyString(value)
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(strings.Split(s, ".")[0])
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stableMapKey(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+anyString(m[key]))
	}
	return strings.Join(parts, "|")
}
