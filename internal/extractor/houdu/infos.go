package houdu

import (
	"fmt"
	"strings"
)

func (x *hdCtx) loadSources() ([]hdSource, error) {
	detail := x.loadCourseDetail()
	var sources []hdSource
	sources = append(sources, x.extractSourceInfo(detail)...)
	lessons := x.lessonList(detail)
	if len(lessons) == 0 {
		lessons = gatherRows(detail, CHILD_LIST_KEYS)
	}
	videoSources := x.buildLessonSources(lessons, firstNonEmpty(x.courseType, x.selectedCourse.CourseType))
	sources = append(videoSources, sources...)
	if len(sources) == 0 {
		return nil, fmt.Errorf("houdu: no playable lessons or files")
	}
	return sources, nil
}

func (x *hdCtx) lessonList(detail map[string]any) []map[string]any {
	if x.cid == "" {
		return nil
	}
	if firstNonEmpty(x.courseType, x.selectedCourse.CourseType) == COURSE_TYPE_RECORDED {
		resp, err := x.requestHoudu("/mini/mini/recordClasslessonList", map[string]any{"class_id": coerceAPIID(x.cid)}, "phoenix")
		if err == nil {
			data := asMap(x.extractData(resp))
			if groups := listAt(data, "group_list"); len(groups) > 0 {
				return groups
			}
			if rows := listAt(data, "list"); len(rows) > 0 {
				return rows
			}
			if rows := listAt(data, "lesson_list"); len(rows) > 0 {
				return rows
			}
		}
	}
	return x.getClassLessonList()
}

func (x *hdCtx) getClassLessonList() []map[string]any {
	seen := map[string]bool{}
	var out []map[string]any
	for page := 1; page <= 20; page++ {
		resp, err := x.requestHoudu("/mini/mini/classLessonList", map[string]any{"class_id": coerceAPIID(x.cid), "page": page, "page_size": 100}, "phoenix")
		if err != nil {
			break
		}
		data := asMap(x.extractData(resp))
		rows := listAt(data, "list")
		if len(rows) == 0 {
			rows = listAt(data, "lesson_list")
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			id := firstString(row, "id", "lesson_id")
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, row)
		}
		total := intVal(data["total"])
		if total == 0 || page*100 >= total {
			break
		}
	}
	return out
}

func (x *hdCtx) buildLessonSources(items []map[string]any, defaultType string) []hdSource {
	if len(items) == 0 {
		return nil
	}
	var out []hdSource
	items = sortGroupList(items)
	flat := isFlatLessonList(items)
	if flat {
		lessons := sortLessonList(items)
		for i, lesson := range lessons {
			out = append(out, x.makeLessonSources(lesson, []int{i + 1}, defaultType)...)
		}
		return out
	}
	for gi, group := range items {
		groupName := cleanName(firstNonEmpty(firstString(group, "group_name", "chapter_name", "chapter_title", "catalog_name", "catalog_title", "module_name", "module_title", "unit_name", "unit_title", "section_name", "section_title", "title", "name"), fmt.Sprintf("章节%d", gi+1)))
		lessons := gatherRows(group, CHILD_LIST_KEYS)
		if len(lessons) == 0 && firstString(group, "id", "lesson_id") != "" {
			lessons = []map[string]any{group}
		}
		for li, lesson := range sortLessonList(lessons) {
			prefix := []int{gi + 1, li + 1}
			for _, src := range x.makeLessonSources(lesson, prefix, defaultType) {
				if groupName != "" {
					src.Extra = ensureExtra(src.Extra)
					src.Extra["chapter"] = groupName
				}
				out = append(out, src)
			}
		}
	}
	return out
}

func (x *hdCtx) makeLessonSources(lesson map[string]any, prefix []int, defaultType string) []hdSource {
	lessonID := firstString(lesson, "id", "lesson_id")
	if lessonID == "" {
		return nil
	}
	lessonName := cleanName(firstNonEmpty(firstString(lesson, "title", "lesson_name", "name"), fmt.Sprintf("lesson_%s", lessonID)))
	name := fmt.Sprintf("[%s]--%s", joinInts(prefix, "."), lessonName)
	lessonType := firstNonEmpty(firstString(lesson, "lesson_type"), defaultType, firstString(lesson, "course_type"))
	modes := lessonModes(lesson, lessonType)
	for _, mode := range modes {
		if pb := x.getPlaybackForMode(lessonID, mode, lesson); pb.URL != "" {
			format := firstNonEmpty(pb.Format, extFormat(pb.URL))
			extra := mergeExtra(pb.Extra, map[string]any{"lesson_id": lessonID, "mode": mode, "lesson_type": lessonType})
			return []hdSource{{Name: name, URL: pb.URL, Kind: "video", Format: format, NeedMerge: pb.NeedMerge || strings.Contains(strings.ToLower(pb.URL), ".m3u8"), Extra: extra}}
		}
	}
	return nil
}

func lessonModes(lesson map[string]any, lessonType string) []string {
	var modes []string
	if lessonType == COURSE_TYPE_RECORDED || str(lesson["lesson_type"]) == COURSE_TYPE_RECORDED {
		modes = append(modes, "record")
	}
	if str(lesson["replay_status"]) == "2" || strings.Contains(firstString(lesson, "replay_status_name"), "回放") {
		modes = append(modes, "playback")
	}
	modes = append(modes, "live")
	return uniqueStrings(modes)
}
