package kuke

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/extractor/shared"
	"github.com/Sophomoresty/mediago/internal/util"
)

func kukeBuildItems(detail map[string]any, gid, subTitle string) []kukeItem {
	roots := records(detail["goodsCourseNodeList"])
	var items []kukeItem
	counters := map[string]int{"video": 0, "file": 0}
	var leaf func(map[string]any, int, int, string, string)
	leaf = func(node map[string]any, ch, sec int, chTitle, secTitle string) {
		title := strings.TrimSpace(firstText(node["title"], node["nodeName"], node["name"], node["courseName"], node["resourceName"]))
		if title == "" {
			return
		}
		nt := intOf(node["nodeType"])
		videoID := firstText(node["polyvVideoId"], node["videoId"], node["vid"])
		if (nt == 3 || videoID != "") && videoID != "" {
			counters["video"]++
			name := fmt.Sprintf("[%d.%d]--%s", ch, counters["video"], title)
			if secTitle != "" {
				name = fmt.Sprintf("[%d.%d.%d]--%s", ch, sec, counters["video"], title)
			}
			items = append(items, kukeItem{Kind: "video", Name: name, Chapter: firstText(subTitle, secTitle, chTitle), NodeID: firstText(node["id"], node["nodeId"]), GoodsMasterID: firstText(node["goodsMasterId"], gid), PolyvVideoID: videoID, Duration: intOf(node["videoDuration"])})
		}
		fileURL := firstText(node["resourceUrl"], node["fileUrl"], node["attachUrl"], node["sourceUrl"], node["downloadUrl"], node["url"])
		if videoID == "" && fileURL != "" {
			counters["file"]++
			name := fmt.Sprintf("(%d.%d)--%s", ch, counters["file"], title)
			if secTitle != "" {
				name = fmt.Sprintf("(%d.%d.%d)--%s", ch, sec, counters["file"], title)
			}
			items = append(items, kukeItem{Kind: "file", Name: name, Chapter: firstText(subTitle, secTitle, chTitle), FileURL: fileURL, FileFmt: firstText(node["extension"], node["fileExt"], node["fileType"], node["suffix"])})
		}
	}
	var walk func([]map[string]any, int, int, int, string, string)
	walk = func(nodes []map[string]any, depth, ch, sec int, chTitle, secTitle string) {
		for i, node := range nodes {
			title := strings.TrimSpace(firstText(node["title"], node["nodeName"], node["name"], node["courseName"]))
			children := firstRecords(node, "children", "childList", "childNodeList", "nodes")
			hasVideo := firstText(node["polyvVideoId"], node["videoId"], node["vid"]) != ""
			hasFile := firstText(node["resourceUrl"], node["fileUrl"], node["attachUrl"], node["sourceUrl"], node["downloadUrl"], node["url"]) != ""
			if len(children) > 0 && !hasVideo && !hasFile && (intOf(node["nodeType"]) == 1 || depth == 0) {
				if depth == 0 {
					ct := fmt.Sprintf("{%d}--%s", i+1, title)
					walk(children, 1, i+1, 1, ct, "")
				} else {
					st := fmt.Sprintf("{%d}--%s", i+1, title)
					walk(children, depth+1, ch, i+1, chTitle, st)
				}
				continue
			}
			if chTitle == "" {
				chTitle = "{1}--未分类"
			}
			leaf(node, ch, sec, chTitle, secTitle)
			if len(children) > 0 {
				walk(children, depth, ch, sec, chTitle, secTitle)
			}
		}
	}
	walk(roots, 0, 1, 1, "", "")
	return items
}

func kukeBuildVideoEntry(c *util.Client, headers map[string]string, item kukeItem, quality string) (*extractor.MediaInfo, error) {
	info := kukeFetchPolyvNodeInfo(c, headers, item)
	playSafe, vid := info.PlaySafe, firstText(info.VideoID, item.PolyvVideoID)
	if vid == "" {
		return nil, fmt.Errorf("kuke: empty polyv video id")
	}
	secureVid := kukeSecureVID(vid)
	manifest, token, seedConst, err := kukeFetchPolyvJS(c, secureVid, headers, playSafe, quality)
	if err != nil || manifest == "" {
		var sec *shared.PolyvSecure
		sec, err = shared.PolyvResolveSecure(c, secureVid, headers)
		if err == nil {
			token = firstText(token, sec.Data.Playsafe.Token)
			manifest, err = shared.PolyvPickBestManifest(sec)
		}
	}
	if err != nil || manifest == "" {
		return nil, fmt.Errorf("kuke polyv %s: %w", secureVid, err)
	}
	streamQuality := firstText(quality, "best")
	stream := extractor.Stream{Quality: streamQuality, URLs: []string{manifest}, Format: "m3u8", NeedMerge: true, Headers: map[string]string{"Referer": "https://www.kuke99.com/"}}
	extra := map[string]any{"node_id": item.NodeID, "polyv_video_id": item.PolyvVideoID, "goods_master_id": item.GoodsMasterID, "chapter": item.Chapter, "duration": item.Duration, "secure_vid": secureVid, "play_safe": token}
	if strings.HasPrefix(manifest, "http") && token != "" {
		if text, e := c.GetString(manifest, headers); e == nil && strings.HasPrefix(strings.TrimSpace(text), "#EXTM3U") {
			if rewritten, e := kukeRewritePolyvM3U8(c, text, manifest, token, seedConst, headers); e == nil {
				stream.URLs = []string{kukeM3U8DataURL(rewritten)}
				extra["m3u8_text"] = rewritten
				extra["source_type"] = "m3u8_text"
			}
		}
	}
	return &extractor.MediaInfo{Site: "kuke", Title: item.Name, Streams: map[string]extractor.Stream{"best": stream}, Extra: extra}, nil
}

func kukeFetchPolyvNodeInfo(c *util.Client, headers map[string]string, item kukeItem) kukePolyvInfo {
	data, err := kukeSignedPost(c, urlPolyvNodeInfo, map[string]any{"goodsType": 1, "videoId": "", "orgId": kukePolyvOrgID, "goodsMasterId": item.GoodsMasterID, "nodeId": item.NodeID}, headers, headers["kk-token"])
	if err != nil {
		return kukePolyvInfo{}
	}
	var node kukeNodeInfoData
	_ = json.Unmarshal(data, &node)
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	if node.KKAes != "" && node.KKSdkString != "" {
		if decoded := kukeDecodeSecurePayload(node.KKAes, node.KKSdkString); len(decoded) > 0 {
			raw = decoded
			node.PlaySafe = firstText(node.PlaySafe, decoded["playSafe"], decoded["token"], mapAny(decoded["playsafe"])["token"], decoded["playsafe"])
			node.Token = firstText(node.Token, decoded["token"])
			node.VideoID = firstText(node.VideoID, decoded["videoId"], decoded["vid"])
		}
	}
	return kukePolyvInfo{PlaySafe: firstText(node.PlaySafe, node.Token), VideoID: node.VideoID, Raw: raw}
}

func kukeFetchPolyvJS(c *util.Client, vid string, headers map[string]string, token string, quality string) (manifest string, outToken string, seedConst any, err error) {
	body, err := c.GetString(fmt.Sprintf(urlPolyvSecureJS, url.PathEscape(vid)), headers)
	if err != nil {
		return "", token, nil, err
	}
	info, err := kukeParsePolyvSecurePayload(body, vid)
	if err != nil {
		return "", token, nil, err
	}
	hls := polyvHLSList(info)
	if len(hls) == 0 {
		return "", firstText(token, info["playSafe"], info["token"], mapAny(info["playsafe"])["token"], info["playsafe"]), info["seed_const"], fmt.Errorf("empty hls")
	}
	picked := pickKukePolyvHLS(hls, quality)
	return normalizeM3U8(picked), firstText(token, info["playSafe"], info["token"], mapAny(info["playsafe"])["token"], info["playsafe"]), info["seed_const"], nil
}

func kukeParsePolyvSecurePayload(text, vid string) (map[string]any, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty polyv secure body")
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(text), &root); err != nil {
		if body := kukeExtractJSONObject(text); body != "" {
			_ = json.Unmarshal([]byte(body), &root)
		}
	}
	if len(root) == 0 {
		return nil, fmt.Errorf("polyv secure JSON parse failed")
	}
	return kukeDecodePolyvSecureInfo(vid, root), nil
}

func kukeExtractJSONObject(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return ""
	}
	return text[start : end+1]
}

func polyvHLSList(info map[string]any) []string {
	var out []string
	for _, key := range []string{"hls", "paths"} {
		switch list := info[key].(type) {
		case string:
			if strings.TrimSpace(list) != "" {
				out = append(out, strings.TrimSpace(list))
			}
		case []any:
			for _, item := range list {
				switch v := item.(type) {
				case string:
					if strings.TrimSpace(v) != "" {
						out = append(out, strings.TrimSpace(v))
					}
				case map[string]any:
					if u := firstText(v["url"], v["m3u8"], v["hls"], v["path"]); u != "" {
						out = append(out, u)
					}
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func pickKukePolyvHLS(hls []string, quality string) string {
	if len(hls) == 0 {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(quality)) {
	case "sd", "smooth", "low", "ld", "360p", "360", "480p", "480":
		return hls[0]
	case "hd", "high", "720p", "720":
		if len(hls) >= 2 {
			return hls[len(hls)-2]
		}
		return hls[len(hls)-1]
	default:
		return hls[len(hls)-1]
	}
}

func kukeRewritePolyvM3U8(c *util.Client, text, m3u8URL, token string, seedConst any, headers map[string]string) (string, error) {
	text = kukeAbsolutizeM3U8(text, m3u8URL)
	uriRe := regexp.MustCompile(`URI=["']([^"']+)["']`)
	var firstErr error
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "#EXT-X-KEY") {
			continue
		}
		m := uriRe.FindStringSubmatch(line)
		if len(m) != 2 {
			continue
		}
		keyURL := kukePolyvKeyURL(m[1], m3u8URL, token)
		if keyURL == "" {
			continue
		}
		keyBytes, err := c.GetBytes(keyURL, headers)
		if err != nil {
			firstErr = err
			continue
		}
		if decrypted := kukeDecryptPolyvKey(keyBytes, seedConst); len(decrypted) == 16 {
			keyBytes = decrypted
		}
		if len(keyBytes) != 16 {
			lines[i] = strings.Replace(line, m[1], keyURL, 1)
			continue
		}
		keyDataURL := "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(keyBytes)
		lines[i] = strings.Replace(line, m[1], keyDataURL, 1)
	}
	if firstErr != nil {
		return strings.Join(lines, "\n"), firstErr
	}
	return strings.Join(lines, "\n"), nil
}

func kukeAbsolutizeM3U8(text, m3u8URL string) string {
	uriRe := regexp.MustCompile(`URI=["']([^"']+)["']`)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(line, `URI="`) || strings.Contains(line, `URI='`) {
			lines[i] = uriRe.ReplaceAllStringFunc(line, func(match string) string {
				m := uriRe.FindStringSubmatch(match)
				if len(m) != 2 {
					return match
				}
				return strings.Replace(match, m[1], kukeJoinURL(m3u8URL, m[1]), 1)
			})
			continue
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(strings.ToLower(trimmed), "http") {
			lines[i] = kukeJoinURL(m3u8URL, trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

func kukePolyvKeyURL(rawURI, m3u8URL, token string) string {
	keyURL := kukeJoinURL(m3u8URL, rawURI)
	if keyURL == "" {
		return kukePolyvKeyURLFromM3U8(m3u8URL, token)
	}
	if !strings.Contains(keyURL, "/playsafe/") {
		if u, err := url.Parse(keyURL); err == nil {
			u.Path = "/playsafe/" + strings.TrimPrefix(u.Path, "/")
			keyURL = u.String()
		}
	}
	if token != "" {
		if u, err := url.Parse(keyURL); err == nil {
			q := u.Query()
			q.Set("token", token)
			u.RawQuery = q.Encode()
			keyURL = u.String()
		}
	}
	return keyURL
}

func kukePolyvKeyURLFromM3U8(m3u8URL, token string) string {
	m := regexp.MustCompile(`/([^/?#]+)_(\d+)\.m3u8(?:[?#]|$)`).FindStringSubmatch(m3u8URL)
	if len(m) != 3 {
		return ""
	}
	rawVID, bitrate := m[1], m[2]
	path1 := rawVID
	if len(path1) > 10 {
		path1 = path1[:10]
	}
	path2 := rawVID[len(rawVID)-1:]
	return fmt.Sprintf(urlPolyvKey, path1, path2, rawVID, bitrate, url.QueryEscape(token))
}

func kukeJoinURL(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "data:") {
		return ref
	}
	b, errB := url.Parse(base)
	r, errR := url.Parse(ref)
	if errB != nil || errR != nil {
		return ref
	}
	return b.ResolveReference(r).String()
}

func kukeM3U8DataURL(text string) string {
	return "data:application/vnd.apple.mpegurl;charset=utf-8," + url.PathEscape(text)
}

func kukeBuildFileEntry(item kukeItem) *extractor.MediaInfo {
	if item.FileURL == "" || item.Name == "" {
		return nil
	}
	fmtName := strings.TrimPrefix(strings.ToLower(item.FileFmt), ".")
	if fmtName == "" {
		fmtName = "dat"
	}
	return &extractor.MediaInfo{Site: "kuke", Title: item.Name, Streams: map[string]extractor.Stream{"best": {Quality: "best", URLs: []string{item.FileURL}, Format: fmtName, Headers: map[string]string{"Referer": "https://www.kuke99.com/"}}}, Extra: map[string]any{"chapter": item.Chapter}}
}

func kukeParseIDs(raw string) (cid, buyID string) {
	if m := kukeCourseRe.FindStringSubmatch(raw); len(m) > 0 {
		cid = firstText(m[1], m[2], m[3], m[4])
		buyID = firstText(m[5])
	}
	if u, err := url.Parse(raw); err == nil {
		q := u.Query()
		cid = firstText(q.Get("goodsMasterId"), q.Get("courseId"), q.Get("id"), cid)
		buyID = firstText(q.Get("userBuyUnitGoodsId"), buyID)
		if strings.Contains(u.Path, "/learn-center/live-detail") {
			cid = firstText(q.Get("id"), cid)
		}
	}
	return
}

func kukeIsSvip(course map[string]any) bool {
	return intOf(mapAny(course["content"])["goodsType"]) == 5
}

func kukeTitleFromDetail(d map[string]any) string {
	return firstText(d["goodsName"], d["courseName"], d["title"], d["goodsTitle"], deepText(d, "goodsInfo", "goodsName"), deepText(d, "goodsInfo", "courseName"), deepText(d, "goodsBaseInfo", "goodsName"), deepText(d, "goodsMasterInfo", "goodsName"), deepText(d, "goods", "goodsName"))
}

func kukeSecureVID(videoID string) string {
	base := strings.Split(videoID, "_")[0]
	if base == "" || videoID == "" {
		return videoID
	}
	return base + "_" + videoID[:1]
}

func normalizeM3U8(s string) string {
	if strings.HasPrefix(s, "http") || strings.HasPrefix(strings.TrimSpace(s), "#EXTM3U") {
		return s
	}
	if strings.HasPrefix(s, "/") {
		return shared.PolyvHLSPlayBase + s
	}
	return shared.PolyvHLSPlayBase + "/" + s
}

func kukeCookieString(jar http.CookieJar) string {
	if jar == nil {
		return ""
	}
	seen, parts := map[string]bool{}, []string{}
	for _, raw := range []string{"https://www.kuke99.com/", "https://kuke99.com/"} {
		u, _ := url.Parse(raw)
		for _, ck := range jar.Cookies(u) {
			if !seen[ck.Name] {
				seen[ck.Name] = true
				parts = append(parts, ck.Name+"="+ck.Value)
			}
		}
	}
	return strings.Join(parts, "; ")
}

func kukeCookieValue(cookie, name string) string {
	for _, part := range strings.Split(cookie, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == name {
			return kv[1]
		}
	}
	return ""
}

func kukeRandHex(n int) string {
	b := make([]byte, n)
	if _, err := crand.Read(b); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(b)
}

func mapAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func firstRecords(m map[string]any, keys ...string) []map[string]any {
	for _, key := range keys {
		if r := records(m[key]); len(r) > 0 {
			return r
		}
	}
	return nil
}

func records(v any) []map[string]any {
	switch x := v.(type) {
	case []map[string]any:
		return x
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, it := range x {
			if m, ok := it.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		for _, k := range []string{"goodsCourseNodeList", "courseList", "list", "items", "children", "data"} {
			if r := records(x[k]); len(r) > 0 {
				return r
			}
		}
	}
	return nil
}

func firstText(vals ...any) string {
	for _, v := range vals {
		if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

func scalarString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case int, int64, float64, float32, bool:
		return fmt.Sprint(x), true
	default:
		return "", false
	}
}

func intOf(v any) int {
	s, _ := scalarString(v)
	if s == "" {
		s = firstText(v)
	}
	f, _ := strconv.ParseFloat(s, 64)
	return int(f)
}

func deepText(m map[string]any, keys ...string) string {
	cur := any(m)
	for _, k := range keys {
		mm := mapAny(cur)
		cur = mm[k]
	}
	return firstText(cur)
}
