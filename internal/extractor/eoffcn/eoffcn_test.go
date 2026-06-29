package eoffcn

import "testing"

func TestParseParamsCapturesSpuID(t *testing.T) {
	p := parseParams(`https://www.eoffcn.com/goods/detail/abc123?spuId=SPU-88`)
	if p.SpuID != "SPU-88" {
		t.Fatalf("SpuID = %q, want %q", p.SpuID, "SPU-88")
	}
}

func TestCollectOldOrdersAndMatch(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"list": []any{
				map[string]any{
					"payMoney":  199.5,
					"spuId":     "SPU-1",
					"systemSn":  "SYS-1",
					"cargoName": "旧课一",
				},
				map[string]any{
					"payMoney": 99,
					"spuId":    "SPU-2",
					"systemSn": "SYS-2",
					"orderInfoExpand": map[string]any{
						"systemSn": "SYS-2",
					},
					"goodsName": "旧课二",
				},
			},
		},
	}
	orders := collectOldOrders(payload)
	if len(orders) != 2 {
		t.Fatalf("orders = %d, want 2", len(orders))
	}
	got, ok := matchOldOrder(orders, eoffcnParams{SpuID: "SPU-2"})
	if !ok {
		t.Fatalf("match by spuId failed")
	}
	if got.SystemOrder != "SYS-2" || got.Title != "旧课二" {
		t.Fatalf("matched order = %#v, want SYS-2/旧课二", got)
	}
	got, ok = matchOldOrder(orders, eoffcnParams{SystemOrder: "SYS-1"})
	if !ok {
		t.Fatalf("match by system order failed")
	}
	if got.SpuID != "SPU-1" || got.PayMoney != "199.5" {
		t.Fatalf("matched order = %#v, want SPU-1/199.5", got)
	}
}

func TestResolveEoffcnWhiteboardKeepsMediaAndStreamExtra(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"video_type": 6,
			"live_url":   "https://cdn.example.com/replay/index.m3u8",
			"board_context": map[string]any{
				"api_url": "https://pcvod.offcncloud.com/replay?room_num=ROOM-NUM&room_id=ROOM-ID&account=acct&k=key",
			},
		},
	}

	playback := resolveEoffcnPlayback(nil, nil, payload)
	if playback.URL != "https://cdn.example.com/replay/index.m3u8" {
		t.Fatalf("URL = %q, want media m3u8", playback.URL)
	}
	if playback.Extra["whiteboard"] != true {
		t.Fatalf("whiteboard extra = %#v, want true", playback.Extra["whiteboard"])
	}
	if playback.Extra["whiteboard_api_url"] != "https://pcvod.offcncloud.com/replay?room_num=ROOM-NUM&room_id=ROOM-ID&account=acct&k=key" {
		t.Fatalf("whiteboard_api_url = %#v", playback.Extra["whiteboard_api_url"])
	}

	info := mediaInfoWithExtra("board", playback.URL, eoffcnStreamHeaders(map[string]string{"Referer": "https://www.eoffcn.com"}, playback.URL, playback.Extra), playback.Extra)
	stream := info.Streams["best"]
	if stream.Extra["whiteboard"] != true {
		t.Fatalf("stream extra whiteboard = %#v, want true", stream.Extra["whiteboard"])
	}
	if stream.Format != "m3u8" || !stream.NeedMerge {
		t.Fatalf("stream format/merge = %s/%v, want m3u8/true", stream.Format, stream.NeedMerge)
	}
	if got := stream.Headers["Referer"]; got != "https://www.eoffcn.com" {
		t.Fatalf("Referer = %q, want original media referer", got)
	}
}

func TestExtractWatchDemandPlaybackBoardOnly(t *testing.T) {
	playback := extractWatchDemandPlayback(`{"data":{"video_type":6,"white_board_play_url":"https://pcvod.offcncloud.com/replay?room_num=RN&room_id=RID&account=acct&k=key"}}`)
	if playback.URL != "https://pcvod.offcncloud.com/replay?room_num=RN&room_id=RID&account=acct&k=key" {
		t.Fatalf("URL = %q, want board API URL", playback.URL)
	}
	if playback.Extra["whiteboard"] != true {
		t.Fatalf("whiteboard extra = %#v, want true", playback.Extra["whiteboard"])
	}
	params, ok := playback.Extra["whiteboard_params"].(map[string]string)
	if !ok {
		t.Fatalf("whiteboard_params = %#v, want map[string]string", playback.Extra["whiteboard_params"])
	}
	if params["room_num"] != "RN" || params["room_id"] != "RID" || params["account"] != "acct" || params["k"] != "key" {
		t.Fatalf("whiteboard params = %#v", params)
	}

	info := mediaInfoWithExtra("board-only", playback.URL, eoffcnStreamHeaders(map[string]string{"Referer": "https://www.eoffcn.com"}, playback.URL, playback.Extra), playback.Extra)
	stream := info.Streams["best"]
	if stream.Format != "html" {
		t.Fatalf("stream format = %q, want html", stream.Format)
	}
	if got := stream.Headers["Referer"]; got != eoffcnBoardReferer {
		t.Fatalf("Referer = %q, want %q", got, eoffcnBoardReferer)
	}
}
