package dingtalk

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

type notableRecordMeta struct {
	Type           string
	ViewID         string
	SheetID        string
	RowID          string
	DentryUUID     string
	Version        string
	DocVersion     string
	CorpID         string
	DocAccessToken string
	DentryKey      string
	HostDocKey     string
	DocKey         string
	Permission     string
	RawURL         string
	Extra          map[string]any
}

type notableFileMeta struct {
	CorpID     string
	DentryUUID string
	FileID     string
	SpaceID    string
	FileType   string
	FileName   string
	Raw        map[string]any
}

func (m notableRecordMeta) Valid() bool {
	return m.DentryUUID != "" && m.RowID != "" && m.SheetID != "" && m.ViewID != ""
}

func extractNotableRecordMeta(rawURL string) notableRecordMeta {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.Contains(strings.ToLower(u.Host), "alidocs.dingtalk.com") || !strings.Contains(strings.ToLower(u.Path), "/notable/record") {
		return notableRecordMeta{}
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
	return notableRecordMeta{
		Type:       pick("type"),
		ViewID:     pick("viewId"),
		SheetID:    pick("sheetId"),
		RowID:      pick("rowId"),
		DentryUUID: pick("dentryUuid", "dentryUUID", "uuid"),
		Version:    pick("version"),
		DocVersion: pick("docVersion", "doc_version"),
		CorpID:     pick("corpId", "orgId"),
		DentryKey:  pick("dentryKey"),
		HostDocKey: pick("hostDocKey"),
		DocKey:     pick("docKey"),
		RawURL:     rawURL,
		Extra:      map[string]any{},
	}
}

func extractNotableRecord(opts *extractor.ExtractOpts, meta notableRecordMeta) (*extractor.MediaInfo, error) {
	cookie := cookieString(opts)
	if token := fetchDocAccessToken(cookie); token != "" {
		meta.DocAccessToken = token
	}
	payloads := fetchNotableDocumentPayloads(opts, &meta)
	wrapper := map[string]any{"payloads": payloads, "record_meta": metaMap(meta, false)}
	mediaURLs := uniqueStrings(findMediaURLs(wrapper))
	fileMetas := extractNotableFileMetas(wrapper)
	var previewExtra map[string]any

	if len(mediaURLs) == 0 && len(fileMetas) > 0 {
		if resolved, err := resolveNotableFileMedia(cookie, fileMetas, meta.RawURL); err == nil {
			mediaURLs = uniqueStrings(resolved.PlaybackURLs)
			previewExtra = map[string]any{
				"file_meta":    resolved.Extra["file_meta"],
				"preview_meta": resolved.Extra["preview_meta"],
			}
		}
	}
	if len(mediaURLs) == 0 {
		if hasNonePermission(payloads, meta) {
			return nil, fmt.Errorf("dingtalk notable record has no readable permission for current login")
		}
		return nil, fmt.Errorf("unable to resolve notable record media url")
	}

	result := &liveReplayResult{
		LiveUUID:     meta.DentryUUID,
		Title:        firstNonEmptyText(extractNotableTitle(wrapper, meta), "notable_record"),
		PlaybackURLs: mediaURLs,
	}
	info, err := buildMediaInfo(result)
	if err != nil {
		return nil, err
	}
	if info.Extra == nil {
		info.Extra = map[string]any{}
	}
	info.Extra["notable_record"] = map[string]any{
		"row_id":        meta.RowID,
		"sheet_id":      meta.SheetID,
		"view_id":       meta.ViewID,
		"dentry_uuid":   meta.DentryUUID,
		"url":           meta.RawURL,
		"file_metas":    len(fileMetas),
		"payload_count": len(payloads),
	}
	for k, v := range previewExtra {
		info.Extra[k] = v
	}
	return info, nil
}

func fetchDocAccessToken(cookie string) string {
	client, err := newDocLwpClient(cookie)
	if err != nil {
		return ""
	}
	if err := client.connect(); err != nil {
		return ""
	}
	defer client.close()
	resp, err := client.call("/r/Adaptor/DingTalkDocI/getAccessToken", nil, 20*time.Second)
	if err != nil {
		return ""
	}
	body := extractBody(resp)
	return firstNonEmptyText(getStringField(body, "accessToken"), findFirstString(body, "accessToken"), findFirstString(resp, "accessToken"))
}

type notableRequest struct {
	URL          string
	Method       string
	Data         any
	Params       map[string]string
	ResponseType string
}

func fetchNotableDocumentPayloads(opts *extractor.ExtractOpts, meta *notableRecordMeta) []map[string]any {
	c := util.NewClient()
	if opts != nil && opts.Cookies != nil {
		c.SetCookieJar(opts.Cookies)
	}
	cookie := cookieString(opts)
	requests := notableRequests(*meta)
	out := make([]map[string]any, 0, len(requests))
	for _, req := range requests {
		payload, err := notableHTTPRequest(c, req, *meta, cookie)
		if err != nil {
			out = append(out, map[string]any{"_source": req.URL, "_error": err.Error()})
			continue
		}
		value := notablePayloadToDebugValue(payload)
		out = append(out, map[string]any{"_source": req.URL, "payload": value})
		if strings.HasSuffix(req.URL, "/api/doc/info") {
			updateNotableMetaFromDocInfo(meta, value)
		}
	}
	return out
}

func notableRequests(meta notableRecordMeta) []notableRequest {
	out := []notableRequest{
		{URL: "https://alidocs.dingtalk.com/api/doc/info", Method: "POST", Data: map[string]any{"withGrays": true}, ResponseType: "json"},
		{URL: "https://alidocs.dingtalk.com/api/document/data", Method: "POST", Data: map[string]any{"fetchBody": map[string]any{"sheetId": meta.SheetID, "rowId": meta.RowID, "viewId": meta.ViewID}, "pageMode": 2}, ResponseType: "json"},
		{URL: "https://alidocs.dingtalk.com/nt/api/docs/preset", Method: "GET", ResponseType: "json"},
		{URL: "https://alidocs.dingtalk.com/nt/api/docs/preset/binary", Method: "GET", ResponseType: "bytes"},
	}
	for _, version := range uniqueStrings([]string{meta.Version, meta.DocVersion, "0"}) {
		versionValue := any(version)
		if n, err := strconv.Atoi(version); err == nil {
			versionValue = n
		}
		out = append(out,
			notableRequest{URL: fmt.Sprintf("https://alidocs.dingtalk.com/nt/api/sheets/%s/records/batch/binary", url.PathEscape(meta.SheetID)), Method: "POST", Data: map[string]any{"newFieldType": true, "version": versionValue, "recordIds": []string{meta.RowID}}, ResponseType: "bytes"},
			notableRequest{URL: fmt.Sprintf("https://alidocs.dingtalk.com/nt/api/sheets/%s/records/binary", url.PathEscape(meta.SheetID)), Method: "GET", Params: map[string]string{"recordIds": meta.RowID, "newFieldType": "true", "version": version}, ResponseType: "bytes"},
			notableRequest{URL: fmt.Sprintf("https://alidocs.dingtalk.com/nt/v2/api/sheet/%s/dsl/records", url.PathEscape(meta.SheetID)), Method: "POST", Data: map[string]any{"recordIds": []string{meta.RowID}, "version": versionValue}, ResponseType: "json"},
			notableRequest{URL: fmt.Sprintf("https://alidocs.dingtalk.com/nt/v2/api/sheet/%s/record/ids", url.PathEscape(meta.SheetID)), Method: "POST", Data: map[string]any{"recordIds": []string{meta.RowID}, "version": versionValue}, ResponseType: "json"},
		)
	}
	out = append(out, notableRequest{URL: "https://alidocs.dingtalk.com/nt/api/sheets/v2/rows", Method: "POST", Data: map[string]any{"sheetId": meta.SheetID, "query": map[string]any{"offset": 0, "limit": 100, "filter": []map[string]any{{"fieldId": "id", "type": "STRING", "value": meta.RowID, "symbol": "EQ"}}}}, ResponseType: "json"})
	return out
}

func notableHTTPRequest(c *util.Client, req notableRequest, meta notableRecordMeta, cookie string) (any, error) {
	rawURL := req.URL
	if len(req.Params) > 0 {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		for k, v := range req.Params {
			if v != "" {
				q.Set(k, v)
			}
		}
		u.RawQuery = q.Encode()
		rawURL = u.String()
	}
	headers := notableHeaders(meta, cookie, strings.ToUpper(req.Method) != "GET")
	var body []byte
	var err error
	if strings.EqualFold(req.Method, "GET") {
		body, err = c.GetBytes(rawURL, headers)
	} else {
		data, _ := json.Marshal(req.Data)
		resp, postErr := c.Post(rawURL, bytes.NewReader(data), headers)
		if postErr != nil {
			return nil, postErr
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("notable request failed: url=%s status=%d", rawURL, resp.StatusCode)
		}
		body, err = io.ReadAll(resp.Body)
	}
	if err != nil {
		return nil, err
	}
	return body, nil
}

func notableHeaders(meta notableRecordMeta, cookie string, jsonContent bool) map[string]string {
	headers := map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"Origin":     "https://alidocs.dingtalk.com",
		"Referer":    firstNonEmptyText(meta.RawURL, "https://alidocs.dingtalk.com/"),
		"User-Agent": docWebUA,
	}
	if jsonContent {
		headers["Content-Type"] = "application/json"
	}
	if cookie != "" {
		headers["Cookie"] = cookie
	}
	if meta.DentryUUID != "" {
		headers["A-DENTRY-UUID"] = meta.DentryUUID
	}
	for key, header := range map[string]string{"corp_id": "A-CORP-ID", "doc_access_token": "A-TOKEN", "dentry_key": "A-DENTRY-KEY", "host_doc_key": "A-HOST-DOC-KEY", "doc_key": "A-DOC-KEY"} {
		if v := metaField(meta, key); v != "" {
			headers[header] = v
		}
	}
	return headers
}

func notablePayloadToDebugValue(payload any) any {
	switch v := payload.(type) {
	case []byte:
		b := maybeDecompress(v)
		trimmed := bytes.TrimSpace(b)
		if len(trimmed) == 0 {
			return map[string]any{"_binary_length": len(v)}
		}
		var decoded any
		if json.Unmarshal(trimmed, &decoded) == nil {
			return decoded
		}
		text := string(trimmed)
		if strings.TrimSpace(text) != "" {
			return text
		}
		return map[string]any{"_binary_length": len(v)}
	case string:
		var decoded any
		if json.Unmarshal([]byte(v), &decoded) == nil {
			return decoded
		}
		return v
	default:
		return v
	}
}

func maybeDecompress(raw []byte) []byte {
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		if r, err := gzip.NewReader(bytes.NewReader(raw)); err == nil {
			defer r.Close()
			if b, err := io.ReadAll(r); err == nil {
				return b
			}
		}
	}
	if r, err := zlib.NewReader(bytes.NewReader(raw)); err == nil {
		defer r.Close()
		if b, err := io.ReadAll(r); err == nil {
			return b
		}
	}
	return raw
}

func updateNotableMetaFromDocInfo(meta *notableRecordMeta, payload any) {
	if meta == nil {
		return
	}
	root, ok := payload.(map[string]any)
	if !ok {
		return
	}
	data := getAnyMap(root["data"])
	if len(data) == 0 {
		data = root
	}
	global := getAnyMap(data["globalConfig"])
	docMeta := getAnyMap(global["docMeta"])
	snapshot := getAnyMap(data["snapshot"])
	portal := getAnyMap(docMeta["portalNodeInfo"])
	if meta.Permission == "" {
		meta.Permission = firstNonEmptyText(anyString(global["permission"]), anyString(snapshot["permission"]))
	}
	if meta.CorpID == "" {
		meta.CorpID = anyString(docMeta["corpId"])
	}
	if meta.DentryUUID == "" {
		meta.DentryUUID = anyString(docMeta["dentryUuid"])
	}
	if meta.DentryKey == "" {
		meta.DentryKey = anyString(docMeta["dentryKey"])
	}
	if meta.HostDocKey == "" {
		meta.HostDocKey = firstNonEmptyText(anyString(docMeta["hostDocKey"]), anyString(docMeta["docKey"]))
	}
	if meta.DocKey == "" {
		meta.DocKey = anyString(docMeta["docKey"])
	}
	if meta.Extra == nil {
		meta.Extra = map[string]any{}
	}
	for k, v := range map[string]any{
		"workspace_type": portal["workSpaceType"],
		"workspace_id":   portal["workSpaceId"],
		"node_id":        portal["nodeId"],
		"doc_name":       firstNonEmptyText(anyString(docMeta["docOriginalName"]), anyString(docMeta["docName"])),
	} {
		if anyString(v) != "" {
			meta.Extra[k] = v
		}
	}
}

func extractNotableFileMetas(payload any) []notableFileMeta {
	seen := map[string]bool{}
	var out []notableFileMeta
	walkJSON(payload, func(key string, value any) {
		var candidates []notableFileMeta
		switch v := value.(type) {
		case map[string]any:
			if notableMapHasFileKeys(v) {
				candidates = append(candidates, normalizeNotableFileMeta(v)...)
			}
		case []any:
			if notableListKey(key) {
				candidates = append(candidates, normalizeNotableFileMeta(v)...)
			}
		case string:
			if notableListKey(key) {
				candidates = append(candidates, normalizeNotableFileMeta(v)...)
			}
		}
		for _, meta := range candidates {
			key := meta.FileName + "|" + meta.DentryUUID + "|" + meta.FileID + "|" + meta.SpaceID
			if key == "|||" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, meta)
		}
	})
	return out
}

func normalizeNotableFileMeta(value any) []notableFileMeta {
	switch v := value.(type) {
	case string:
		var decoded any
		if json.Unmarshal([]byte(v), &decoded) != nil {
			return nil
		}
		return normalizeNotableFileMeta(decoded)
	case []any:
		var out []notableFileMeta
		for _, item := range v {
			out = append(out, normalizeNotableFileMeta(item)...)
		}
		return out
	case map[string]any:
		return []notableFileMeta{{
			Raw:        v,
			CorpID:     firstNonEmptyText(anyString(v["corpId"]), anyString(v["orgId"])),
			DentryUUID: firstNonEmptyText(anyString(v["dentryUuid"]), anyString(v["dentryUUID"]), anyString(v["uuid"])),
			FileID:     firstNonEmptyText(anyString(v["fileId"]), anyString(v["objectId"]), anyString(v["dentryId"]), anyString(v["driveDentryId"]), anyString(v["resourceId"])),
			SpaceID:    firstNonEmptyText(anyString(v["spaceId"]), anyString(v["bizId"]), anyString(v["driveSpaceId"]), anyString(v["workspaceId"])),
			FileType:   firstNonEmptyText(anyString(v["fileType"]), anyString(v["type"]), anyString(v["mimeType"]), anyString(v["contentType"])),
			FileName:   firstNonEmptyText(anyString(v["fileName"]), anyString(v["name"]), anyString(v["title"]), anyString(v["resourceName"])),
		}}
	default:
		return nil
	}
}

func resolveNotableFileMedia(cookie string, files []notableFileMeta, rawURL string) (*liveReplayResult, error) {
	client, err := newLwpClient(cookie)
	if err != nil {
		return nil, err
	}
	if err := client.connect(); err != nil {
		return nil, err
	}
	defer client.close()
	var errs []string
	for _, file := range files {
		if file.SpaceID == "" || file.FileID == "" {
			continue
		}
		meta := previewDentryMeta{
			SpaceID:    file.SpaceID,
			FileID:     file.FileID,
			DentryUUID: file.DentryUUID,
			CorpID:     file.CorpID,
			OrgID:      file.CorpID,
			FileName:   file.FileName,
			FileType:   file.FileType,
			RawURL:     rawURL,
		}
		_ = resolvePreviewDentryUUID(client, &meta)
		media, err := resolvePreviewMedia(client, meta)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if len(media.PlaybackURLs) == 0 {
			continue
		}
		media.Title = firstNonEmptyText(file.FileName, media.Title)
		media.Extra = map[string]any{"file_meta": file, "preview_meta": meta}
		return media, nil
	}
	return nil, fmt.Errorf("unable to resolve notable CSpace file media: %s", strings.Join(errs, " | "))
}

func notableMapHasFileKeys(m map[string]any) bool {
	keys := map[string]bool{}
	for k := range m {
		keys[strings.ToLower(k)] = true
	}
	for _, key := range []string{"fileid", "objectid", "dentryid", "drivedentryid", "dentryuuid", "spaceid", "bizid", "drivespaceid", "filename", "resourceid", "downloadurl", "previewurl", "fileurl", "videourl", "mp4url"} {
		if keys[key] {
			return true
		}
	}
	return false
}

func notableListKey(key string) bool {
	switch key {
	case "value", "files", "attachments", "fileList":
		return true
	default:
		return false
	}
}

func hasNonePermission(payloads []map[string]any, meta notableRecordMeta) bool {
	statuses := map[string]bool{}
	if meta.Permission != "" {
		statuses[strings.ToUpper(meta.Permission)] = true
	}
	for _, payload := range payloads {
		collectPermissionStatuses(payload["payload"], statuses)
	}
	return statuses["NONE"] || statuses["NO_PERMISSION"]
}

func collectPermissionStatuses(v any, out map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for _, key := range []string{"permission", "permissionStatus", "authStatus"} {
			if s := strings.ToUpper(anyString(x[key])); s != "" {
				out[s] = true
			}
		}
		for _, child := range x {
			collectPermissionStatuses(child, out)
		}
	case []any:
		for _, child := range x {
			collectPermissionStatuses(child, out)
		}
	}
}

func extractNotableTitle(payload any, meta notableRecordMeta) string {
	if title := findFirstString(getAnyMap(payload), "title", "fileName", "name", "recordName"); title != "" {
		return title
	}
	return firstNonEmptyText(anyString(meta.Extra["doc_name"]), meta.RowID, meta.SheetID, truncateString(meta.DentryUUID, 12), "notable_record")
}

func metaField(meta notableRecordMeta, key string) string {
	switch key {
	case "corp_id":
		return meta.CorpID
	case "doc_access_token":
		return meta.DocAccessToken
	case "dentry_key":
		return meta.DentryKey
	case "host_doc_key":
		return meta.HostDocKey
	case "doc_key":
		return meta.DocKey
	default:
		return ""
	}
}

func metaMap(meta notableRecordMeta, redact bool) map[string]any {
	out := map[string]any{
		"type":         meta.Type,
		"view_id":      meta.ViewID,
		"sheet_id":     meta.SheetID,
		"row_id":       meta.RowID,
		"dentry_uuid":  meta.DentryUUID,
		"version":      meta.Version,
		"doc_version":  meta.DocVersion,
		"corp_id":      meta.CorpID,
		"dentry_key":   meta.DentryKey,
		"host_doc_key": meta.HostDocKey,
		"doc_key":      meta.DocKey,
		"permission":   meta.Permission,
		"url":          meta.RawURL,
	}
	if meta.DocAccessToken != "" {
		if redact {
			out["doc_access_token"] = "<redacted>"
		} else {
			out["doc_access_token"] = meta.DocAccessToken
		}
	}
	for k, v := range meta.Extra {
		out[k] = v
	}
	return out
}

func getAnyMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func anyString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return strings.TrimSpace(x.String())
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func truncateString(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}
