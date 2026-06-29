package xsteach

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sophomoresty/mediago/internal/util"
)

func TestRewriteQcloudM3U8AbsolutizesAndAppendsToken(t *testing.T) {
	text := "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\nseg-1.ts\n"
	got := rewriteQcloudM3U8(text, "https://cdn.example.com/path/index.m3u8", "tok")
	if !strings.Contains(got, `URI="https://cdn.example.com/path/key.bin?token=tok"`) {
		t.Fatalf("rewritten key missing token: %s", got)
	}
	if !strings.Contains(got, "https://cdn.example.com/path/seg-1.ts") {
		t.Fatalf("segment not absolutized: %s", got)
	}
}

func TestLoadFinalQcloudM3U8ReturnsDataURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/master.m3u8":
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=100,RESOLUTION=640x360\nlow/index.m3u8\n"))
		case "/low/index.m3u8":
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\nseg.ts\n"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := util.NewClient()
	u, text := loadFinalQcloudM3U8(c, qcloudPlayInfo{MasterURL: srv.URL + "/master.m3u8", DRMToken: "tok"})
	if !strings.HasPrefix(u, "data:application/vnd.apple.mpegurl;base64,") {
		t.Fatalf("url = %q", u)
	}
	if !strings.Contains(text, srv.URL+"/low/key.bin?token=tok") || !strings.Contains(text, srv.URL+"/low/seg.ts") {
		t.Fatalf("rewritten text = %s", text)
	}
	encoded := strings.TrimPrefix(u, "data:application/vnd.apple.mpegurl;base64,")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || string(decoded) != text {
		t.Fatalf("data URL content mismatch: %v", err)
	}
}
