package yikaobang

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/extractor/shared"
)

func (s *ykbSession) buildMedia(raw string, target ykbTarget, result ykbParseResult) (*extractor.MediaInfo, []error) {
	var errs []error
	entries := make([]*extractor.MediaInfo, 0, len(result.Videos)+len(result.Files)+len(result.Courses))
	for _, item := range result.Videos {
		item = s.fillItemDefaults(item, target, result)
		entry, err := s.buildVideoEntry(item)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		entries = append(entries, entry)
	}
	for _, item := range result.Files {
		item = s.fillItemDefaults(item, target, result)
		entry, err := s.buildFileEntry(item)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		entries = append(entries, entry)
	}
	entries = dedupeMediaEntries(entries)

	if len(entries) == 1 && len(result.Courses) == 0 && len(result.Chapters) == 0 && (target.VideoID != "" || target.ChapterID == "") {
		return entries[0], errs
	}
	if len(entries) == 0 && len(result.Courses) > 0 {
		for _, course := range result.Courses {
			entries = append(entries, courseEntry(course))
		}
	}
	if len(entries) == 0 {
		return nil, errs
	}
	return &extractor.MediaInfo{
		Site:     ykbSite,
		Title:    cleanTitle(firstNonEmpty(result.Title, target.CourseID, target.VideoID, "医考帮")),
		Entries:  entries,
		Chapters: result.Chapters,
		Extra: map[string]any{
			"course_id":    target.CourseID,
			"chapter_id":   target.ChapterID,
			"video_id":     target.VideoID,
			"source_url":   strings.TrimSpace(raw),
			"course_count": len(result.Courses),
		},
	}, errs
}

func (s *ykbSession) fillItemDefaults(item ykbItem, target ykbTarget, result ykbParseResult) ykbItem {
	item.CourseID = firstNonEmpty(item.CourseID, target.CourseID)
	item.ChapterID = firstNonEmpty(item.ChapterID, target.ChapterID)
	item.VideoID = firstNonEmpty(item.VideoID, target.VideoID, item.ID)
	item.Aliyun = mergeAliyun(item.Aliyun, result.Aliyun)
	if item.Aliyun.VideoID == "" {
		item.Aliyun.VideoID = item.VideoID
	}
	if item.Title == "" {
		item.Title = cleanTitle(firstNonEmpty(item.ID, item.VideoID, item.URL, "医考帮"))
	}
	if item.Format == "" {
		item.Format = ykbFormat(item.URL, "")
	}
	return item
}

func (s *ykbSession) buildVideoEntry(item ykbItem) (*extractor.MediaInfo, error) {
	if item.URL == "" && item.VideoID != "" && !item.Aliyun.complete() && !s.listOnly {
		if enriched, err := s.enrichVideoItem(item); err == nil {
			item = enriched
		}
	}
	mediaURL := item.URL
	format := item.Format
	extra := map[string]any{
		"type":       "video",
		"id":         item.ID,
		"course_id":  item.CourseID,
		"chapter_id": item.ChapterID,
		"video_id":   item.VideoID,
		"source":     item.Source,
	}
	if mediaURL == "" && item.Aliyun.complete() && !s.listOnly {
		info, err := s.resolveAliyun(item.Aliyun)
		if err != nil {
			return nil, fmt.Errorf("yikaobang video %s aliyun resolve: %w", firstNonEmpty(item.VideoID, item.ID), err)
		}
		mediaURL = info.URL
		format = firstNonEmpty(info.Format, ykbFormat(mediaURL, item.Format))
		if info.M3U8Text != "" {
			mediaURL = "data:application/vnd.apple.mpegurl;base64," + base64.StdEncoding.EncodeToString([]byte(info.M3U8Text))
			format = "m3u8"
			extra["m3u8_text"] = info.M3U8Text
			extra["source_type"] = "m3u8_text"
		}
		extra["aliyun_api"] = info.APIURL
		extra["aliyun_definition"] = info.Definition
		extra["aliyun_encrypt_type"] = info.EncryptType
	}
	if mediaURL == "" {
		if s.listOnly {
			return &extractor.MediaInfo{Site: ykbSite, Title: cleanTitle(item.Title), Extra: extra}, nil
		}
		return nil, fmt.Errorf("yikaobang video %s has no direct URL or complete Aliyun STS", firstNonEmpty(item.VideoID, item.ID, item.Title))
	}
	format = firstNonEmpty(format, ykbFormat(mediaURL, ""))
	stream := extractor.Stream{Quality: firstNonEmpty(s.quality, "best"), URLs: []string{mediaURL}, Format: format, Size: item.Size, Headers: streamHeaders(s.headers), NeedMerge: format == "m3u8"}
	return &extractor.MediaInfo{Site: ykbSite, Title: cleanTitle(item.Title), Streams: map[string]extractor.Stream{"best": stream}, Extra: extra}, nil
}

func (s *ykbSession) buildFileEntry(item ykbItem) (*extractor.MediaInfo, error) {
	if item.URL == "" {
		return nil, fmt.Errorf("yikaobang file %s has no URL", firstNonEmpty(item.ID, item.Title))
	}
	format := firstNonEmpty(item.Format, ykbFormat(item.URL, "file"))
	stream := extractor.Stream{Quality: "file", URLs: []string{item.URL}, Format: format, Size: item.Size, Headers: streamHeaders(s.headers)}
	return &extractor.MediaInfo{
		Site:    ykbSite,
		Title:   cleanTitle(item.Title),
		Streams: map[string]extractor.Stream{"file": stream},
		Extra: map[string]any{
			"type":       "file",
			"id":         item.ID,
			"course_id":  item.CourseID,
			"chapter_id": item.ChapterID,
			"file_url":   item.URL,
			"source":     item.Source,
		},
	}, nil
}

func (s *ykbSession) enrichVideoItem(item ykbItem) (ykbItem, error) {
	if item.VideoID == "" {
		return item, nil
	}
	target := ykbTarget{CourseID: item.CourseID, ChapterID: item.ChapterID, VideoID: item.VideoID}
	params := yikaobangVideoParams(target)
	requests := []ykbAPIRequest{}
	for _, endpoint := range []string{ykbEndpointCourseAK, ykbEndpointLegacyVideo} {
		requests = appendAPIRequests(requests, ykbLegacyAPI(endpoint), params, endpoint)
	}
	var payloads []ykbPayload
	var errs []error
	seen := map[string]bool{}
	for _, req := range requests {
		key := req.Method + " " + yikaobangURLWithParams(req.URL, req.Params)
		if seen[key] {
			continue
		}
		seen[key] = true
		payload, err := s.fetchPayload(req)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		payloads = append(payloads, payload)
	}
	if len(payloads) == 0 {
		if len(errs) == 0 {
			return item, nil
		}
		return item, fmt.Errorf("%s", joinErrors(errs))
	}
	result := parseYikaobangPayloads(payloads, target)
	item.Aliyun = mergeAliyun(item.Aliyun, result.Aliyun)
	for _, candidate := range result.Videos {
		if candidate.URL == "" && !candidate.Aliyun.complete() {
			continue
		}
		if candidate.VideoID != "" && item.VideoID != "" && candidate.VideoID != item.VideoID {
			continue
		}
		if item.URL == "" {
			item.URL = candidate.URL
		}
		item.Format = firstNonEmpty(item.Format, candidate.Format)
		item.Size = firstNonEmptyInt64(item.Size, candidate.Size)
		item.Title = cleanTitle(firstNonEmpty(item.Title, candidate.Title))
		item.ID = firstNonEmpty(item.ID, candidate.ID)
		item.Aliyun = mergeAliyun(item.Aliyun, candidate.Aliyun)
		break
	}
	if item.Aliyun.VideoID == "" {
		item.Aliyun.VideoID = item.VideoID
	}
	return item, nil
}

func firstNonEmptyInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func (s *ykbSession) resolveAliyun(auth ykbAliyun) (*shared.AliyunPlayInfo, error) {
	payload := shared.AliyunPlayPayload{
		AccessKeyID:     auth.AccessKeyID,
		AccessKeySecret: auth.AccessKeySecret,
		SecurityToken:   auth.SecurityToken,
		Region:          firstNonEmpty(auth.Region, "cn-shanghai"),
		AuthInfo:        auth.AuthInfo,
		AuthTimeout:     firstNonEmpty(auth.AuthTimeout, "7200"),
		Raw:             auth,
	}
	return shared.AliyunResolvePlayInfo(s.client, payload, auth.VideoID, shared.AliyunPlayOptions{
		Referer:         ykbRefererURL,
		Origin:          strings.TrimRight(ykbH5Base, "/"),
		Quality:         s.quality,
		Formats:         "m3u8,mp4",
		AuthTimeout:     firstNonEmpty(auth.AuthTimeout, "7200"),
		Headers:         s.headers,
		FetchM3U8:       true,
		RewriteM3U8Keys: true,
	})
}

func courseEntry(course ykbCourse) *extractor.MediaInfo {
	return &extractor.MediaInfo{
		Site:  ykbSite,
		Title: cleanTitle(firstNonEmpty(course.Title, course.ID, "医考帮课程")),
		Extra: map[string]any{
			"type":        "course",
			"course_id":   course.ID,
			"course_url":  course.URL,
			"cover":       course.Cover,
			"activity_id": course.ActivityID,
			"app_id":      course.AppID,
			"raw":         course.Raw,
		},
	}
}

func dedupeMediaEntries(entries []*extractor.MediaInfo) []*extractor.MediaInfo {
	seen := map[string]bool{}
	out := make([]*extractor.MediaInfo, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		key := entry.Title
		for _, stream := range entry.Streams {
			if len(stream.URLs) > 0 {
				key = stream.URLs[0]
				break
			}
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entry)
	}
	return out
}
