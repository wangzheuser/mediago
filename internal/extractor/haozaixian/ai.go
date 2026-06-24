package haozaixian

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func (x *hzCtx) getAIInfos() ([]hzLesson, []hzMaterial, error) {
	root, err := x.requestJSON(fmt.Sprintf("%s?courseId=%s", ai_course_info_url, x.cid), nil)
	if err != nil {
		return nil, nil, err
	}
	if str(root["errNo"]) != "0" {
		return nil, nil, fmt.Errorf("haozaixian ai course info: errNo=%v", root["errNo"])
	}
	data := asMap(root["data"])
	groups := aiLessonGroups(data["lessonList"])
	var lessons []hzLesson
	seq := 0
	for _, group := range groups {
		for _, row := range group {
			lessonID := firstNonEmpty(str(row["lessonId"]), str(row["id"]))
			lessonName := cleanName(firstNonEmpty(str(row["lessonName"]), str(row["name"]), str(row["title"])))
			if lessonID == "" || lessonName == "" {
				continue
			}
			seq++
			lessons = append(lessons, hzLesson{
				Name:       fmt.Sprintf("[%d.1.1]--%s", seq, lessonName),
				LessonName: lessonName,
				LessonID:   lessonID,
				RoundID:    firstNonEmpty(str(row["roundId"]), str(asMap(row["mainCourseInfo"])["roundId"])),
				Seq:        seq,
				Type:       "ai_video",
			})
		}
	}
	if len(lessons) == 0 {
		return nil, nil, fmt.Errorf("haozaixian ai: empty lesson list")
	}
	return lessons, nil, nil
}

func aiLessonGroups(v any) [][]map[string]any {
	switch t := v.(type) {
	case []any:
		return [][]map[string]any{listMaps(t)}
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([][]map[string]any, 0, len(keys))
		for _, k := range keys {
			if rows := listMaps(t[k]); len(rows) > 0 {
				out = append(out, rows)
			}
		}
		return out
	default:
		return nil
	}
}

func (x *hzCtx) getAIRoundID(lessonID string) string {
	if lessonID == "" {
		return ""
	}
	root, err := x.requestJSON(fmt.Sprintf("%s?courseId=%s&lessonId=%s", ai_lesson_detail_url, x.cid, lessonID), nil)
	if err != nil || str(root["errNo"]) != "0" {
		return ""
	}
	data := asMap(root["data"])
	return firstNonEmpty(str(asMap(data["mainCourseInfo"])["roundId"]), str(data["roundId"]))
}

func (x *hzCtx) getAIVideoURLs(lessonID, roundID string) []aiVideo {
	if lessonID == "" || roundID == "" {
		return nil
	}
	endpoint := fmt.Sprintf("%s?lessonId=%s&courseId=%s&roundId=%s", ai_video_by_round_url, lessonID, x.cid, roundID)
	root, err := x.requestJSON(endpoint, nil)
	if err != nil || str(root["errNo"]) != "0" {
		return nil
	}
	data := asMap(root["data"])
	style := data["styleContent"]
	if str(style) == "" {
		return nil
	}
	var parsed any = style
	if s, ok := style.(string); ok {
		if err := json.Unmarshal([]byte(s), &parsed); err != nil {
			return nil
		}
	}
	var videos []aiVideo
	for _, node := range collectMaps(parsed) {
		if str(node["type"]) != "Video" {
			continue
		}
		props := asMap(node["props"])
		videoURL := firstNonEmpty(str(props["video"]), str(node["video"]))
		if !strings.HasPrefix(videoURL, "http") {
			continue
		}
		if x.qualityMode() == "sd" {
			videoURL = strings.ReplaceAll(videoURL, "_720.", "_480.")
			videoURL = strings.ReplaceAll(videoURL, "_720_", "_480_")
		} else {
			videoURL = strings.ReplaceAll(videoURL, "_480.", "_720.")
			videoURL = strings.ReplaceAll(videoURL, "_480_", "_720_")
		}
		videos = append(videos, aiVideo{URL: videoURL, Duration: intVal(firstExisting(props["duration"], node["duration"]))})
	}
	if len(videos) == 0 {
		return nil
	}
	sort.SliceStable(videos, func(i, j int) bool { return videos[i].Duration > videos[j].Duration })
	for i := range videos {
		videos[i].IsMain = i == 0
	}
	return videos
}
