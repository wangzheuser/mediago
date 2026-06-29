package plaso

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sophomoresty/mediago/internal/extractor"
)

func TestExtractMock(t *testing.T) {
	routes := goldenLoadRoutes(t)
	goldenInstallTransport(t, routes)
	jar := goldenNewJar(t)
	got, err := (&Plaso{}).Extract("https://www.plaso.cn/?sfId=F1", &extractor.ExtractOpts{Cookies: jar})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	goldenAssertMedia(t, "plaso", got)
}

func TestEndpointVariantsAndAccessToken(t *testing.T) {
	cases := []struct {
		raw      string
		base     string
		platform string
	}{
		{"https://www.plaso.cn/course?id=1", "https://www.plaso.cn", "plaso"},
		{"https://www.aiwenyun.cn/course?id=1", "https://www.aiwenyun.cn", "aiwenyun"},
		{"https://jhpy.plaso.cn/course?id=1", "https://jhpy.plaso.cn", "jhpy"},
	}
	for _, tc := range cases {
		eps := newPlasoEndpoints(tc.raw)
		if eps.base != tc.base || eps.platform != tc.platform {
			t.Fatalf("endpoint %q = (%q,%q), want (%q,%q)", tc.raw, eps.base, eps.platform, tc.base, tc.platform)
		}
	}
	jar := goldenNewJar(t)
	goldenSetCookie(t, jar, "https://www.plaso.cn", "access_token", "tok-1")
	h := newPlasoEndpoints("https://www.plaso.cn/").headers(jar)
	if h["access-token"] != "tok-1" || !strings.Contains(h["Cookie"], "access_token=tok-1") {
		t.Fatalf("headers missing access token/cookie: %#v", h)
	}
}

func TestPlasoAlgorithms(t *testing.T) {
	if got := plasoPlayerURLEncrypt("abc"); got != "d54bdf" {
		t.Fatalf("plasoPlayerURLEncrypt = %q, want d54bdf", got)
	}
	u, q := pickPlayURL(map[string]any{"playUrls": map[string]any{"hd": map[string]any{"url": "https://cdn.example/hd.m3u8"}, "ld": "https://cdn.example/ld.m3u8"}}, "hd")
	if u != "https://cdn.example/hd.m3u8" || q != "hd" {
		t.Fatalf("pickPlayURL = (%q,%q)", u, q)
	}
	signed := buildPlasoCourseSTSSignedURL("course/file.pdf", plasoSTS{AccessKeyID: "ak", AccessKeySecret: "sk", SecurityToken: "tok", Region: "cn-shanghai", Bucket: "bucket"}, time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC))
	pu, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("parse signed URL: %v", err)
	}
	qv := pu.Query()
	if pu.Host != "bucket.oss-cn-shanghai.aliyuncs.com" || qv.Get("x-oss-signature-version") != "OSS4-HMAC-SHA256" || qv.Get("x-oss-security-token") != "tok" || len(qv.Get("x-oss-signature")) != 64 {
		t.Fatalf("unexpected v4 signed URL: %s", signed)
	}
}

func TestCollectFileItemsWithChapters(t *testing.T) {
	payload := map[string]any{"courseChapterList": []any{
		map[string]any{
			"title": "第一章",
			"children": []any{
				map[string]any{"name": "第1节", "files": []any{
					map[string]any{"fileId": "V1", "name": "视频", "url": "https://cdn.example/v1.m3u8", "type": "video"},
					map[string]any{"fileId": "D1", "name": "讲义", "location": "docs/a.pdf", "type": "pdf"},
				}},
			},
		},
	}}
	files := collectFileItems(payload)
	if len(files) != 2 {
		t.Fatalf("collectFileItems len=%d, want 2: %#v", len(files), files)
	}
	if files[0].Chapter != "第一章 / 第1节" || !reflect.DeepEqual(files[0].Index, []int{1, 1, 1}) {
		t.Fatalf("first item chapter/index = %q/%v", files[0].Chapter, files[0].Index)
	}
	if files[1].ID != "D1" || files[1].Chapter != "第一章 / 第1节" {
		t.Fatalf("document item not preserved: %#v", files[1])
	}
}

func TestExtractPackageChaptersAndFiles(t *testing.T) {
	routes := map[string]json.RawMessage{
		"__default":                   json.RawMessage(`{"code":0,"data":{},"list":[]}`),
		"POST /sc/nc/newGetShareInfo": json.RawMessage(`{"code":0,"data":{}}`),
		"POST /course/api/v1/nct/m/package/task/list": json.RawMessage(`{"code":0,"obj":{"courseChapterList":[{"title":"第一章","files":[{"fileId":"V1","name":"正课","url":"https://cdn.example/plaso/v1.m3u8","type":"video"},{"fileId":"D1","name":"讲义","url":"https://cdn.example/plaso/a.pdf","type":"pdf"}]}]}}`),
	}
	goldenInstallTransport(t, routes)
	got, err := (&Plaso{}).Extract("https://www.plaso.cn/course?packageId=P1", &extractor.ExtractOpts{Cookies: goldenNewJar(t)})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("entries len=%d, want 2: %#v", len(got.Entries), got.Entries)
	}
	video := got.Entries[0]
	if video.Extra["chapter"] != "第一章" || video.Streams["best"].Extra["chapter"] != "第一章" || video.Streams["best"].Format != "m3u8" {
		t.Fatalf("video chapter/stream not propagated: %#v / %#v", video.Extra, video.Streams["best"])
	}
	doc := got.Entries[1]
	if doc.Streams["best"].Format != "pdf" || doc.Extra["source_type"] != "direct" {
		t.Fatalf("document stream not resolved: %#v / %#v", doc.Extra, doc.Streams["best"])
	}
}

func goldenLoadRoutes(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile("testdata/sample.json")
	if err != nil {
		t.Fatalf("read sample fixture: %v", err)
	}
	routes := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &routes); err != nil {
		t.Fatalf("parse sample fixture: %v", err)
	}
	return routes
}

func goldenInstallTransport(t *testing.T, routes map[string]json.RawMessage) {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if raw, ok := goldenExactRoute(routes, r); ok {
			goldenWriteResponse(w, raw)
			return
		}
		if strings.HasSuffix(strings.ToLower(r.URL.Path), ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4,\nsegment.ts\n#EXT-X-ENDLIST\n"))
			return
		}
		if raw, ok := routes["__default"]; ok {
			goldenWriteResponse(w, raw)
			return
		}
		http.Error(w, `{"code":404,"data":{}}`, http.StatusNotFound)
	})
	httpSrv := httptest.NewServer(handler)
	httpsSrv := httptest.NewTLSServer(handler)
	oldDefault := http.DefaultTransport
	base, ok := oldDefault.(*http.Transport)
	if !ok {
		t.Fatalf("http.DefaultTransport is %T, want *http.Transport", oldDefault)
	}
	tr := base.Clone()
	tr.Proxy = nil
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	httpAddr := httpSrv.Listener.Addr().String()
	httpsAddr := httpsSrv.Listener.Addr().String()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		target := httpAddr
		if strings.HasSuffix(addr, ":443") {
			target = httpsAddr
		}
		return dialer.DialContext(ctx, network, target)
	}
	http.DefaultTransport = tr
	t.Cleanup(func() {
		http.DefaultTransport = oldDefault
		httpSrv.Close()
		httpsSrv.Close()
	})
}

func goldenExactRoute(routes map[string]json.RawMessage, r *http.Request) (json.RawMessage, bool) {
	for _, key := range []string{
		r.Method + " " + r.Host + r.URL.Path,
		r.Method + " " + r.URL.Path,
		r.Host + r.URL.Path,
		r.URL.Path,
	} {
		if raw, ok := routes[key]; ok {
			return raw, true
		}
	}
	return nil, false
}

func goldenWriteResponse(w http.ResponseWriter, raw json.RawMessage) {
	var body string
	if len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, &body) == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(raw)
}

func goldenNewJar(t *testing.T) http.CookieJar {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	return jar
}

func goldenSetCookie(t *testing.T, jar http.CookieJar, rawURL, name, value string) {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse cookie URL %q: %v", rawURL, err)
	}
	jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
}

func goldenAssertMedia(t *testing.T, site string, got *extractor.MediaInfo) {
	t.Helper()
	if got == nil {
		t.Fatalf("Extract returned nil MediaInfo")
	}
	if got.Site != site {
		t.Fatalf("Site = %q, want %q", got.Site, site)
	}
	if len(got.Streams) == 0 && len(got.Entries) == 0 {
		t.Fatalf("MediaInfo has no streams or entries: %#v", got)
	}
	for i, entry := range got.Entries {
		if entry == nil {
			t.Fatalf("Entries[%d] is nil", i)
		}
		if entry.Site != site {
			t.Fatalf("Entries[%d].Site = %q, want %q", i, entry.Site, site)
		}
		if len(entry.Streams) == 0 && len(entry.Entries) == 0 {
			t.Fatalf("Entries[%d] has no streams or child entries: %#v", i, entry)
		}
	}
}
