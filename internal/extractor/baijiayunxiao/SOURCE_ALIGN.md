# baijiayunxiao 源码对齐对照

## URL 常量

| .cdc.py 行 | Go 行/名 | 一致? |
|---|---|---|
| `Baijiayun_Video.pyc.1shot.cdc.py:40-41` | `baijiayunxiao.go:24-27` `urlGetPlayInfo/urlGetPlayURL/urlHome` | ✓ |
| `Baijiayunxiao_Course.pyc.1shot.cdc.py:41-47` | `baijiayunxiao.go:29-31` `urlCourseInfo/urlToken/urlPlayToken` | ✓ |

## HTTP 调用

| 源码方法 | Go 函数 | method | 一致? |
|---|---|---|---|
| `Baijiayun_Video._resolve_live_enter_url` `221-262` | `resolveLiveEnter` `205-235` | POST JSON | ✓ |
| `Baijiayun_Video._get_play_info_data` `319-345` | `shared.BaijiayunResolvePlayback/ResolveVOD` `67-113` | GET | ✓ |
| `Baijiayunxiao_Course._get_infos` `1278-1363` | `resolveCourse` `130-163` | GET + parse | ✓ |
| `Baijiayunxiao_Course._get_token` `1368-1393` | `fetchLessonToken` `166-184` | GET + regex/JSON | ✓ |

## JSON / 结构映射

| 源码 key 链 | Go struct / parse | 一致? |
|---|---|---|
| `data.periods`, `data.chapter` | `courseInfoResponse.Data.Periods/Chapter` | ✓ |
| `id`, `video_id`, `room_id`, `title`, `name`, `periods_title`, `child`, `children` | `courseNode` tags | ✓ |
| `token`, `video_id`, `room_id`, `classid` | `playTokenResponse` tags + regex fallback | ✓ |
| `data.play_info`, `data.video_url`, `data.playback_url`, `data.video[].url` | `shared.BaijiayunPlaybackResponse` | ✓ |

## 阻塞步骤

无。
