# yizhiknow 源码对齐对照

Source: `~/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses/Yizhiknow/`.
Encrypted/truncated sections were cross-checked with `~/code/xwz-downloader-source-release/decrypted_source/Yizhiknow.py`.

## URL 常量

| .cdc.py 行 | yizhiknow.go 行/名 | 一致? |
| --- | --- | --- |
| `Yizhiknow_Base.py:36 referer = 'https://user.yizhiknow.com'` | `yizhiknow.go:19 refererURL` | ✓ |
| `Yizhiknow_Base.py:37 origin = 'https://user.yizhiknow.com'` | `yizhiknow.go:20 originURL` | ✓ |
| `Yizhiknow_Base.py:38 api_host = 'https://curriculum-api.yizhiknow.com'` | `yizhiknow.go:21 apiHost` | ✓ |
| `Yizhiknow_Base.py:39 api_secret = 'dcwsnmsb'` | `yizhiknow.go:22 apiSecret` | ✓ |
| `_check_cookie`: `/curriculum/user/getMultiPlatformMyCurricums` | `yizhiknow.go:23 listPath` | ✓ |
| `_request_live_courses`: `/curriculum/user/getMyselfLiveCurricumX` | `yizhiknow.go:24 liveListPath` | ✓ |
| `_load_detail`: `/curriculum/newDetailX` | `yizhiknow.go:25 detailPath` | ✓ |
| `_load_status`: `/curriculum/user/getCurriculumStatusV2` | `yizhiknow.go:26 statusPath` | ✓ |
| `_request_lesson_resource_result`: `/curriculum/getLessonResourceV2` | `yizhiknow.go:27 lessonResourcePath` | ✓ |
| `_request_live_resource`: `/curriculum/getPlayLiveSteamX` | `yizhiknow.go:28 liveResourcePath` | ✓ |

## HTTP 调用

| 源码方法 | Go 函数 | method | 一致? |
| --- | --- | --- | --- |
| `Yizhiknow_Base._request_json` signs payload, then calls API host | `requestJSON` + `signParams` | GET/POST JSON | ✓ |
| `_check_cookie` GET `getMultiPlatformMyCurricums?page=1&page_size=...` | `checkCookie` | GET | ✓ |
| `_load_detail` GET `newDetailX?curriculum_id={cid}` | `detail` | GET | ✓ |
| `_load_status` GET `getCurriculumStatusV2` | `Extract` status warm-up | GET | ✓ |
| `_request_lesson_resource_result` GET `getLessonResourceV2` | `resolveLesson` | GET | ✓ |
| `_request_live_resource` GET `getPlayLiveSteamX?vid_x={vid}` | `resolveLesson` fallback | GET | ✓ |

## JSON 字段映射

| 源码 key 链 | Go 解析 | 一致? |
| --- | --- | --- |
| token keys `token`, `Token`, `Access-Token`, `access_token`, `accessToken` | `tokenFromJar` accepts same names | ✓ |
| `_request_api_data`: response `code`, `data` | `requestData` validates `code` and returns `data` | ✓ |
| `_get_cid`: `/course/video/(\d+)`, query `curriculum_id/curriculumId/course_id/id` | `parseCID` | ✓ |
| `_get_title`: `curriculum_detail.title`, `title` | `Extract` title selection | ✓ |
| `_get_infos`: `lesson_list`, group `lesson`, chapter `name`, lesson `lesson_id/curriculum_lesson_id/type/title/study_material/stream_vod` | `collectLessons` | ✓ |
| `_collect_media_candidates`: `mp4_url`, `mp4Url`, `media_url`, `mediaUrl`, `url`, `play_url`, `playUrl` plus walked strings | `collectMediaCandidates` | ✓ |
| `_normalize_media_url`: `//` to `https:`, extensions `.m3u8/.mp4/.m4v/.mov/.flv/.mp3/.m4a/.aac/.wav` | `normalizeMediaURL` | ✓ |

## 阻塞步骤

无. The extractor skips downloader-only document conversion paths and returns resolved video/audio media URLs from the same lesson resource APIs.
