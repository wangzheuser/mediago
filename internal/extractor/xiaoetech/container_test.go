package xiaoetech

import (
	"encoding/base64"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sophomoresty/mediago/internal/extractor"
	"github.com/Sophomoresty/mediago/internal/util"
)

func TestExtractColumnExpandsChildVideo(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch {
		case strings.Contains(r.URL.Path, "column.items.get"):
			_, _ = w.Write([]byte(`{"code":0,"data":{"list":[{"resource_id":"vid-1","resource_type":3,"resource_title":"Lesson 1"}]}}`))
		case strings.Contains(r.URL.Path, "video.detail_info.get"):
			_, _ = w.Write([]byte(`{"code":0,"data":{"video_m3u8_url":"https://media.example.com/xet/vid-1.m3u8"}}`))
		default:
			_, _ = w.Write([]byte(`{"code":0,"data":{"list":[]}}`))
		}
	})
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()
	httpsSrv := httptest.NewTLSServer(handler)
	defer httpsSrv.Close()
	installMockTransport(t, httpSrv.URL, httpsSrv.URL)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new cookie jar: %v", err)
	}
	media, err := (&Xiaoetech{}).Extract("https://demo.h5.xiaoeknow.com/p/course/column/col-1", &extractor.ExtractOpts{Cookies: jar})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(media.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(media.Entries))
	}
	got := media.Entries[0].Streams["default"].URLs[0]
	if got != "https://media.example.com/xet/vid-1.m3u8" {
		t.Fatalf("child URL = %q", got)
	}
}

func TestLiveMediaURLDecodesPrivateLookbackAndInlinesKey(t *testing.T) {
	privateURL := "https://media.example.com/live/private.m3u8"
	encoded := base64.StdEncoding.EncodeToString([]byte(privateURL))
	encoded = strings.TrimRight(encoded, "=")
	encoded = strings.ReplaceAll(strings.ReplaceAll(encoded, "+", "-"), "/", "_")
	obfuscated := "__ba" + encoded
	keyBytes := []byte("0123456789abcdef")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "get_lookback_list"):
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write([]byte(`{"code":0,"data":{"list":[{"aliveVideoUrlEncrypt":"` + obfuscated + `"}]}}`))
		case r.URL.Path == "/live/private.m3u8":
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"/distribute.vod.pri.get/1.0.0?token=abc\"\nseg.ts\n"))
		case r.URL.Path == "/distribute.vod.pri.get/1.0.0":
			_, _ = w.Write(keyBytes)
		default:
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write([]byte(`{"code":0,"data":{"list":[]}}`))
		}
	})
	httpSrv := httptest.NewServer(handler)
	defer httpSrv.Close()
	httpsSrv := httptest.NewTLSServer(handler)
	defer httpsSrv.Close()
	installMockTransport(t, httpSrv.URL, httpsSrv.URL)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new cookie jar: %v", err)
	}
	got, extra := liveMediaURL(util.NewClient(), jar, xetCtx{appID: "demo", xetDomain: xetDomainDefault, cid: "live-1", typ: "live"}, "live-1")
	if !strings.HasPrefix(got, "data:application/vnd.apple.mpegurl;base64,") {
		t.Fatalf("live URL = %q, extra=%v", got, extra)
	}
	text, ok := extra["m3u8_text"].(string)
	if !ok {
		t.Fatalf("m3u8_text missing: %#v", extra)
	}
	if !strings.Contains(text, `URI="data:application/octet-stream;base64,MDEyMzQ1Njc4OWFiY2RlZg=="`) {
		t.Fatalf("key was not inlined: %s", text)
	}
	if !strings.Contains(text, "https://media.example.com/live/seg.ts") {
		t.Fatalf("segment was not absolutized: %s", text)
	}
}
