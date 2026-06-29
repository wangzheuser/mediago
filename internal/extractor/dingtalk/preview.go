package dingtalk

import (
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

// DingTalk document/video preview URLs come from alidocs "CSpace" pages.  The
// Python source parses several historic query aliases, then tries three LWP RPCs
// in order: CSpace/preview, CSpace/getVideoTranscodingInfo, CSpace/downloadInfoV2.
type previewDentryMeta struct {
	SpaceID    string
	FileID     string
	DentryUUID string
	CorpID     string
	OrgID      string
	FileName   string
	FileType   string
	Version    string
	Scene      string
	Operate    string
	FileSize   string
	RawURL     string
}

type previewFileMeta struct {
	FileName         string
	Uploader         string
	SpaceName        string
	FileSize         string
	FileSizeText     string
	CreateTime       string
	CreateTimeText   string
	ModifiedTime     string
	ModifiedTimeText string
	Raw              map[string]any
}

func extractPreviewDentryMeta(rawURL string) previewDentryMeta {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return previewDentryMeta{}
	}
	q := queryValuesCI(u.Query())
	pick := func(names ...string) string {
		for _, name := range names {
			if v := q[strings.ToLower(name)]; v != "" {
				return v
			}
		}
		return ""
	}
	meta := previewDentryMeta{
		SpaceID:    pick("spaceId", "spaceID", "spaceid", "spaceld", "spaceLd", "spaceLD", "driveSpaceId", "bizId", "cloudSpaceSpaceId"),
		FileID:     pick("fileId", "fileID", "fileid", "fileld", "fileLd", "fileLD", "dentryId", "dentryID", "driveDentryId", "objectId", "cloudSpaceDentryId"),
		DentryUUID: pick("dentryUuid", "dentryUUID", "uuid"),
		CorpID:     pick("corpId"),
		OrgID:      pick("orgId"),
		FileName:   pick("fileName", "name", "title"),
		FileType:   pick("fileType", "bizType", "type", "extension"),
		Version:    pick("version"),
		Scene:      pick("scene"),
		Operate:    pick("operate"),
		RawURL:     rawURL,
	}
	if meta.CorpID == "" {
		meta.CorpID = meta.OrgID
	}
	return meta
}

func queryValuesCI(values url.Values) map[string]string {
	out := make(map[string]string, len(values))
	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		if v := strings.TrimSpace(vals[0]); v != "" {
			out[strings.ToLower(key)] = v
		}
	}
	return out
}

func hydratePreviewDentryMeta(opts *extractor.ExtractOpts, meta previewDentryMeta) previewDentryMeta {
	if meta.RawURL == "" || (meta.SpaceID != "" && meta.FileID != "") || !isPreviewDentryCandidate(meta.RawURL) {
		return meta
	}
	c := util.NewClient()
	if opts != nil && opts.Cookies != nil {
		c.SetCookieJar(opts.Cookies)
	}
	resp, err := c.Get(meta.RawURL, map[string]string{
		"Referer":    "https://alidocs.dingtalk.com/",
		"User-Agent": pcUA,
		"Cookie":     cookieString(opts),
	})
	if err != nil {
		return meta
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return meta
	}
	mergePreviewDentryMeta(&meta, extractPreviewDentryMeta(resp.Request.URL.String()))
	mergePreviewDentryMeta(&meta, extractPreviewMetaFromText(string(body)))
	return meta
}

func isPreviewDentryCandidate(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "previewdentry") ||
		strings.Contains(lower, "uni-preview") ||
		strings.Contains(lower, "biztype=video") ||
		strings.Contains(lower, "cloudspacedentryid=") ||
		strings.Contains(lower, "fileid=") ||
		strings.Contains(lower, "fileld=")
}

func mergePreviewDentryMeta(base *previewDentryMeta, extra previewDentryMeta) {
	if base == nil {
		return
	}
	if base.SpaceID == "" {
		base.SpaceID = extra.SpaceID
	}
	if base.FileID == "" {
		base.FileID = extra.FileID
	}
	if base.DentryUUID == "" {
		base.DentryUUID = extra.DentryUUID
	}
	if base.CorpID == "" {
		base.CorpID = firstNonEmptyText(extra.CorpID, extra.OrgID)
	}
	if base.OrgID == "" {
		base.OrgID = firstNonEmptyText(extra.OrgID, extra.CorpID)
	}
	if base.FileName == "" {
		base.FileName = extra.FileName
	}
	if base.FileType == "" {
		base.FileType = extra.FileType
	}
	if base.Version == "" {
		base.Version = extra.Version
	}
	if base.Scene == "" {
		base.Scene = extra.Scene
	}
	if base.Operate == "" {
		base.Operate = extra.Operate
	}
	if base.FileSize == "" {
		base.FileSize = extra.FileSize
	}
	if base.RawURL == "" {
		base.RawURL = extra.RawURL
	}
	corpID := firstNonEmptyText(base.CorpID, base.OrgID)
	if corpID != "" {
		base.CorpID = corpID
		base.OrgID = corpID
	}
}

func extractPreviewMetaFromText(text string) previewDentryMeta {
	return previewDentryMeta{
		SpaceID:    firstNonEmptyText(findFieldInText(text, "spaceId", "spaceld", "spaceLd"), findFieldInText(text, "cloudSpaceSpaceId")),
		FileID:     firstNonEmptyText(findFieldInText(text, "fileId", "fileld", "fileLd", "dentryId", "driveDentryId"), findFieldInText(text, "cloudSpaceDentryId")),
		DentryUUID: findFieldInText(text, "dentryUuid", "dentryUUID"),
		CorpID:     findFieldInText(text, "corpId", "orgId"),
		OrgID:      findFieldInText(text, "orgId", "corpId"),
		FileName:   findFieldInText(text, "fileName", "title", "name"),
		FileType:   findFieldInText(text, "fileType", "bizType", "type", "extension"),
		Version:    findFieldInText(text, "version"),
		Scene:      findFieldInText(text, "scene"),
		Operate:    findFieldInText(text, "operate"),
		FileSize:   findFieldInText(text, "fileSize"),
	}
}

func previewDentry(opts *extractor.ExtractOpts, meta previewDentryMeta) (*extractor.MediaInfo, error) {
	cookie := cookieString(opts)
	client, err := newLwpClient(cookie)
	if err != nil {
		return nil, err
	}
	if err := client.connect(); err != nil {
		return nil, err
	}
	defer client.close()

	_ = resolvePreviewDentryUUID(client, &meta)
	fileMeta := resolvePreviewFileMeta(client, meta)
	media, err := resolvePreviewMedia(client, meta)
	if err != nil {
		return nil, err
	}
	media.Title = firstNonEmptyText(fileMeta.FileName, meta.FileName, media.Title, "dingtalk_preview_"+coalesce(meta.FileID, meta.DentryUUID, meta.SpaceID))
	info, err := buildMediaInfo(media)
	if err != nil {
		return nil, err
	}
	if info.Extra == nil {
		info.Extra = map[string]any{}
	}
	info.Extra["preview_dentry"] = map[string]any{
		"space_id":           meta.SpaceID,
		"file_id":            meta.FileID,
		"dentry_uuid":        meta.DentryUUID,
		"url":                meta.RawURL,
		"uploader":           fileMeta.Uploader,
		"space_name":         fileMeta.SpaceName,
		"file_size":          firstNonEmptyText(fileMeta.FileSize, metaValue(fileMeta.Raw, "size", "fileSize", "contentSize")),
		"file_size_text":     fileMeta.FileSizeText,
		"create_time":        fileMeta.CreateTime,
		"create_time_text":   fileMeta.CreateTimeText,
		"modified_time":      fileMeta.ModifiedTime,
		"modified_time_text": fileMeta.ModifiedTimeText,
	}
	return info, nil
}

func resolvePreviewDentryUUID(client *lwpClient, meta *previewDentryMeta) string {
	if meta == nil || meta.DentryUUID != "" {
		if meta == nil {
			return ""
		}
		return meta.DentryUUID
	}
	for _, body := range buildCSpaceGetTokenBodies(*meta) {
		resp, err := client.call("/r/Adaptor/CSpace/getToken", body, 20*time.Second)
		if err != nil {
			continue
		}
		bodyMap := extractBody(resp)
		uuid := firstNonEmptyText(findFieldInText(fmt.Sprint(bodyMap), "dentryUuid"), findFirstString(bodyMap, "dentryUuid", "dentryUUID", "uuid"))
		if uuid != "" {
			meta.DentryUUID = uuid
			return uuid
		}
	}
	return ""
}

func resolvePreviewFileMeta(client *lwpClient, meta previewDentryMeta) previewFileMeta {
	for _, body := range buildCSpaceInfoDentryBodies(meta) {
		resp, err := client.call("/r/Adaptor/CSpace/infoDentryV2", body, 20*time.Second)
		if err != nil {
			continue
		}
		if fm := buildPreviewFileMeta(extractBody(resp)); fm.FileName != "" || len(fm.Raw) > 0 {
			return fm
		}
	}
	return previewFileMeta{}
}

func resolvePreviewMedia(client *lwpClient, meta previewDentryMeta) (*liveReplayResult, error) {
	steps := []struct {
		uri    string
		bodies [][]any
		source string
	}{
		{"/r/Adaptor/CSpace/preview", buildCSpacePreviewBodies(meta), "preview"},
		{"/r/Adaptor/CSpace/getVideoTranscodingInfo", buildCSpaceVideoTranscodingBodies(meta), "videoTranscoding"},
		{"/r/Adaptor/CSpace/downloadInfoV2", buildCSpaceDownloadInfoBodies(meta), "downloadInfo"},
	}
	var errs []string
	for _, step := range steps {
		for _, body := range step.bodies {
			resp, err := client.call(step.uri, body, 20*time.Second)
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			bodyMap := extractBody(resp)
			urls := findMediaURLs(bodyMap)
			if len(urls) == 0 {
				urls = findMediaURLs(resp)
			}
			if len(urls) == 0 {
				errs = append(errs, "no playable media url in "+step.source+" response")
				continue
			}
			return &liveReplayResult{
				LiveUUID:     firstNonEmptyText(meta.DentryUUID, meta.FileID),
				Title:        firstNonEmptyText(meta.FileName, path.Base(meta.FileID)),
				PlaybackURLs: uniqueStrings(urls),
			}, nil
		}
	}
	return nil, fmt.Errorf("unable to resolve preview media url: %s", strings.Join(errs, " | "))
}

func buildCSpaceGetTokenBodies(meta previewDentryMeta) [][]any {
	corpID := firstNonEmptyText(meta.CorpID, meta.OrgID)
	spaceID := meta.SpaceID
	fileID := meta.FileID
	if corpID == "" || spaceID == "" || fileID == "" {
		return nil
	}
	return uniqueLWPBodies([]map[string]any{
		{"corpId": corpID, "spaceId": spaceID, "dentryId": fileID},
		{"orgId": corpID, "spaceId": spaceID, "dentryId": fileID},
		{"corpId": corpID, "bizId": spaceID, "objectId": fileID},
	})
}

func buildCSpacePreviewBodies(meta previewDentryMeta) [][]any {
	if meta.SpaceID == "" || meta.FileID == "" {
		return nil
	}
	from := buildPreviewFromSource(meta)
	fileType := guessPreviewFileType(meta.FileName, meta.FileType)
	return uniqueLWPBodies([]map[string]any{
		{"dentryUuid": meta.DentryUUID, "bizType": fileType, "objectId": meta.FileID, "bizId": meta.SpaceID},
		{"corpId": firstNonEmptyText(meta.CorpID, meta.OrgID), "dentryUuid": meta.DentryUUID, "bizType": fileType, "objectId": meta.FileID, "bizId": meta.SpaceID},
		{"corpId": firstNonEmptyText(meta.CorpID, meta.OrgID), "dentryUuid": meta.DentryUUID, "bizType": fileType, "fileId": meta.FileID, "spaceId": meta.SpaceID},
		{"bizId": meta.SpaceID, "objectId": meta.FileID, "bizType": fileType},
		{"spaceId": meta.SpaceID, "fileId": meta.FileID, "bizType": fileType},
		{"fromSource": from, "dentryUuid": meta.DentryUUID, "bizType": fileType, "objectId": meta.FileID, "bizId": meta.SpaceID},
		{"fromSource": from, "dentryUuid": meta.DentryUUID, "bizType": "video", "objectId": meta.FileID, "bizId": meta.SpaceID},
	})
}

func buildCSpaceVideoTranscodingBodies(meta previewDentryMeta) [][]any {
	if meta.SpaceID == "" || meta.FileID == "" {
		return nil
	}
	from := buildPreviewFromSource(meta)
	payloads := []map[string]any{}
	for _, status := range []int{0, 1, 2} {
		payloads = append(payloads, map[string]any{"fromSource": from, "videoPlayStatus": status, "dentryId": meta.FileID, "spaceId": meta.SpaceID})
	}
	payloads = append(payloads,
		map[string]any{"fromSource": from, "dentryId": meta.FileID, "spaceId": meta.SpaceID},
		map[string]any{"videoPlayStatus": 0, "dentryId": meta.FileID, "spaceId": meta.SpaceID},
	)
	return uniqueLWPBodies(payloads)
}

func buildCSpaceDownloadInfoBodies(meta previewDentryMeta) [][]any {
	if meta.SpaceID == "" || meta.FileID == "" {
		return nil
	}
	from := buildPreviewFromSource(meta)
	options := map[string]any{"supportBrowser": true, "supportCDN": true}
	return uniqueLWPBodies([]map[string]any{
		{"optionsParam": options, "fromSource": from, "version": meta.Version, "fileId": meta.FileID, "spaceId": meta.SpaceID},
		{"optionsParam": options, "fromSource": from, "fileId": meta.FileID, "spaceId": meta.SpaceID},
	})
}

func buildCSpaceInfoDentryBodies(meta previewDentryMeta) [][]any {
	if meta.SpaceID == "" || meta.FileID == "" {
		return nil
	}
	from := buildPreviewFromSource(meta)
	extra := map[string]any{"needNick": true}
	return uniqueLWPBodies([]map[string]any{
		{"needMenuActions": true, "needExtra": extra, "fromSource": from, "version": meta.Version, "id": meta.FileID, "spaceId": meta.SpaceID},
		{"needMenuActions": true, "needExtra": extra, "fromSource": from, "id": meta.FileID, "spaceId": meta.SpaceID},
	})
}

func buildPreviewFromSource(meta previewDentryMeta) map[string]string {
	scene := meta.Scene
	if scene == "" {
		scene = "driveSpace"
		if strings.Contains(strings.ToLower(meta.RawURL), "alidocs.dingtalk.com/uni-preview") {
			scene = "universalSpace"
		}
	}
	return map[string]string{"scene": scene, "operate": firstNonEmptyText(meta.Operate, "preview")}
}

func uniqueLWPBodies(payloads []map[string]any) [][]any {
	type pair struct {
		key  string
		body []any
	}
	items := make([]pair, 0, len(payloads))
	for _, payload := range payloads {
		if len(payload) == 0 {
			continue
		}
		keys := make([]string, 0, len(payload))
		for k := range payload {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for _, k := range keys {
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(fmt.Sprint(payload[k]))
			b.WriteString(";")
		}
		items = append(items, pair{key: b.String(), body: []any{payload}})
	}
	seen := map[string]bool{}
	out := make([][]any, 0, len(items))
	for _, item := range items {
		if seen[item.key] {
			continue
		}
		seen[item.key] = true
		out = append(out, item.body)
	}
	return out
}

func guessPreviewFileType(fileName, fileType string) string {
	t := strings.ToLower(strings.TrimSpace(fileType))
	if strings.Contains(t, "video") {
		return "video"
	}
	if t == "file" || t == "preview" || t == "download" {
		t = ""
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(fileName)), ".")
	videoExt := map[string]bool{"mp4": true, "mov": true, "m4v": true, "flv": true, "avi": true, "wmv": true, "mkv": true, "webm": true, "ts": true, "m3u8": true}
	if videoExt[t] || videoExt[ext] {
		return "video"
	}
	return "video"
}

func buildPreviewFileMeta(payload map[string]any) previewFileMeta {
	item := extractFirstDentryModelItem(payload)
	if item == nil {
		return previewFileMeta{}
	}
	modifier := getMapField(item, "modifier")
	creator := getMapField(item, "creator")
	fileName := firstNonEmptyText(metaValue(item, "name", "fileName", "title"), basenameFromPreviewPath(metaValue(item, "path")))
	uploader := firstNonEmptyText(metaValue(modifier, "nick", "uid"), metaValue(creator, "nick", "uid"), metaValue(item, "modifierNick", "creatorNick"))
	size := firstNonEmptyText(metaValue(item, "size", "fileSize", "contentSize"))
	createTime := normalizePreviewTimestamp(metaValue(item, "createTime", "createdTime"))
	modifiedTime := normalizePreviewTimestamp(metaValue(item, "modifiedTime"))
	return previewFileMeta{
		FileName:         fileName,
		Uploader:         uploader,
		SpaceName:        metaValue(item, "spaceName"),
		FileSize:         size,
		FileSizeText:     formatPreviewFileSize(size),
		CreateTime:       createTime,
		CreateTimeText:   formatPreviewTimestamp(createTime),
		ModifiedTime:     modifiedTime,
		ModifiedTimeText: formatPreviewTimestamp(modifiedTime),
		Raw:              item,
	}
}

func extractFirstDentryModelItem(payload map[string]any) map[string]any {
	candidates := []any{}
	if data := getMapField(payload, "data"); data != nil {
		candidates = append(candidates, data["dentryModel"], data["dentry"], data["fileInfo"])
	}
	candidates = append(candidates, payload["dentryModel"], payload["dentry"], payload["fileInfo"])
	for _, candidate := range candidates {
		m, ok := candidate.(map[string]any)
		if !ok || len(m) == 0 {
			continue
		}
		if items, ok := m["items"].([]any); ok && len(items) > 0 {
			if item, ok := items[0].(map[string]any); ok {
				return item
			}
		}
		return m
	}
	return nil
}

func basenameFromPreviewPath(value string) string {
	if value == "" {
		return ""
	}
	return path.Base(strings.TrimSpace(value))
}

func normalizePreviewTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !regexp.MustCompile(`^\d+$`).MatchString(value) {
		return ""
	}
	return value
}

func formatPreviewTimestamp(value string) string {
	if value == "" {
		return ""
	}
	var ts int64
	if _, err := fmt.Sscanf(value, "%d", &ts); err != nil || ts <= 0 {
		return ""
	}
	if ts > 0x174876E800 {
		ts /= 1000
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}

func formatPreviewFileSize(value string) string {
	var size float64
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%f", &size); err != nil || size <= 0 {
		return ""
	}
	return fmt.Sprintf("%.2fMB", size/1048576)
}

func metaValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := getStringField(m, key); s != "" {
			return s
		}
		if v, ok := m[key]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func findFieldInText(text string, fieldNames ...string) string {
	if text == "" {
		return ""
	}
	for _, field := range fieldNames {
		quoted := regexp.QuoteMeta(field)
		patterns := []string{
			`(?i)"` + quoted + `"\s*:\s*"([^"]+)"`,
			`(?i)\\"` + quoted + `\\"\s*:\s*\\"([^"\\]+)\\"`,
			`(?i)'` + quoted + `'\s*:\s*'([^']+)'`,
			`(?i)(?:^|[?&#\s])` + quoted + `=([^&#"'>\s]+)`,
			`(?i)"` + quoted + `"\s*:\s*([0-9A-Za-z._-]+)`,
			`(?i)` + quoted + `\s*:\s*([0-9A-Za-z._-]+)`,
		}
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if m := re.FindStringSubmatch(text); len(m) == 2 {
				if v, err := url.QueryUnescape(strings.TrimSpace(m[1])); err == nil && v != "" {
					return v
				}
				return strings.TrimSpace(m[1])
			}
		}
	}
	return ""
}
