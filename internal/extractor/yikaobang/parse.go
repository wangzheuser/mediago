package yikaobang

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
)

type ykbParseResult struct {
	Title    string
	Courses  []ykbCourse
	Chapters []extractor.Chapter
	Videos   []ykbItem
	Files    []ykbItem
	Aliyun   ykbAliyun
	Raw      []any
}

type ykbCourse struct {
	ID         string
	Title      string
	URL        string
	Cover      string
	ActivityID string
	AppID      string
	Raw        map[string]any
}

type ykbItem struct {
	Kind      string
	ID        string
	Title     string
	URL       string
	Format    string
	Size      int64
	CourseID  string
	ChapterID string
	VideoID   string
	Aliyun    ykbAliyun
	Raw       map[string]any
	Source    string
}

type ykbAliyun struct {
	VideoID         string
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	Region          string
	AuthInfo        string
	AuthTimeout     string
}

func (a ykbAliyun) complete() bool {
	return a.VideoID != "" && a.AccessKeyID != "" && a.AccessKeySecret != ""
}

func parseYikaobangPayloads(payloads []ykbPayload, target ykbTarget) ykbParseResult {
	result := ykbParseResult{}
	for _, payload := range payloads {
		if payload.Root != nil {
			result.Raw = append(result.Raw, payload.Root)
		}
		result.collectValue(payload.Root, payload.Source, "", "", target.CourseID, 0)
		if len(payload.Body) > 0 {
			result.collectTextURLs(payload.Body, payload.Source, target.CourseID, "")
		}
	}
	if result.Title == "" {
		result.Title = firstNonEmpty(target.CourseID, target.VideoID, "医考帮")
	}
	result.Courses = dedupeCourses(result.Courses)
	result.Chapters = dedupeChapters(result.Chapters)
	result.Videos = dedupeItems(result.Videos)
	result.Files = dedupeItems(result.Files)
	return result
}

func (r *ykbParseResult) collectValue(value any, source, chapterTitle, chapterID, courseID string, depth int) {
	if value == nil || depth > 12 {
		return
	}
	switch v := value.(type) {
	case map[string]any:
		r.collectMap(v, source, chapterTitle, chapterID, courseID, depth)
	case []any:
		for _, item := range v {
			r.collectValue(item, source, chapterTitle, chapterID, courseID, depth+1)
		}
	case string:
		r.collectTextURLs(v, source, courseID, chapterID)
	}
}

func (r *ykbParseResult) collectMap(m map[string]any, source, chapterTitle, chapterID, courseID string, depth int) {
	if len(m) == 0 {
		return
	}
	if title := firstTitle(m); r.Title == "" && title != "" && !looksLikeChapterNode(m) && !looksLikeVideoNode(m) && !looksLikeFileNode(m) {
		r.Title = title
	}
	currentCourseID := firstNonEmpty(courseID, textValue(m, "course_id", "courseId", "courseID", "c_id", "cid"))
	currentChapterID := chapterID
	currentChapterTitle := chapterTitle
	if looksLikeChapterNode(m) {
		chapter := chapterFromMap(m, source, len(r.Chapters)+1)
		if chapter.Title != "" {
			r.Chapters = append(r.Chapters, chapter)
			currentChapterTitle = chapter.Title
			currentChapterID = firstNonEmpty(chapter.URL, currentChapterID)
		}
	}
	if course := courseFromMap(m, source); course.ID != "" || course.Title != "" {
		r.Courses = append(r.Courses, course)
		if currentCourseID == "" {
			currentCourseID = course.ID
		}
	}
	if aliyun := aliyunFromMap(m); aliyun.AccessKeyID != "" || aliyun.AccessKeySecret != "" || aliyun.SecurityToken != "" {
		if aliyun.VideoID != "" {
			item := videoFromMap(m, source, currentChapterTitle, currentChapterID, currentCourseID)
			if item.VideoID == "" {
				item.VideoID = aliyun.VideoID
			}
			item.Aliyun = mergeAliyun(item.Aliyun, aliyun)
			if item.Title == "" {
				item.Title = cleanTitle(firstNonEmpty(currentChapterTitle, item.VideoID))
			}
			r.Videos = append(r.Videos, item)
		} else {
			r.Aliyun = mergeAliyun(r.Aliyun, aliyun)
		}
	}
	if looksLikeVideoNode(m) {
		item := videoFromMap(m, source, currentChapterTitle, currentChapterID, currentCourseID)
		if item.URL != "" || item.VideoID != "" || item.Aliyun.AccessKeyID != "" {
			r.Videos = append(r.Videos, item)
		}
	}
	if looksLikeFileNode(m) {
		item := fileFromMap(m, source, currentChapterTitle, currentChapterID, currentCourseID)
		if item.URL != "" {
			r.Files = append(r.Files, item)
		}
	}
	for key, child := range m {
		nextChapterTitle := currentChapterTitle
		nextChapterID := currentChapterID
		if isChapterChildrenKey(key) && firstTitle(m) != "" {
			nextChapterTitle = firstTitle(m)
			nextChapterID = firstNonEmpty(textValue(m, "chapter_id", "chapterId", "id"), currentChapterID)
		}
		r.collectValue(child, source, nextChapterTitle, nextChapterID, currentCourseID, depth+1)
	}
}

func (r *ykbParseResult) collectTextURLs(text, source, courseID, chapterID string) {
	for _, mediaURL := range extractURLsFromText(text) {
		format := ykbFormat(mediaURL, "")
		switch {
		case ykbLooksLikeMediaURL(mediaURL):
			r.Videos = append(r.Videos, ykbItem{Kind: "video", Title: cleanTitle(firstNonEmpty(chapterID, courseID, "医考帮视频")), URL: mediaURL, Format: format, CourseID: courseID, ChapterID: chapterID, Source: source})
		case ykbLooksLikeFileURL(mediaURL):
			r.Files = append(r.Files, ykbItem{Kind: "file", Title: cleanTitle(firstNonEmpty(chapterID, courseID, "医考帮资料")), URL: mediaURL, Format: format, CourseID: courseID, ChapterID: chapterID, Source: source})
		}
	}
}

func firstTitle(m map[string]any) string {
	return cleanTitle(firstNonEmpty(textValue(m, "course_title", "courseTitle", "course_name", "courseName", "title", "name", "label", "mTitle", "fileName", "file_name")))
}

func courseFromMap(m map[string]any, source string) ykbCourse {
	if !looksLikeCourseNode(m) {
		return ykbCourse{}
	}
	id := firstNonEmpty(textValue(m, "course_id", "courseId", "courseID", "id", "c_id", "cid"))
	title := firstTitle(m)
	return ykbCourse{
		ID:         id,
		Title:      cleanTitle(firstNonEmpty(title, id, "医考帮课程")),
		URL:        firstNonEmpty(normalizeYikaobangURL(textValue(m, "url", "link", "is_open_link", "share_url"), source, false), courseURL(id)),
		Cover:      normalizeYikaobangURL(textValue(m, "cover_img", "cover", "course_cover", "coverImg", "thumb"), source, false),
		ActivityID: textValue(m, "activity_id", "activityId", "xue_activity_id"),
		AppID:      textValue(m, "app_id", "appId", "appid"),
		Raw:        m,
	}
}

func chapterFromMap(m map[string]any, source string, index int) extractor.Chapter {
	id := firstNonEmpty(textValue(m, "chapter_id", "chapterId", "id", "pid", "parent_id"))
	title := firstTitle(m)
	return extractor.Chapter{Title: cleanTitle(firstNonEmpty(title, id, fmt.Sprintf("章节 %d", index))), URL: firstNonEmpty(id, source), Index: index}
}

func videoFromMap(m map[string]any, source, chapterTitle, chapterID, courseID string) ykbItem {
	mediaURL := directMediaURL(m, source)
	videoID := firstNonEmpty(textValue(m, "vid", "video_id", "videoId", "videoID", "free_watch_vid", "fileId", "file_id"))
	id := firstNonEmpty(textValue(m, "id", "obj_id", "video_id", "videoId", "vid"), videoID)
	title := firstNonEmpty(firstTitle(m), chapterTitle, id, videoID, "医考帮视频")
	format := ykbFormat(mediaURL, firstNonEmpty(textValue(m, "mFormat", "format", "suffix")))
	return ykbItem{
		Kind:      "video",
		ID:        id,
		Title:     cleanTitle(title),
		URL:       mediaURL,
		Format:    format,
		Size:      int64Value(m["size"], m["size_byte"], m["fileSize"], m["videoSize"]),
		CourseID:  firstNonEmpty(courseID, textValue(m, "course_id", "courseId", "courseID", "c_id", "cid")),
		ChapterID: firstNonEmpty(chapterID, textValue(m, "chapter_id", "chapterId")),
		VideoID:   videoID,
		Aliyun:    aliyunFromMap(m),
		Raw:       m,
		Source:    source,
	}
}

func fileFromMap(m map[string]any, source, chapterTitle, chapterID, courseID string) ykbItem {
	fileURL := directFileURL(m, source)
	id := firstNonEmpty(textValue(m, "id", "file_id", "fileId", "obj_id"), fileURL)
	title := firstNonEmpty(firstTitle(m), chapterTitle, id, "医考帮资料")
	format := ykbFormat(fileURL, firstNonEmpty(textValue(m, "suffix", "format", "type")))
	return ykbItem{
		Kind:      "file",
		ID:        id,
		Title:     cleanTitle(title),
		URL:       fileURL,
		Format:    format,
		Size:      int64Value(m["size_byte"], m["size"], m["fileSize"], m["file_size"]),
		CourseID:  firstNonEmpty(courseID, textValue(m, "course_id", "courseId", "courseID", "c_id", "cid")),
		ChapterID: firstNonEmpty(chapterID, textValue(m, "chapter_id", "chapterId")),
		Raw:       m,
		Source:    source,
	}
}

func aliyunFromMap(m map[string]any) ykbAliyun {
	return ykbAliyun{
		VideoID:         firstNonEmpty(textValue(m, "vid", "video_id", "videoId", "VideoId", "free_watch_vid")),
		AccessKeyID:     firstNonEmpty(textValue(m, "akId", "acId", "AccessKeyId", "AccessKeyID", "accessKeyId", "access_key_id", "ky")),
		AccessKeySecret: firstNonEmpty(textValue(m, "akSecret", "akSceret", "AccessKeySecret", "accessKeySecret", "access_key_secret", "access_secret", "sc")),
		SecurityToken:   firstNonEmpty(textValue(m, "st", "securityToken", "SecurityToken", "sts_token", "stsToken", "token", "tk")),
		Region:          firstNonEmpty(textValue(m, "region", "Region", "regionId", "domain_region"), "cn-shanghai"),
		AuthInfo:        textValue(m, "AuthInfo", "authInfo", "auth_info"),
		AuthTimeout:     firstNonEmpty(textValue(m, "AuthTimeout", "authTimeout", "auth_timeout"), "7200"),
	}
}

func mergeAliyun(base, overlay ykbAliyun) ykbAliyun {
	return ykbAliyun{
		VideoID:         firstNonEmpty(base.VideoID, overlay.VideoID),
		AccessKeyID:     firstNonEmpty(base.AccessKeyID, overlay.AccessKeyID),
		AccessKeySecret: firstNonEmpty(base.AccessKeySecret, overlay.AccessKeySecret),
		SecurityToken:   firstNonEmpty(base.SecurityToken, overlay.SecurityToken),
		Region:          firstNonEmpty(base.Region, overlay.Region, "cn-shanghai"),
		AuthInfo:        firstNonEmpty(base.AuthInfo, overlay.AuthInfo),
		AuthTimeout:     firstNonEmpty(base.AuthTimeout, overlay.AuthTimeout, "7200"),
	}
}

func directMediaURL(m map[string]any, source string) string {
	for _, key := range []string{"play_url", "playUrl", "PlayURL", "video_url", "videoUrl", "video", "m3u8", "m3u8_url", "m3u8Url", "hlsUrl", "hls_url", "mediaUrl", "media_url", "mp4Url", "mp4_url", "fileUrl", "file_url", "downloadUrl", "download_url", "url"} {
		if raw := textValue(m, key); raw != "" {
			mediaURL := normalizeYikaobangURL(raw, source, false)
			if ykbLooksLikeMediaURL(mediaURL) {
				return mediaURL
			}
		}
	}
	return ""
}

func directFileURL(m map[string]any, source string) string {
	for _, key := range []string{"download_url", "downloadUrl", "file_url", "fileUrl", "resourceUrl", "resource_url", "sourceUrl", "source_url", "url", "link", "path"} {
		if raw := textValue(m, key); raw != "" {
			fileURL := normalizeYikaobangURL(raw, source, true)
			if ykbLooksLikeFileURL(fileURL) || textValue(m, "suffix") != "" {
				return fileURL
			}
		}
	}
	return ""
}

func looksLikeCourseNode(m map[string]any) bool {
	if firstTitle(m) == "" || looksLikeVideoNode(m) || looksLikeFileNode(m) {
		return false
	}
	if hasAny(m, "course_title", "courseTitle", "course_name", "courseName") {
		return true
	}
	return hasAny(m, "cover_img", "cover", "activity_id", "activityId", "xue_activity_id", "series", "require_interface", "buy", "is_open_link", "lecturer", "description") && hasText(m, "id", "course_id", "courseId", "c_id", "cid")
}

func looksLikeChapterNode(m map[string]any) bool {
	if firstTitle(m) == "" || looksLikeVideoNode(m) || looksLikeFileNode(m) {
		return false
	}
	if hasAny(m, "chapterList") {
		return false
	}
	if hasAny(m, "children", "courseList", "content", "videoList", "videos") {
		return true
	}
	return hasAny(m, "chapter_id", "chapterId", "parent_id", "parentId", "nodeLevel")
}

func looksLikeVideoNode(m map[string]any) bool {
	if directMediaURL(m, "") != "" {
		return true
	}
	if hasText(m, "vid", "video_id", "videoId", "free_watch_vid") && firstTitle(m) != "" {
		return true
	}
	return hasText(m, "akId", "acId", "AccessKeyId", "accessKeyId") && hasText(m, "vid", "video_id", "videoId", "free_watch_vid")
}

func looksLikeFileNode(m map[string]any) bool {
	if directFileURL(m, "") != "" {
		return true
	}
	return hasText(m, "suffix") && firstTitle(m) != "" && hasText(m, "url", "path", "fileUrl", "downloadUrl")
}

func isChapterChildrenKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "courselist", "course_list", "content", "children", "videolist", "video_list", "videos", "items":
		return true
	default:
		return false
	}
}

func courseURL(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	return ykbH5Base + "course/detail?id=" + url.QueryEscape(id)
}

func dedupeCourses(items []ykbCourse) []ykbCourse {
	seen := map[string]bool{}
	out := make([]ykbCourse, 0, len(items))
	for _, item := range items {
		key := firstNonEmpty(item.ID, item.URL, item.Title)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func dedupeChapters(items []extractor.Chapter) []extractor.Chapter {
	seen := map[string]bool{}
	out := make([]extractor.Chapter, 0, len(items))
	for _, item := range items {
		key := firstNonEmpty(item.URL, item.Title)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		item.Index = len(out) + 1
		out = append(out, item)
	}
	return out
}

func dedupeItems(items []ykbItem) []ykbItem {
	seen := map[string]bool{}
	out := make([]ykbItem, 0, len(items))
	for _, item := range items {
		primary := firstNonEmpty(item.URL, item.VideoID, item.ID, item.Title)
		key := strings.Join([]string{item.Kind, primary}, "|")
		if primary == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}
