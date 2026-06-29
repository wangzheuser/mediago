package imooc

import "testing"

func TestParseURLPlaylistKeepsVideoAndMongoID(t *testing.T) {
	cid, mid, host := parseURL("https://www.imooc.com/course/playlist/2001?t=m3u8&_id=64fabc123&cdn=aliyun1")
	if host != "https://www.imooc.com" {
		t.Fatalf("host=%q", host)
	}
	if cid != "64fabc123" || mid != "2001" {
		t.Fatalf("cid=%q mid=%q, want mongo _id and playlist video id", cid, mid)
	}
}

func TestParseURLLessonFragmentMid(t *testing.T) {
	cid, mid, host := parseURL("https://coding.imooc.com/lesson/415.html#mid=33829")
	if host != "https://coding.imooc.com" {
		t.Fatalf("host=%q", host)
	}
	if cid != "415" || mid != "33829" {
		t.Fatalf("cid=%q mid=%q, want lesson id and fragment mid", cid, mid)
	}
}
