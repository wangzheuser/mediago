package ckjr

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sophomoresty/mediago/internal/util"
)

func TestCourseListAndLessonExtractionFromFixture(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal(readGoldenFixture(t), &root); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	courses := extractCourseListItems(root["marketing_award"], "", routeInfo{Company: "fixture", BaseURL: "https://school.ckjr001.com/kpv2p/fixture/"})
	if len(courses) != 1 {
		t.Fatalf("courses len = %d, want 1", len(courses))
	}
	course := courses[0]
	if course.ID != "1001" || course.Kind != "video" || course.ProdType != "5" || course.CourseType != "0" {
		t.Fatalf("course = %#v, want id=1001 kind=video prod=5 type=0", course)
	}
	if !strings.Contains(course.URL, "courseId=1001") || !strings.Contains(course.URL, "/homePage/course/video") {
		t.Fatalf("course URL = %q", course.URL)
	}

	r := routeCfg["video"]
	r.ID = "1001"
	entries, chapters := entriesFromPayloads(nil, r, []any{root["course_detail"]}, map[string]string{})
	if len(chapters) != 0 {
		t.Fatalf("chapters = %#v, want flat lesson fixture", chapters)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	stream := entries[0].Streams["best"]
	if stream.Format != "mp4" || len(stream.URLs) != 1 || stream.URLs[0] != "https://cdn.example/ckjr.mp4" {
		t.Fatalf("stream = %#v", stream)
	}
}

func TestQCloudPayloadCandidateFromFixture(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal(readGoldenFixture(t), &root); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	cands := qcloudCandidatesFromPayload(nil, asMap(root["qcloud_playinfo"]), nil, "", "")
	if len(cands) != 1 {
		t.Fatalf("qcloud candidates len = %d, want 1", len(cands))
	}
	if cands[0].URL != "https://cdn.example/ckjr.mp4" || cands[0].Format != "mp4" || cands[0].Kind != "video" {
		t.Fatalf("candidate = %#v", cands[0])
	}
}

func TestQCloudM3U8RewritesEncryptedKey(t *testing.T) {
	overlayKey := "00112233445566778899aabbccddeeff"
	overlayIV := "0102030405060708090a0b0c0d0e0f10"
	clearKey := []byte("0123456789abcdef")
	encryptedKey := aesCBCEncryptNoPad(t, clearKey, overlayKey, overlayIV)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/master.m3u8":
			fmt.Fprintf(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=100\n%s\n", srv.URL+"/variant.m3u8")
		case "/variant.m3u8":
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"/key.bin\"\n#EXTINF:1,\nseg.ts\n#EXT-X-ENDLIST\n")
		case "/key.bin":
			if got := r.URL.Query().Get("token"); got != "drm-token" {
				t.Errorf("key token = %q, want drm-token", got)
			}
			_, _ = w.Write(encryptedKey)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	variantURL, text := qcloudFinalM3U8(util.NewClient(), srv.URL+"/master.m3u8", "drm-token", nil, overlayKey, overlayIV)
	if variantURL != srv.URL+"/variant.m3u8" {
		t.Fatalf("variantURL = %q", variantURL)
	}
	wantKey := `URI="data:application/octet-stream;base64,` + base64.StdEncoding.EncodeToString(clearKey) + `"`
	if !strings.Contains(text, wantKey) {
		t.Fatalf("rewritten m3u8 missing decrypted data key %q in:\n%s", wantKey, text)
	}
	if !strings.Contains(text, srv.URL+"/seg.ts") {
		t.Fatalf("rewritten m3u8 missing absolute segment URL:\n%s", text)
	}
}

func aesCBCEncryptNoPad(t *testing.T, plain []byte, keyHex, ivHex string) []byte {
	t.Helper()
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	iv, err := hex.DecodeString(ivHex)
	if err != nil {
		t.Fatalf("decode iv: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	if len(plain)%aes.BlockSize != 0 {
		t.Fatalf("plain length %d is not block aligned", len(plain))
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return out
}
