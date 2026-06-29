package houdu

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor/shared"
)

type hdPlayback struct {
	URL        string
	Format     string
	NeedMerge  bool
	Extra      map[string]any
	Whiteboard houduWhiteboardInfo
}

type houduWhiteboardInfo struct {
	Whiteboard bool
	APIURL     string
	Params     map[string]string
	Source     string
}

func (x *hdCtx) getPlayURLForMode(lessonID, mode string) string {
	return x.getPlaybackForMode(lessonID, mode, nil).URL
}

func (x *hdCtx) getPlaybackForMode(lessonID, mode string, lesson map[string]any) hdPlayback {
	body := map[string]any{"lesson_id": coerceAPIID(lessonID)}
	lessonWB := detectHouduWhiteboard(lesson)
	tryAPI := func(path string) hdPlayback {
		resp, err := x.requestHoudu(path, body, "phoenix")
		if err != nil {
			return hdPlayback{}
		}
		wb := mergeHouduWhiteboardInfo(lessonWB, detectHouduWhiteboard(resp))
		if pb := x.extractPlayback(resp); pb.URL != "" {
			pb.Whiteboard = mergeHouduWhiteboardInfo(wb, pb.Whiteboard)
			pb.Extra = mergeExtra(pb.Extra, houduWhiteboardExtra(pb.Whiteboard))
			pb.Extra = mergeExtra(pb.Extra, map[string]any{"play_api_path": path, "play_mode": mode})
			return pb
		}
		data := asMap(x.extractData(resp))
		wb = mergeHouduWhiteboardInfo(wb, detectHouduWhiteboard(data))
		if pb := x.resolveBaijiayunFromMap(data); pb.URL != "" {
			pb.Whiteboard = mergeHouduWhiteboardInfo(wb, pb.Whiteboard)
			pb.Extra = mergeExtra(pb.Extra, houduWhiteboardExtra(pb.Whiteboard))
			pb.Extra = mergeExtra(pb.Extra, map[string]any{"play_api_path": path, "play_mode": mode})
			return pb
		}
		if pb := houduBoardOnlyPlayback(mode, data, wb); pb.URL != "" {
			pb.Extra = mergeExtra(pb.Extra, map[string]any{"play_api_path": path, "play_mode": mode})
			return pb
		}
		if stub := buildPlayStubURL(data); stub != "" {
			if pb := x.resolvePlaybackURL(stub); pb.URL != "" {
				pb.Whiteboard = mergeHouduWhiteboardInfo(wb, pb.Whiteboard)
				pb.Extra = mergeExtra(pb.Extra, houduWhiteboardExtra(pb.Whiteboard))
				pb.Extra = mergeExtra(pb.Extra, map[string]any{"play_api_path": path, "play_mode": mode})
				return pb
			}
		}
		return hdPlayback{}
	}
	switch mode {
	case "record":
		for _, path := range []string{"/mini/mini/recordLessonPlayURLForPC", "/mini/mini/recordLessonPlayURL", "/mini/mini/recordLessonPlayParams"} {
			if pb := tryAPI(path); pb.URL != "" {
				return pb
			}
		}
	case "playback":
		for _, path := range []string{"/mini/mini/lessonPlaybackURLForPC", "/mini/mini/lessonPlaybackURL", "/mini/mini/liveLessonPlayParams"} {
			if pb := tryAPI(path); pb.URL != "" {
				return pb
			}
		}
	default:
		// Source: first try lessonRoomURLForPC, then fall back to liveLessonPlayParams
		if pb := tryAPI("/mini/mini/lessonRoomURLForPC"); pb.URL != "" {
			return pb
		}
		return tryAPI("/mini/mini/liveLessonPlayParams")
	}
	return hdPlayback{}
}

func (x *hdCtx) extractPlayURL(resp map[string]any) string {
	return x.extractPlayback(resp).URL
}

func (x *hdCtx) extractPlayback(resp map[string]any) hdPlayback {
	data := x.extractData(resp)
	wb := detectHouduWhiteboard(data)
	if u := bestVideoURL(data); u != "" {
		return x.withHouduWhiteboard(x.resolvePlaybackURL(u), wb)
	}
	m := asMap(data)
	for _, key := range []string{"url", "play_url", "playUrl", "hls_url", "hlsUrl", "video_url", "videoUrl"} {
		if pb := x.resolvePlaybackURL(str(m[key])); pb.URL != "" {
			return x.withHouduWhiteboard(pb, wb)
		}
	}
	for _, s := range walkStrings(data) {
		if strings.HasPrefix(strings.TrimSpace(s), "http") {
			if pb := x.resolvePlaybackURL(s); pb.URL != "" {
				return x.withHouduWhiteboard(pb, wb)
			}
		}
	}
	if wb.Whiteboard && wb.APIURL != "" {
		return hdPlayback{URL: wb.APIURL, Format: whiteboardURLFormat(wb.APIURL), Whiteboard: wb, Extra: houduWhiteboardExtra(wb)}
	}
	return hdPlayback{}
}

func (x *hdCtx) resolvePlayURL(raw string) string {
	return x.resolvePlaybackURL(raw).URL
}

func (x *hdCtx) resolvePlaybackURL(raw string) hdPlayback {
	if raw == "" {
		return hdPlayback{}
	}
	if u := normalizeMediaURL(raw); u != "" {
		u = appendMiniToken(u, x.token)
		format := extFormat(u)
		return hdPlayback{URL: u, Format: format, NeedMerge: format == "m3u8"}
	}
	if pb := x.resolveBaijiayunURL(raw); pb.URL != "" {
		return pb
	}
	wb := houduWhiteboardFromURL(raw, "play_url")
	if wb.Whiteboard && wb.APIURL != "" {
		return hdPlayback{URL: wb.APIURL, Format: whiteboardURLFormat(wb.APIURL), Whiteboard: wb, Extra: houduWhiteboardExtra(wb)}
	}
	return hdPlayback{}
}

func (x *hdCtx) resolveBaijiayunFromMap(data map[string]any) hdPlayback {
	vid := firstString(data, "video_id", "vid", "live_id")
	roomID := firstString(data, "room_id", "roomid", "classid", "class_id")
	token := firstString(data, "token", "play_token", "playToken")
	if token == "" {
		return hdPlayback{}
	}
	wb := detectHouduWhiteboard(data)
	wb = mergeHouduWhiteboardInfo(wb, houduWhiteboardFromBaijiayunParams(vid, roomID, token, "baijiayun_params"))
	headers := map[string]string{"User-Agent": USER_AGENT, "Referer": referer}
	if x.c != nil && vid != "" {
		if u, err := shared.BaijiayunResolveVOD(x.c, vid, token, headers); err == nil {
			return x.withHouduWhiteboard(x.resolvePlaybackURL(u), wb)
		}
	}
	if x.c != nil && roomID != "" {
		if u, err := shared.BaijiayunResolvePlayback(x.c, roomID, token, headers); err == nil {
			return x.withHouduWhiteboard(x.resolvePlaybackURL(u), wb)
		}
	}
	if wb.Whiteboard && wb.APIURL != "" {
		return hdPlayback{URL: wb.APIURL, Format: whiteboardURLFormat(wb.APIURL), Whiteboard: wb, Extra: houduWhiteboardExtra(wb)}
	}
	return hdPlayback{}
}

func (x *hdCtx) resolveBaijiayunURL(playURL string) hdPlayback {
	u, err := url.Parse(strings.TrimSpace(playURL))
	if err != nil {
		return hdPlayback{}
	}
	q := u.Query()
	data := map[string]any{
		"video_id": firstNonEmpty(q.Get("video_id"), q.Get("vid"), q.Get("live_id")),
		"room_id":  firstNonEmpty(q.Get("room_id"), q.Get("roomid"), q.Get("classid"), q.Get("class_id")),
		"token":    q.Get("token"),
	}
	return x.resolveBaijiayunFromMap(data)
}

func (x *hdCtx) withHouduWhiteboard(pb hdPlayback, wb houduWhiteboardInfo) hdPlayback {
	if pb.URL == "" {
		return pb
	}
	pb.Whiteboard = mergeHouduWhiteboardInfo(pb.Whiteboard, wb)
	pb.Extra = mergeExtra(pb.Extra, houduWhiteboardExtra(pb.Whiteboard))
	return pb
}

func buildPlayStubURL(data map[string]any) string {
	token := firstString(data, "token", "play_token", "playToken")
	vid := firstString(data, "video_id", "vid", "live_id")
	roomID := firstString(data, "room_id", "roomid", "classid", "class_id")
	if token != "" && vid != "" {
		return fmt.Sprintf("https://h5.houduweilai.com/recordedCourses/play?video_id=%s&token=%s", url.QueryEscape(vid), url.QueryEscape(token))
	}
	if token != "" && roomID != "" {
		return fmt.Sprintf("https://h5.houduweilai.com/live/play?room_id=%s&token=%s", url.QueryEscape(roomID), url.QueryEscape(token))
	}
	return ""
}

func houduBoardOnlyPlayback(mode string, data map[string]any, base houduWhiteboardInfo) hdPlayback {
	stub := buildPlayStubURL(data)
	if stub == "" || (!base.Whiteboard && !isHouduBoardOnlyCandidate(mode, data)) {
		return hdPlayback{}
	}
	wb := mergeHouduWhiteboardInfo(base, houduWhiteboardFromBaijiayunParams(
		firstString(data, "video_id", "vid", "live_id"),
		firstString(data, "room_id", "roomid", "classid", "class_id"),
		firstString(data, "token", "play_token", "playToken"),
		"baijiayun_params",
	))
	wb.Whiteboard = true
	if wb.APIURL == "" {
		wb.APIURL = stub
	}
	if wb.Source == "" {
		wb.Source = "baijiayun_params"
	}
	return hdPlayback{URL: stub, Format: "html", Whiteboard: wb, Extra: houduWhiteboardExtra(wb)}
}

func detectHouduWhiteboard(v any) houduWhiteboardInfo {
	info := houduWhiteboardInfo{Params: map[string]string{}}
	var walk func(any, string)
	walk = func(x any, key string) {
		if isHouduWhiteboardKey(key) {
			info.Whiteboard = true
			if info.Source == "" {
				info.Source = key
			}
		}
		switch t := x.(type) {
		case map[string]any:
			for k, child := range t {
				if isHouduWhiteboardParamKey(k) {
					addHouduWhiteboardParam(info.Params, k, str(child))
				}
				if s, ok := child.(string); ok {
					considerHouduWhiteboardURL(&info, s, k, isHouduWhiteboardKey(k))
					if looksLikeHouduWhiteboardString(s) {
						info.Whiteboard = true
						if info.Source == "" {
							info.Source = k
						}
					}
				}
				walk(child, k)
			}
		case []any:
			for _, child := range t {
				walk(child, key)
			}
		case string:
			if looksLikeHouduWhiteboardString(t) {
				info.Whiteboard = true
				if info.Source == "" {
					info.Source = key
				}
			}
			considerHouduWhiteboardURL(&info, t, key, isHouduWhiteboardKey(key))
		}
	}
	walk(v, "")
	if len(info.Params) == 0 {
		info.Params = nil
	}
	return info
}

func houduWhiteboardFromURL(raw, source string) houduWhiteboardInfo {
	info := houduWhiteboardInfo{Params: map[string]string{}, Source: source}
	considerHouduWhiteboardURL(&info, raw, source, false)
	if len(info.Params) == 0 {
		info.Params = nil
	}
	return info
}

func houduWhiteboardFromBaijiayunParams(vid, roomID, token, source string) houduWhiteboardInfo {
	info := houduWhiteboardInfo{Params: map[string]string{}, Source: source}
	if vid != "" {
		addHouduWhiteboardParam(info.Params, "video_id", vid)
		info.APIURL = fmt.Sprintf("https://h5.houduweilai.com/recordedCourses/play?video_id=%s&token=%s", url.QueryEscape(vid), url.QueryEscape(token))
	}
	if roomID != "" {
		addHouduWhiteboardParam(info.Params, "room_id", roomID)
		if info.APIURL == "" {
			info.APIURL = fmt.Sprintf("https://h5.houduweilai.com/live/play?room_id=%s&token=%s", url.QueryEscape(roomID), url.QueryEscape(token))
		}
	}
	if token != "" {
		addHouduWhiteboardParam(info.Params, "token", token)
	}
	if len(info.Params) == 0 {
		info.Params = nil
	}
	return info
}

func mergeHouduWhiteboardInfo(a, b houduWhiteboardInfo) houduWhiteboardInfo {
	out := a
	if b.Whiteboard {
		out.Whiteboard = true
	}
	if out.APIURL == "" || houduWhiteboardURLScore(b.APIURL) > houduWhiteboardURLScore(out.APIURL) {
		out.APIURL = b.APIURL
	}
	if out.Source == "" {
		out.Source = b.Source
	}
	if len(b.Params) > 0 {
		if out.Params == nil {
			out.Params = map[string]string{}
		}
		for k, v := range b.Params {
			if _, ok := out.Params[k]; !ok && strings.TrimSpace(v) != "" {
				out.Params[k] = v
			}
		}
	}
	return out
}

func houduWhiteboardExtra(info houduWhiteboardInfo) map[string]any {
	if !info.Whiteboard {
		return nil
	}
	extra := map[string]any{
		"whiteboard":      true,
		"whiteboard_type": "baijiayun_or_houdu",
		"render_required": true,
	}
	if info.APIURL != "" {
		extra["whiteboard_api_url"] = info.APIURL
	}
	if len(info.Params) > 0 {
		extra["whiteboard_params"] = info.Params
	}
	if info.Source != "" {
		extra["whiteboard_source"] = info.Source
	}
	return extra
}

func considerHouduWhiteboardURL(info *houduWhiteboardInfo, raw, source string, fromBoardKey bool) {
	s := normalizeHouduURL(raw)
	if s == "" {
		return
	}
	collectHouduWhiteboardParams(info.Params, s)
	if !isHouduHTTPURL(s) {
		return
	}
	if fromBoardKey || isHouduWhiteboardURL(s) {
		info.Whiteboard = true
		if info.Source == "" {
			info.Source = source
		}
		if info.APIURL == "" || houduWhiteboardURLScore(s) > houduWhiteboardURLScore(info.APIURL) {
			info.APIURL = s
		}
	}
}

func collectHouduWhiteboardParams(params map[string]string, raw string) {
	if params == nil {
		return
	}
	u, err := url.Parse(normalizeHouduURL(raw))
	if err != nil {
		return
	}
	for key, values := range u.Query() {
		if len(values) > 0 && isHouduWhiteboardParamKey(key) {
			addHouduWhiteboardParam(params, key, values[0])
		}
	}
}

func addHouduWhiteboardParam(params map[string]string, key, value string) {
	if params == nil {
		return
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" || value == "<nil>" {
		return
	}
	if len(value) > 2048 {
		return
	}
	if _, ok := params[key]; !ok {
		params[key] = value
	}
}

func isHouduWhiteboardParamKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "room_id", "roomid", "classid", "class_id", "video_id", "videoid", "vid", "live_id", "liveid", "token", "play_token", "playtoken":
		return true
	default:
		return false
	}
}

func isHouduWhiteboardKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	for _, marker := range []string{"whiteboard", "white_board", "board", "board_only", "blackboard", "courseware"} {
		if strings.Contains(k, marker) {
			return true
		}
	}
	return false
}

func looksLikeHouduWhiteboardString(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if low == "" {
		return false
	}
	for _, marker := range []string{"whiteboard", "white_board", "board-only", "board_only", "blackboard", "board", "板书"} {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

func isHouduWhiteboardURL(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if !isHouduHTTPURL(low) {
		return false
	}
	if strings.Contains(low, "whiteboard") || strings.Contains(low, "white_board") || strings.Contains(low, "board") {
		return true
	}
	return false
}

func isHouduBoardOnlyCandidate(mode string, data map[string]any) bool {
	if mode != "playback" && mode != "record" {
		return false
	}
	token := firstString(data, "token", "play_token", "playToken")
	roomID := firstString(data, "room_id", "roomid", "classid", "class_id")
	return token != "" && roomID != ""
}

func houduWhiteboardURLScore(s string) int {
	low := strings.ToLower(strings.TrimSpace(s))
	if low == "" {
		return 0
	}
	score := 1
	if strings.Contains(low, "h5.houduweilai.com") {
		score += 100
	}
	if strings.Contains(low, "baijiayun.com") || strings.Contains(low, "bjcloud") {
		score += 70
	}
	if strings.Contains(low, "whiteboard") || strings.Contains(low, "white_board") || strings.Contains(low, "board") {
		score += 40
	}
	if strings.Contains(low, "room_id=") || strings.Contains(low, "video_id=") || strings.Contains(low, "vid=") || strings.Contains(low, "token=") {
		score += 20
	}
	if normalizeMediaURL(low) != "" {
		score -= 10
	}
	return score
}

func normalizeHouduURL(raw string) string {
	s := strings.TrimSpace(strings.Trim(raw, `"'`))
	s = strings.ReplaceAll(s, `\/`, `/`)
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	return s
}

func isHouduHTTPURL(raw string) bool {
	low := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://")
}

func whiteboardURLFormat(raw string) string {
	if normalizeMediaURL(raw) != "" {
		return extFormat(raw)
	}
	return "html"
}

func mergeExtra(dst, src map[string]any) map[string]any {
	if len(dst) == 0 && len(src) == 0 {
		return nil
	}
	out := map[string]any{}
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range src {
		out[k] = v
	}
	return out
}

func appendMiniToken(playURL, token string) string {
	if playURL == "" || token == "" || strings.Contains(playURL, "miniToken=") {
		return playURL
	}
	sep := "?"
	if strings.Contains(playURL, "?") {
		sep = "&"
	}
	return playURL + sep + "miniToken=" + url.QueryEscape(token)
}

func bestVideoURL(value any) string {
	m := asMap(value)
	if data := asMap(m["data"]); len(data) > 0 {
		m = data
	}
	playInfo := asMap(m["play_info"])
	if len(playInfo) == 0 {
		playInfo = asMap(m["playInfo"])
	}
	order := []string{"1080p", "superHD", "720p", "high", "480p", "standard"}
	for _, key := range order {
		variant := asMap(playInfo[key])
		for _, cdn := range listAt(variant, "cdn_list") {
			for _, urlKey := range []string{"enc_url", "url"} {
				if u := normalizeMediaURL(str(cdn[urlKey])); u != "" {
					return u
				}
			}
		}
		for _, urlKey := range []string{"enc_url", "url"} {
			if u := normalizeMediaURL(str(variant[urlKey])); u != "" {
				return u
			}
		}
	}
	for _, s := range walkStrings(value) {
		if u := normalizeMediaURL(s); u != "" {
			return u
		}
	}
	return ""
}
