package main

import (
	"testing"

	"github.com/nichuanfang/medigo/internal/extractor"
)

func TestResolutionFromQuality(t *testing.T) {
	tests := map[string]string{
		"1080p":  "1920x1080",
		"720p":   "1280x720",
		"480p":   "854x480",
		"source": "unknown",
	}
	for quality, want := range tests {
		if got := resolutionFromQuality(quality); got != want {
			t.Fatalf("resolutionFromQuality(%q) = %q, want %q", quality, got, want)
		}
	}
}

func TestCodecFromStream(t *testing.T) {
	tests := []struct {
		name   string
		stream extractor.Stream
		want   string
	}{
		{name: "dash", stream: extractor.Stream{Format: "dash"}, want: "h264+aac"},
		{name: "m3u8", stream: extractor.Stream{Format: "m3u8"}, want: "avc"},
		{name: "mp4", stream: extractor.Stream{Format: "mp4"}, want: "h264"},
		{name: "mpd url", stream: extractor.Stream{URLs: []string{"https://example.com/video.mpd"}}, want: "h264+aac"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := codecFromStream(tt.stream); got != tt.want {
				t.Fatalf("codecFromStream() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHumanSize(t *testing.T) {
	tests := map[int64]string{
		0:    "unknown",
		1:    "1B",
		1024: "1.0KiB",
		1536: "1.5KiB",
	}
	for size, want := range tests {
		if got := humanSize(size); got != want {
			t.Fatalf("humanSize(%d) = %q, want %q", size, got, want)
		}
	}
}
