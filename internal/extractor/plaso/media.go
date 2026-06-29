package plaso

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sophomoresty/mediago/internal/extractor/shared"
)

type plasoSTS struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	Region          string
	Bucket          string
	Host            string
	Endpoint        string
	Raw             any
}

func (s *plasoSession) fetchAliPlaySource(f fileItem) plasoSource {
	id := firstNonEmpty(f.ID, f.MyID, f.VideoID)
	if id == "" && firstNonEmpty(f.Location, f.LocationPath) == "" {
		return plasoSource{}
	}
	data := s.playRequestData(f)
	if v, err := s.postJSON(s.eps.url(m3u8Path), data); err == nil {
		if src := s.sourceFromPlayInfo(v, "aliyun_play_info"); src.URL != "" {
			src.Extra = mergeExtra(src.Extra, map[string]any{"api": s.eps.url(m3u8Path)})
			return src
		}
	}
	return s.fetchAliSTSPlaySource(f)
}

func (s *plasoSession) fetchAliSTSPlaySource(f fileItem) plasoSource {
	videoID := firstNonEmpty(f.VideoID, f.Vid)
	if videoID == "" {
		return plasoSource{}
	}
	payload := shared.AliyunPlayPayload{}
	for _, api := range []string{s.eps.url(stsPath), s.eps.url(stsPreviewPath)} {
		v, err := s.postJSON(api, s.playRequestData(f))
		if err != nil {
			continue
		}
		walk(v, func(m map[string]any) {
			if payload.AccessKeyID != "" {
				return
			}
			candidate := shared.AliyunPayloadFromMap(m, m)
			if candidate.AccessKeyID != "" && candidate.AccessKeySecret != "" {
				if candidate.Region == "" {
					candidate.Region = firstNonEmpty(firstText(m, "region", "regionId", "Region"), "cn-shanghai")
				}
				payload = candidate
			}
		})
		if payload.AccessKeyID != "" {
			break
		}
	}
	if payload.AccessKeyID == "" {
		return plasoSource{}
	}
	info, err := shared.AliyunResolvePlayInfo(s.client, payload, videoID, shared.AliyunPlayOptions{
		Referer:           s.eps.base,
		Origin:            s.eps.base,
		Quality:           s.quality,
		PreferDefinitions: aliyunPreferDefinitions(s.quality),
		Headers:           streamHeaders(s.headers),
		FetchM3U8:         true,
		RewriteM3U8Keys:   true,
	})
	if err != nil {
		return plasoSource{}
	}
	return plasoSource{URL: info.URL, Format: firstNonEmpty(info.Format, formatOf(info.URL, f.Type)), Quality: firstNonEmpty(info.Definition, "best"), SourceType: info.SourceType, NeedMerge: info.NeedMerge, Size: info.Size, M3U8Text: info.M3U8Text, Extra: map[string]any{"aliyun_vid": videoID, "aliyun_api": info.APIURL, "encrypt_type": info.EncryptType}}
}

func (s *plasoSession) fetchPolyvSource(f fileItem) plasoSource {
	vid := firstNonEmpty(f.Vid)
	if vid == "" && looksPolyvVID(f.VideoID) {
		vid = f.VideoID
	}
	if vid == "" && f.ID != "" {
		if v, err := s.postJSON(s.eps.url(polySignPath), s.playRequestData(f)); err == nil {
			vid = findFirst(v, "vid", "polyvVid", "polyv_vid", "videoId", "video_id")
		}
	}
	if vid == "" {
		return plasoSource{}
	}
	if sec, err := shared.PolyvResolveSecure(s.client, vid, s.headers); err == nil {
		if manifest, err := shared.PolyvPickBestManifest(sec); err == nil {
			src := plasoSource{URL: s.normalizeMediaURL(manifest, ""), Format: "m3u8", Quality: "best", SourceType: "polyv", NeedMerge: true, Extra: map[string]any{"polyv_vid": vid}}
			if text, err := s.client.GetString(src.URL, streamHeaders(s.headers)); err == nil && strings.HasPrefix(strings.TrimSpace(text), "#EXTM3U") {
				if rewritten, err := shared.PolyvRewriteM3U8Keys(s.client, text, sec.Data.Playsafe.Token, s.eps.base); err == nil {
					src.M3U8Text = rewritten
				}
			}
			return src
		}
	}
	if body, err := s.client.PostForm(polyVideoURL, map[string]string{"vid": vid}, s.headers); err == nil {
		if m := mediaRe.FindString(body); m != "" {
			m = strings.ReplaceAll(m, `\/`, `/`)
			return plasoSource{URL: s.normalizeMediaURL(m, ""), Format: formatOf(m, f.Type), Quality: "best", SourceType: "polyv_video_info", NeedMerge: strings.Contains(strings.ToLower(m), ".m3u8"), Extra: map[string]any{"polyv_vid": vid}}
		}
	}
	if v, err := s.postJSON(s.eps.url(m3u8SignPath), map[string]string{"fileId": f.ID, "id": f.ID, "vid": vid}); err == nil {
		if src := s.sourceFromPlayInfo(v, "polyv_sign"); src.URL != "" {
			src.Extra = mergeExtra(src.Extra, map[string]any{"polyv_vid": vid, "api": s.eps.url(m3u8SignPath)})
			return src
		}
	}
	return plasoSource{}
}

func (s *plasoSession) fetchPlistSource(f fileItem) plasoSource {
	for _, raw := range []string{f.LocationPath, f.Location, f.URL} {
		plistURL := s.normalizeMediaURL(raw, "")
		if plistURL == "" || !strings.HasPrefix(plistURL, "http") || !isLikelyPlistURL(plistURL) {
			continue
		}
		text, err := s.client.GetString(plistURL, streamHeaders(s.headers))
		if err != nil {
			continue
		}
		var root any
		if err := json.Unmarshal([]byte(text), &root); err == nil {
			if src := s.pickPlistMedia(root, plistURL, f); src.URL != "" {
				src.Extra = mergeExtra(src.Extra, map[string]any{"plist_url": plistURL})
				return src
			}
		}
		if m := mediaRe.FindString(text); m != "" {
			m = s.normalizeMediaURL(m, plistURL)
			fmtv := formatOf(m, f.Type)
			return plasoSource{URL: m, Format: fmtv, SourceType: "plist_regex", NeedMerge: fmtv == "m3u8", Extra: map[string]any{"plist_url": plistURL}}
		}
	}
	return plasoSource{}
}

func (s *plasoSession) pickPlistMedia(root any, baseURL string, f fileItem) plasoSource {
	type cand struct {
		url, duration, startMS string
		audio                  bool
	}
	var cands []cand
	walk(root, func(m map[string]any) {
		if u, _ := pickPlayURL(m, s.quality); u != "" {
			cands = append(cands, cand{url: s.normalizeMediaURL(u, baseURL), audio: isAudioMap(m), duration: firstText(m, "duration"), startMS: firstText(m, "start_ms", "startMs")})
		}
		for _, entry := range asAnyList(m["media"]) {
			mm := asAnyMap(entry)
			if len(mm) == 0 {
				continue
			}
			u := firstText(mm, "m3u8Url", "m3u8URL", "url", "path", "location", "src")
			if u == "" {
				u, _ = pickPlayURL(mm, s.quality)
			}
			if u != "" {
				cands = append(cands, cand{url: s.normalizeMediaURL(u, baseURL), audio: isAudioMap(mm), duration: firstText(mm, "duration"), startMS: firstText(mm, "start_ms", "startMs")})
			}
		}
	})
	var picked, audio cand
	for _, c := range cands {
		if c.url == "" {
			continue
		}
		if c.audio {
			if audio.url == "" {
				audio = c
			}
			continue
		}
		picked = c
		break
	}
	if picked.url == "" {
		picked = audio
	}
	if picked.url == "" {
		return plasoSource{}
	}
	fmtv := formatOf(picked.url, f.Type)
	src := plasoSource{URL: picked.url, Format: fmtv, SourceType: "plist", NeedMerge: fmtv == "m3u8", AudioURL: audio.url, Extra: map[string]any{"duration": picked.duration, "start_ms": picked.startMS}}
	if fmtv == "m3u8" {
		if text, err := s.client.GetString(picked.url, streamHeaders(s.headers)); err == nil && strings.HasPrefix(strings.TrimSpace(text), "#EXTM3U") {
			src.M3U8Text = text
		}
	}
	return src
}

func (s *plasoSession) buildDirectDocumentSource(f fileItem) plasoSource {
	if !isDocumentLike(f) {
		return plasoSource{}
	}
	raw := firstNonEmpty(f.URL, f.LocationPath, f.Location)
	if raw == "" {
		return plasoSource{}
	}
	if direct := s.directSource(f, raw); direct.URL != "" {
		direct.SourceType = "direct_file"
		return direct
	}
	sts := s.fetchCourseSTS(f)
	if sts.AccessKeyID == "" || sts.AccessKeySecret == "" {
		return plasoSource{}
	}
	signedURL := buildPlasoCourseSTSSignedURL(raw, sts, time.Now().Add(time.Hour))
	if signedURL == "" {
		return plasoSource{}
	}
	return plasoSource{URL: signedURL, Format: formatOf(raw, f.Type), SourceType: "oss_sts_file", Size: f.Size, Extra: map[string]any{"oss_bucket": sts.Bucket, "oss_host": sts.Host, "oss_region": sts.Region}}
}

func (s *plasoSession) fetchCourseSTS(f fileItem) plasoSTS {
	var out plasoSTS
	for _, api := range []string{s.eps.url(stsPath), s.eps.url(stsPreviewPath)} {
		v, err := s.postJSON(api, s.playRequestData(f))
		if err != nil {
			continue
		}
		walk(v, func(m map[string]any) {
			if out.AccessKeyID != "" {
				return
			}
			ak := firstText(m, "AccessKeyId", "AccessKeyID", "accessKeyId", "accessKeyID", "access_key_id", "accessKey", "access_key", "accessId", "access_id", "OSSAccessKeyId", "id")
			sk := firstText(m, "AccessKeySecret", "accessKeySecret", "access_key_secret", "accessSecret", "access_secret", "secret")
			if ak == "" || sk == "" {
				return
			}
			out = plasoSTS{AccessKeyID: ak, AccessKeySecret: sk, SecurityToken: firstText(m, "SecurityToken", "securityToken", "security_token", "sts_token", "token"), Region: firstText(m, "region", "Region", "regionId", "domain_region"), Bucket: firstText(m, "bucket", "bucketName", "Bucket"), Host: firstText(m, "host", "Host", "domain", "ossHost", "downloadHost"), Endpoint: firstText(m, "endpoint", "Endpoint", "ossEndpoint"), Raw: m}
			out.Host = normalizeHost(out.Host)
			out.Endpoint = normalizeHost(out.Endpoint)
			if out.Region == "" {
				out.Region = regionFromEndpoint(firstNonEmpty(out.Endpoint, out.Host))
			}
		})
		if out.AccessKeyID != "" {
			break
		}
	}
	return out
}

func (s *plasoSession) sourceFromPlayInfo(v any, sourceType string) plasoSource {
	u, quality := pickPlayURL(v, s.quality)
	u = s.normalizeMediaURL(u, s.eps.base)
	if u == "" {
		return plasoSource{}
	}
	fmtv := formatOf(u, "video")
	src := plasoSource{URL: u, Format: fmtv, Quality: firstNonEmpty(quality, "best"), SourceType: sourceType, NeedMerge: fmtv == "m3u8", Size: parseSize(findFirstValue(v, "size", "Size", "fileSize"))}
	if fmtv == "m3u8" {
		if text, err := s.client.GetString(u, streamHeaders(s.headers)); err == nil && strings.HasPrefix(strings.TrimSpace(text), "#EXTM3U") {
			src.M3U8Text = text
		}
	}
	return src
}

func (s *plasoSession) playRequestData(f fileItem) map[string]string {
	data := map[string]string{}
	for _, kv := range [][2]string{
		{"fileId", f.ID}, {"id", f.ID}, {"myid", f.MyID}, {"myId", f.MyID},
		{"location", f.Location}, {"locationPath", f.LocationPath},
		{"storageId", f.StorageID}, {"storage_id", f.StorageID},
		{"vid", f.Vid}, {"videoId", f.VideoID},
	} {
		if strings.TrimSpace(kv[1]) != "" {
			data[kv[0]] = strings.TrimSpace(kv[1])
		}
	}
	return data
}

func (s *plasoSession) normalizeMediaURL(raw, baseURL string) string {
	u := strings.TrimSpace(strings.ReplaceAll(raw, `\/`, `/`))
	u = strings.Trim(u, "\"'")
	if u == "" {
		return ""
	}
	if decoded, err := url.QueryUnescape(u); err == nil && strings.Contains(decoded, "://") {
		u = decoded
	}
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if baseURL != "" {
		base, err1 := url.Parse(baseURL)
		ref, err2 := url.Parse(u)
		if err1 == nil && err2 == nil {
			return base.ResolveReference(ref).String()
		}
	}
	if strings.HasPrefix(u, "/") {
		return s.eps.base + u
	}
	return u
}

func (s *plasoSession) buildPlayerSource(f fileItem) plasoSource {
	if f.ID == "" || isDocumentLike(f) || (!isVideoLike(f) && firstNonEmpty(f.URL, f.Location, f.LocationPath, f.Vid, f.VideoID) != "") {
		return plasoSource{}
	}
	playerURL := plasoPlayerURL + url.QueryEscape(f.ID)
	return plasoSource{
		URL:        playerURL,
		Format:     "html",
		Quality:    "player",
		SourceType: "player_html",
		Extra: map[string]any{
			"player_url":           playerURL,
			"player_url_encrypted": plasoPlayerURLEncrypt(playerURL),
			"render_required":      true,
		},
	}
}

func buildPlasoCourseSTSSignedURL(raw string, sts plasoSTS, expiresAt time.Time) string {
	if firstNonEmpty(sts.Region, regionFromEndpoint(firstNonEmpty(sts.Endpoint, sts.Host))) != "" {
		if signed := buildPlasoCourseSTSV4SignedURL(raw, sts, expiresAt); signed != "" {
			return signed
		}
	}
	return buildPlasoCourseSTSV1SignedURL(raw, sts, expiresAt)
}

func buildPlasoCourseSTSV1SignedURL(raw string, sts plasoSTS, expiresAt time.Time) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, `\/`, `/`))
	if raw == "" {
		return ""
	}
	var u *url.URL
	var err error
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "//") {
		if strings.HasPrefix(raw, "//") {
			raw = "https:" + raw
		}
		u, err = url.Parse(raw)
		if err != nil {
			return ""
		}
	} else {
		object := strings.TrimLeft(raw, "/")
		host := firstNonEmpty(sts.Host, buildOSSHost(sts.Bucket, sts.Endpoint, sts.Region))
		if host == "" {
			return ""
		}
		u = &url.URL{Scheme: "https", Host: host, Path: "/" + object}
	}
	expires := strconv.FormatInt(expiresAt.Unix(), 10)
	bucket := firstNonEmpty(sts.Bucket, bucketFromHost(u.Host))
	canonical := u.EscapedPath()
	if bucket != "" && !strings.HasPrefix(canonical, "/"+bucket+"/") {
		canonical = "/" + bucket + canonical
	}
	if sts.SecurityToken != "" {
		canonical += "?security-token=" + url.QueryEscape(sts.SecurityToken)
	}
	toSign := "GET\n\n\n" + expires + "\n" + canonical
	mac := hmac.New(sha1.New, []byte(sts.AccessKeySecret))
	_, _ = mac.Write([]byte(toSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	q := u.Query()
	q.Set("OSSAccessKeyId", sts.AccessKeyID)
	q.Set("Expires", expires)
	q.Set("Signature", sig)
	if sts.SecurityToken != "" {
		q.Set("security-token", sts.SecurityToken)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func buildPlasoCourseSTSV4SignedURL(raw string, sts plasoSTS, expiresAt time.Time) string {
	u, ok := plasoOSSURL(raw, sts)
	if !ok {
		return ""
	}
	region := firstNonEmpty(sts.Region, regionFromEndpoint(firstNonEmpty(sts.Endpoint, u.Host)))
	if sts.AccessKeyID == "" || sts.AccessKeySecret == "" || region == "" {
		return ""
	}
	now := expiresAt.Add(-time.Hour).UTC()
	dateTime := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	scope := date + "/" + region + "/oss/aliyun_v4_request"
	q := u.Query()
	q.Set("x-oss-signature-version", "OSS4-HMAC-SHA256")
	q.Set("x-oss-date", dateTime)
	q.Set("x-oss-expires", "3600")
	q.Set("x-oss-credential", sts.AccessKeyID+"/"+scope)
	if sts.SecurityToken != "" {
		q.Set("x-oss-security-token", sts.SecurityToken)
	}
	canonicalQuery := q.Encode()
	canonicalHeaders := "host:" + strings.ToLower(u.Host) + "\n"
	canonicalRequest := strings.Join([]string{
		"GET",
		firstNonEmpty(u.EscapedPath(), "/"),
		canonicalQuery,
		canonicalHeaders,
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")
	hashed := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"OSS4-HMAC-SHA256",
		dateTime,
		scope,
		hex.EncodeToString(hashed[:]),
	}, "\n")
	signature := hex.EncodeToString(hmacSHA256(ossV4SigningKey(sts.AccessKeySecret, date, region), []byte(stringToSign)))
	q.Set("x-oss-signature", signature)
	u.RawQuery = q.Encode()
	return u.String()
}

func plasoOSSURL(raw string, sts plasoSTS) (*url.URL, bool) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, `\/`, `/`))
	if raw == "" {
		return nil, false
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		return u, err == nil
	}
	host := firstNonEmpty(sts.Host, buildOSSHost(sts.Bucket, sts.Endpoint, sts.Region))
	if host == "" {
		return nil, false
	}
	return &url.URL{Scheme: "https", Host: host, Path: "/" + strings.TrimLeft(raw, "/")}, true
}

func ossV4SigningKey(secret, date, region string) []byte {
	kDate := hmacSHA256([]byte("aliyun_v4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte("oss"))
	return hmacSHA256(kService, []byte("aliyun_v4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}
