package gaotu

import "testing"

func TestEndpointsForBrandDomains(t *testing.T) {
	tests := []struct {
		name      string
		rawURL    string
		courseURL string
		infoURL   string
		videoURL  string
		liveURL   string
		sourceURL string
		fileURL   string
		priceURL  string
		referer   string
	}{
		{
			name:      "gaotu",
			rawURL:    "https://www.gaotu.cn/course?clazzNumber=G001",
			courseURL: "https://api.gaotu.cn/studyPlatform/v1/unit/clazz/list?isDebounce=true&os=h5-pc&p_client=1",
			infoURL:   "https://interactive.gaotu.cn/live/api/studyCenter/v1/user/pc/clazz/detail",
			videoURL:  "https://api.gaotu.cn/live/zplan/login/videoLive",
			liveURL:   "https://interactive.gaotu.cn/live/api/live/zplan/playbackWeb",
			sourceURL: "https://interactive.gaotu.cn/live/api/pan/listDir",
			fileURL:   "https://interactive.gaotu.cn/live/api/pan/file",
			priceURL:  "https://api.gaotu.cn/cs/api/product/course/detailButton?productSpuNumber=%s",
			referer:   "https://www.gaotu.cn",
		},
		{
			name:      "tutu",
			rawURL:    "https://gaotu100.com/course?clazzNumber=T001",
			courseURL: "https://api.gaotu100.com/studyPlatform/v1/unit/clazz/list?isDebounce=true&os=h5-pc&p_client=2",
			infoURL:   "https://interactive.gaotu100.com/live/api/studyCenter/v1/user/pc/clazz/detail",
			videoURL:  "https://api.gaotu100.com/live/zplan/login/videoLive",
			liveURL:   "https://interactive.gaotu100.com/live/api/live/zplan/playbackWeb",
			sourceURL: "https://interactive.gaotu100.com/live/api/pan/listDir",
			fileURL:   "https://interactive.gaotu100.com/live/api/pan/file",
			priceURL:  "https://api.gaotu100.com/cs/api/product/course/detailButton?productSpuNumber=%s",
			referer:   "https://gaotu100.com",
		},
		{
			name:      "gaozhong",
			rawURL:    "https://www.gtgz.cn/course?clazzNumber=H001",
			courseURL: "https://api.gtgz.cn/studyPlatform/v1/unit/clazz/list?isDebounce=true&os=h5-pc&p_client=8",
			infoURL:   "https://interactive.gtgz.cn/live/api/studyCenter/v1/user/pc/clazz/detail",
			videoURL:  "https://api.gtgz.cn/live/zplan/login/videoLive",
			liveURL:   "https://interactive.gtgz.cn/live/api/live/zplan/playbackWeb",
			sourceURL: "https://interactive.gtgz.cn/live/api/pan/listDir",
			fileURL:   "https://interactive.gtgz.cn/live/api/pan/file",
			priceURL:  "https://api.gtgz.cn/cs/api/product/course/detailButton?productSpuNumber=%s",
			referer:   "https://www.gtgz.cn",
		},
		{
			name:      "suyang",
			rawURL:    "https://www.naiyouxuexi.com/course?clazzNumber=S001",
			courseURL: "https://api.naiyouxuexi.com/studyPlatform/v1/unit/clazz/list?isDebounce=true&os=h5-pc&p_client=18",
			infoURL:   "https://interactive.naiyouxuexi.com/live/api/studyCenter/v1/user/pc/clazz/detail",
			videoURL:  "https://api.naiyouxuexi.com/live/zplan/login/videoLive",
			liveURL:   "https://interactive.naiyouxuexi.com/live/api/live/zplan/playbackWeb",
			sourceURL: "https://interactive.naiyouxuexi.com/live/api/pan/listDir",
			fileURL:   "https://interactive.naiyouxuexi.com/live/api/pan/file",
			priceURL:  "https://api.naiyouxuexi.com/cs/api/product/course/detailButton?productSpuNumber=%s",
			referer:   "https://www.naiyouxuexi.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := endpointsFor(tt.rawURL)
			if got.referer != tt.referer {
				t.Fatalf("referer = %q, want %q", got.referer, tt.referer)
			}
			if got.courseURL() != tt.courseURL {
				t.Fatalf("courseURL = %q, want %q", got.courseURL(), tt.courseURL)
			}
			if got.infoURL() != tt.infoURL {
				t.Fatalf("infoURL = %q, want %q", got.infoURL(), tt.infoURL)
			}
			if got.videoURL() != tt.videoURL {
				t.Fatalf("videoURL = %q, want %q", got.videoURL(), tt.videoURL)
			}
			if got.liveURL() != tt.liveURL {
				t.Fatalf("liveURL = %q, want %q", got.liveURL(), tt.liveURL)
			}
			if got.sourceURL() != tt.sourceURL {
				t.Fatalf("sourceURL = %q, want %q", got.sourceURL(), tt.sourceURL)
			}
			if got.fileURL() != tt.fileURL {
				t.Fatalf("fileURL = %q, want %q", got.fileURL(), tt.fileURL)
			}
			if got.priceURL() != tt.priceURL {
				t.Fatalf("priceURL = %q, want %q", got.priceURL(), tt.priceURL)
			}
		})
	}
}

func TestGaotuPriceFromPayload(t *testing.T) {
	price, ok := gaotuPriceFromPayload(map[string]any{
		"data": map[string]any{
			"coreButton": map[string]any{
				"price": "12345",
			},
		},
	})
	if !ok {
		t.Fatal("price not found")
	}
	if price != 123.45 {
		t.Fatalf("price = %v, want 123.45", price)
	}
}

func TestCollectGaotuPanNodes(t *testing.T) {
	nodes := collectGaotuPanNodes(map[string]any{
		"data": map[string]any{
			"dirList": []any{
				map[string]any{
					"entityType":   float64(1),
					"entityNumber": "DIR1",
					"name":         "资料目录",
					"rootNumber":   "ROOT1",
				},
				map[string]any{
					"entityType":   float64(2),
					"entityNumber": "DOC1",
					"url":          "https://cdn.example.com/handout.pdf?token=x",
					"name":         "讲义.pdf",
					"rootNumber":   "ROOT1",
				},
				map[string]any{
					"entityType":   float64(100),
					"entityNumber": "VID1",
					"url":          "https://interactive.gaotu.cn/play?vid=abc",
					"name":         "课堂回放",
					"rootNumber":   "ROOT1",
				},
			},
		},
	})
	if len(nodes) != 3 {
		t.Fatalf("len(nodes) = %d, want 3: %#v", len(nodes), nodes)
	}
	if !isGaotuDir(nodes[0]) {
		t.Fatalf("first node should be directory: %#v", nodes[0])
	}
	if nodes[1].ID != "DOC1" || nodes[1].Format != "pdf" || nodes[1].Root != "ROOT1" {
		t.Fatalf("doc node parsed incorrectly: %#v", nodes[1])
	}
	if nodes[2].Type != "100" || nodes[2].ID != "VID1" {
		t.Fatalf("video node parsed incorrectly: %#v", nodes[2])
	}
}
