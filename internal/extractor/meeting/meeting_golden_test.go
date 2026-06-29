package meeting

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
	"strings"
	"testing"
	"time"

	"github.com/Sophomoresty/mediago/internal/extractor"
)

func TestExtractMock(t *testing.T) {
	routes := goldenLoadRoutes(t)
	goldenInstallTransport(t, routes)
	jar := goldenNewJar(t)
	got, err := (&Meeting{}).Extract("https://meeting.tencent.com/cw/ABC123?pwd=secret", &extractor.ExtractOpts{Cookies: jar})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	goldenAssertMedia(t, "meeting", got)
}

func TestParseMeetingBatchText(t *testing.T) {
	items := parseMeetingBatchText(`标题：第一节课
链接：https://meeting.tencent.com/cw/ABC123
密码：pass123

第二节课
https://meeting.tencent.com/live/456789?pwd=livepwd
`)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2: %#v", len(items), items)
	}
	if items[0].URL != "https://meeting.tencent.com/cw/ABC123" || items[0].Password != "pass123" || items[0].Title != "第一节课" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].URL != "https://meeting.tencent.com/live/456789?pwd=livepwd" || items[1].Password != "livepwd" || items[1].Title != "第二节课" {
		t.Fatalf("unexpected second item: %#v", items[1])
	}
}

func TestExtractBatchText(t *testing.T) {
	routes := map[string]json.RawMessage{
		"POST /wemeet-tapi/v2/meetlog/public/detail/common-record-info":       json.RawMessage(`{"data":{"title":"Record","sharing_id":"SHARE1","recordings":[{"recording_id":"REC1"}]}}`),
		"GET /wemeet-cloudrecording-webapi/v1/sign":                           json.RawMessage(`{"data":{"origin_video_url":"https://media.example.com/meeting/rec1.mp4","title":"录制.mp4","id":"REC1"}}`),
		"POST /wemeet-tapi/liveportal/v2/query_live_stream":                   json.RawMessage(`{"data":{"room_id":"ROOM1"}}`),
		"POST /wemeet-tapi/liveportal/v2/query_meeting_room_live_replay_info": json.RawMessage(`{"data":{"replay_url_long":"https://media.example.com/meeting/live.m3u8","title":"直播回放"}}`),
		"__default": json.RawMessage(`{"data":{}}`),
	}
	goldenInstallTransport(t, routes)
	jar := goldenNewJar(t)
	raw := `标题：录播课
链接：https://meeting.tencent.com/cw/ABC123
密码：secret

直播课
https://meeting.tencent.com/live/456789?pwd=livepwd`
	got, err := (&Meeting{}).Extract(raw, &extractor.ExtractOpts{Cookies: jar})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("entries = %d, want 2: %#v", len(got.Entries), got.Entries)
	}
	assertMeetingEntryURL(t, got, "https://media.example.com/meeting/rec1.mp4")
	assertMeetingEntryURL(t, got, "https://media.example.com/meeting/live.m3u8")
	if !strings.HasPrefix(got.Entries[0].Title, "录播课_") {
		t.Fatalf("batch title prefix not merged: %q", got.Entries[0].Title)
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

func assertMeetingEntryURL(t *testing.T, got *extractor.MediaInfo, want string) {
	t.Helper()
	for _, entry := range got.Entries {
		for _, stream := range entry.Streams {
			for _, u := range stream.URLs {
				if u == want {
					return
				}
			}
		}
	}
	t.Fatalf("entry URL %s not found in %#v", want, got.Entries)
}
