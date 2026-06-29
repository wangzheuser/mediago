package ahu

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sophomoresty/mediago/internal/util"
)

func TestRequestAliyunPlayInfoRewritesVoDEncryption(t *testing.T) {
	key := []byte("0123456789abcdef")
	mediaID := "12345678-1234-1234-1234-123456789abc"
	challenge := "challenge-token"
	keyToken := base64.StdEncoding.EncodeToString([]byte(mediaID + challenge))
	keyDataURL := "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(key)

	var playConfigSeen bool
	var licenseHits int
	installAhuMockTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.Host, "vod.") && r.URL.Query().Get("Action") == "GetPlayInfo":
			playConfigSeen = strings.Contains(r.URL.Query().Get("PlayConfig"), "AliyunVoDEncryption")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"PlayInfoList": map[string]any{"PlayInfo": []map[string]any{{
					"PlayURL":     "https://cdn.example.com/master.m3u8",
					"Definition":  "HD",
					"Format":      "m3u8",
					"Encrypt":     "1",
					"EncryptType": "AliyunVoDEncryption",
				}}},
			})
		case r.Host == "cdn.example.com" && r.URL.Path == "/master.m3u8":
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\nvariant/index.m3u8\n"))
		case r.Host == "cdn.example.com" && r.URL.Path == "/variant/index.m3u8":
			_, _ = w.Write([]byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-KEY:METHOD=AES-128,URI="/keys/key1"
#EXTINF:4,
seg1.ts
#EXT-X-ENDLIST
`))
		case r.Host == "cdn.example.com" && r.URL.Path == "/keys/key1":
			_, _ = w.Write([]byte(keyToken))
		case strings.HasPrefix(r.Host, "mts.") && r.Method == http.MethodPost:
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse license form: %v", err)
			}
			if r.PostForm.Get("Action") != "GetLicense" || r.PostForm.Get("MediaId") != mediaID || r.PostForm.Get("data") != challenge {
				t.Fatalf("unexpected license form: %v", r.PostForm)
			}
			licenseHits++
			_ = json.NewEncoder(w).Encode(map[string]string{"License": base64.StdEncoding.EncodeToString(key)})
		default:
			t.Fatalf("unexpected request: %s %s%s", r.Method, r.Host, r.URL.String())
		}
	}))

	playAuth, _ := json.Marshal(map[string]string{
		"AccessKeyId":     "ak-test",
		"AccessKeySecret": "secret-test",
		"SecurityToken":   "token-test",
		"Region":          "cn-shanghai",
		"AuthInfo":        "auth-test",
	})
	got, err := requestAliyunPlayInfo(util.NewClient(), "video-test", string(playAuth), map[string]string{"Referer": course_list_url})
	if err != nil {
		t.Fatalf("requestAliyunPlayInfo returned error: %v", err)
	}
	if !playConfigSeen {
		t.Fatal("GetPlayInfo request did not include AliyunVoDEncryption PlayConfig")
	}
	if licenseHits != 1 {
		t.Fatalf("GetLicense hits = %d, want 1", licenseHits)
	}
	const prefix = "data:application/vnd.apple.mpegurl;base64,"
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("expected prepared m3u8 data URL, got %q", got)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, prefix))
	if err != nil {
		t.Fatalf("decode prepared m3u8: %v", err)
	}
	text := string(decoded)
	if !strings.Contains(text, `URI="`+keyDataURL+`"`) {
		t.Fatalf("prepared m3u8 missing inline data key:\n%s", text)
	}
	if !strings.Contains(text, "https://cdn.example.com/variant/seg1.ts") {
		t.Fatalf("prepared m3u8 did not absolutize segment URL:\n%s", text)
	}
}

func installAhuMockTransport(t *testing.T, handler http.Handler) {
	t.Helper()
	plain := httptest.NewServer(handler)
	tlsSrv := httptest.NewTLSServer(handler)

	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatal("unexpected default transport type")
	}
	oldTransport := baseTransport.Clone()
	oldProxy := util.DefaultProxy()
	if err := util.SetDefaultProxy(""); err != nil {
		t.Fatal(err)
	}

	plainURL, err := url.Parse(plain.URL)
	if err != nil {
		t.Fatal(err)
	}
	tlsURL, err := url.Parse(tlsSrv.URL)
	if err != nil {
		t.Fatal(err)
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tr := oldTransport.Clone()
	tr.Proxy = nil
	tr.ForceAttemptHTTP2 = false
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, plainURL.Host)
	}
	tr.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		raw, err := dialer.DialContext(ctx, network, tlsURL.Host)
		if err != nil {
			return nil, err
		}
		conn := tls.Client(raw, &tls.Config{InsecureSkipVerify: true})
		if err := conn.HandshakeContext(ctx); err != nil {
			_ = raw.Close()
			return nil, err
		}
		return conn, nil
	}
	http.DefaultTransport = tr

	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		_ = util.SetDefaultProxy(oldProxy)
		plain.Close()
		tlsSrv.Close()
	})
}
