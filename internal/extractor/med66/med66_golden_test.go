package med66

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
	goldenSetCookie(t, jar, "https://member.med66.com/", "cdeluid", "USER1")
	got, err := (&Med66{}).Extract("https://live.cdeledu.com/liveapi/entry/getReplayInfo?liveRoomId=ROOM1&accessid=ACC1&recordId=REC1&userid=USER1", &extractor.ExtractOpts{Cookies: jar})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	goldenAssertMedia(t, "med66", got)
}

func TestExtractCourseListFallback(t *testing.T) {
	routes := map[string]json.RawMessage{
		"POST /homes/mycourse/courseInfo": json.RawMessage(`{"code":200,"data":[{"courseId":"med123","title":"医学课程","eduSubjectId":10,"classType":2,"classId":3,"linkedCourseIds":"med123","isAi":0}]}`),
		"__default":                       json.RawMessage(`{"code":0,"data":{}}`),
	}
	goldenInstallTransport(t, routes)
	jar := goldenNewJar(t)
	goldenSetCookie(t, jar, "https://member.med66.com/", "cdeluid", "USER1")
	got, err := (&Med66{}).Extract("https://member.med66.com/homes/mycourse", &extractor.ExtractOpts{Cookies: jar})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if got.Title != "med66_courses" || len(got.Entries) != 1 {
		t.Fatalf("unexpected course list media: %#v", got)
	}
	if got.Entries[0].Extra["course_id"] != "med123" {
		t.Fatalf("course_id = %#v, want med123", got.Entries[0].Extra["course_id"])
	}
	if !strings.Contains(got.Entries[0].Extra["url"].(string), "courseId=med123") {
		t.Fatalf("course URL = %#v, want med123 query", got.Entries[0].Extra["url"])
	}
}

func TestMed66CourseIDEqualAcceptsMedPrefix(t *testing.T) {
	if !med66CourseIDEqual("123", "med123") || !med66CourseIDEqual("med123", "123") {
		t.Fatalf("med66CourseIDEqual should normalize optional med prefix")
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
		if r.Method == http.MethodPost && r.URL.Path == "/api/room/replay/login" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if got := r.Form.Get("userid"); got != "USER1" {
				http.Error(w, "unexpected userid "+got, http.StatusBadRequest)
				return
			}
			if got := r.Form.Get("accessid"); got != "ACC1" {
				http.Error(w, "unexpected accessid "+got, http.StatusBadRequest)
				return
			}
			if got := r.Form.Get("version"); got != MED66_CC_REPLAY_VERSION {
				http.Error(w, "unexpected version "+got, http.StatusBadRequest)
				return
			}
		}
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
