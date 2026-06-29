package mddclass

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var mddclassBareMediaPathRe = regexp.MustCompile(`(?i)(?:^|/)[^"'\s<>]+\.(?:m3u8|mp4|flv|ts)(?:[?#].*)?$`)

func mddclassResolveOCSMediaURL(sess *mddclassSession, values ...any) string {
	for _, candidate := range mddclassCollectOCSMediaCandidates(values...) {
		mediaURL := mddclassNormalizeOCSResourceURL(candidate)
		if mediaURL == "" || mddclassIsPlaceholderURL(mediaURL) || !mddclassLooksLikeMediaURL(mediaURL) {
			continue
		}
		return mddclassSignOCSMediaURL(sess, mediaURL)
	}
	return ""
}

func mddclassCollectOCSMediaCandidates(values ...any) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(any, int)
	walk = func(value any, depth int) {
		if depth > 8 || value == nil {
			return
		}
		switch v := value.(type) {
		case map[string]any:
			for _, key := range []string{
				"media_url", "mediaUrl", "mediaURL", "video_url", "videoUrl",
				"play_url", "playUrl", "hls_url", "hlsUrl", "m3u8_url", "m3u8Url",
				"mp4_url", "mp4Url", "download_url", "downloadUrl", "source_url",
				"sourceUrl", "resource_url", "resourceUrl", "file_url", "fileUrl",
				"url", "href",
				"path", "filePath", "mediaPath", "resourcePath", "sourcePath",
				"playPath", "hlsPath", "m3u8Path", "mp4Path", "objectKey", "key",
			} {
				if raw := mddclassFirstText(v[key]); raw != "" {
					addOCSCandidate(raw, seen, &out)
				}
			}
			for _, key := range []string{"cdnHosts", "cdn_hosts", "hosts"} {
				if hosts, ok := v[key].([]any); ok {
					for _, host := range hosts {
						addOCSCandidate(mddclassFirstText(host), seen, &out)
					}
				}
			}
			for _, nested := range v {
				walk(nested, depth+1)
			}
		case []any:
			for _, item := range v {
				walk(item, depth+1)
			}
		case string:
			addOCSCandidate(v, seen, &out)
		}
	}
	for _, value := range values {
		walk(value, 0)
	}
	return out
}

func addOCSCandidate(raw string, seen map[string]bool, out *[]string) {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	raw = strings.ReplaceAll(raw, `\u0026`, "&")
	raw = strings.ReplaceAll(raw, `\/`, `/`)
	raw = strings.ReplaceAll(raw, "&amp;", "&")
	if raw == "" || seen[raw] {
		return
	}
	if strings.Contains(strings.ToLower(raw), "courseware-ocs") ||
		strings.Contains(strings.ToLower(raw), "p1-ocs") ||
		mddclassLooksLikeMediaURL(raw) ||
		mddclassBareMediaPathRe.MatchString(raw) {
		seen[raw] = true
		*out = append(*out, raw)
	}
}

func mddclassNormalizeOCSResourceURL(raw string) string {
	raw = mddclassNormalizeMediaURL(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if strings.HasPrefix(raw, "/") {
		return strings.TrimRight(mddclassOCSMaterialHost, "/") + raw
	}
	if strings.Contains(raw, "/") || mddclassBareMediaPathRe.MatchString(raw) {
		return strings.TrimRight(mddclassOCSMaterialHost, "/") + "/" + strings.TrimLeft(raw, "/")
	}
	return ""
}

func mddclassSignOCSMediaURL(sess *mddclassSession, raw string) string {
	if sess == nil || raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if token := mddclassFirstText(sess.Auth["ocsAccessToken"], sess.Auth["ocs_access_token"], sess.Auth["ocsPlayerAccessToken"], sess.Auth["playerAccessToken"]); token != "" {
		for _, key := range []string{"accessToken", "ocsAccessToken", "token"} {
			if q.Get(key) == "" {
				q.Set(key, token)
				break
			}
		}
	}
	if tenant := mddclassFirstText(sess.Auth["tenantId"], sess.Auth["tenant_id"]); tenant != "" && q.Get("tenantId") == "" {
		q.Set("tenantId", tenant)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func mddclassIsOCSURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return strings.Contains(host, "courseware-ocs") ||
		strings.Contains(host, "p1-ocs") ||
		strings.Contains(host, "sksight.com") && strings.Contains(strings.ToLower(u.Path), "ocs")
}

func mddclassOCSHint(sess *mddclassSession, coursewareInfo map[string]any) string {
	if sess == nil {
		return ""
	}
	hasOCSMarker := false
	for _, key := range []string{"coursewareId", "courseware_id", "courseWareId", "ocsId", "ocs_id", "tenantId", "tenant_id", "userSign", "user_sign", "userSignKey", "user_sign_key", "ocsAccessToken", "ocs_access_token"} {
		if mddclassFirstText(coursewareInfo[key]) != "" {
			hasOCSMarker = true
			break
		}
	}
	if !hasOCSMarker {
		return ""
	}
	missing := []string{}
	if mddclassFirstText(coursewareInfo["coursewareId"], coursewareInfo["courseware_id"], coursewareInfo["courseWareId"], coursewareInfo["ocsId"], coursewareInfo["ocs_id"]) == "" {
		missing = append(missing, "coursewareId")
	}
	if mddclassFirstText(coursewareInfo["tenantId"], coursewareInfo["tenant_id"], sess.Auth["tenantId"], sess.Auth["tenant_id"]) == "" {
		missing = append(missing, "tenantId")
	}
	if mddclassFirstText(coursewareInfo["userSign"], coursewareInfo["user_sign"], sess.Auth["userSign"], sess.Auth["user_sign"]) == "" {
		missing = append(missing, "userSign")
	}
	if len(missing) > 0 {
		return fmt.Sprintf("OCS courseware at %s requires %s", mddclassOCSReferer, strings.Join(missing, ", "))
	}
	return fmt.Sprintf("OCS courseware metadata is present; direct media URL is still absent after parsing %s", mddclassOCSReferer)
}
