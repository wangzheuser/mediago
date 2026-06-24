# luffycity 源码对齐对照

## URL 常量

| .cdc.py 行 | luffycity.go 行/名 | 一致? |
|---|---|---|
| `Luffycity_Base.py:33 referer = 'https://www.luffycity.com/'` | `luffycity.go:19 urlReferer = "https://www.luffycity.com/"` | ✓ |
| `Luffycity_Base.py:34 origin = 'https://www.luffycity.com'` | `luffycity.go:20 urlOrigin = "https://www.luffycity.com"` | ✓ |
| `Luffycity_Base.py:35 api_base = 'https://api.luffycity.com/api/v1'` | `luffycity.go:21 urlAPIBase = "https://api.luffycity.com/api/v1"` | ✓ |
| `Luffycity_Config.py:24 USER_AGENT = 'Mozilla/5.0 ... Chrome/124.0.0.0 ...'` | `luffycity.go:23 luffyUA = "Mozilla/5.0 ... Chrome/124.0.0.0 ..."` | ✓ |
| `Luffycity_Course.py:917 'https://vod.{}.aliyuncs.com/?{}'` | `aliyun.go:110 "https://vod." + region + ".aliyuncs.com/?"` | ✓ |
| `Luffycity_Course.py:1138 'https://mts.{}.aliyuncs.com/?'` | Aliyun license endpoint is source-recorded; extraction returns signed VOD play URL, no downloader license callback in `MediaInfo` | n/a |
| `Luffycity_Course.py:1403 'https://hcdn2.luffycity.com' + file_url` | `media.go:138-149 urlCDN = "https://hcdn2.luffycity.com"` | ✓ |

## HTTP 调用

| 源码方法 | Go 函数 | method | 一致? |
|---|---|---|---|
| `Luffycity_Base._check_cookie line 349` calls `/auth/token/`, `/study/courses/` | `luffyBuildSession line 96` | GET JSON | ✓ |
| `Luffycity_Base._request_json line 270` | `luffyAPIGet line 316` | GET JSON with `Referer`, `Origin`, `Authorization` | ✓ |
| `Luffycity_Course._get_course_list line 280` | `luffyFetchCourseList line 158` | GET `/study/courses/`, `/study/category-courses/`, `/course/{type}/` | ✓ |
| `Luffycity_Course._get_cid line 339` | `luffyResolveTarget line 114` | URL regex + course-list match | ✓ |
| `Luffycity_Course._get_title line 444` | `luffyFetchTitle line 233` | GET `/play/{section}/`, `/study/module/degree/{cid}/`, `/course/{type}/{cid}/` | ✓ |
| `Luffycity_Course._get_infos line 768` | `luffyFetchSections line 247` | GET `/play/sections/`, `/study/module/degree/{cid}/`, `/course/{type}/{cid}/sections/` | ✓ |
| `Luffycity_Course._resolve_polyv_source line 822` | `luffyResolvePlaySource line 286` | Polyv via `shared.PolyvResolveSecure` + `shared.PolyvPickBestManifest` | ✓ |
| `Luffycity_Course._resolve_aliyun_source line 1226` | `luffyResolveAliyun line 47` | GET `/media/play/{vid}/`, then signed Aliyun `GetPlayInfo` | ✓ |
| `Luffycity_Course._request_aliyun_play_info line 895` | `luffyRequestAliyunPlayInfo line 85` | signed GET `https://vod.{region}.aliyuncs.com/?...` | ✓ |

## JSON 字段映射

| 源码 key 链 | Go 解析 | 一致? |
|---|---|---|
| `_load_login_payload: key/token/authToken/auth_type, cookie, Authorization` | `luffyBuildSession`, `cookieValue(... luffy-client-key/token/key)`, `Authorization: Token ...` | ✓ |
| `_api_data: code in 0/200 or success, then data` | `luffyAPIData` | ✓ |
| `_normalize_course_item: course_id/courseId/id/cid/degree_course_id/actual_course_id` | `luffyNormalizeCourse` | ✓ |
| `_normalize_course_item: course_name/courseName/name/title` | `luffyNormalizeCourse` | ✓ |
| `_normalize_course_item: course_type/courseType/type, is_valid/is_buy/isBuy/purchased/has_buy` | `luffyNormalizeCourse` | ✓ |
| `_get_cid: play_id/study_id/actual_id/degree_id/free_id and query course_id/courseId/cid/id` | `luffyResolveTarget` regex + query parsing | ✓ |
| `_parse_sections_payload: chapters/sections/courses/course_list/courseList/modules/children/list` | `childMaps`, `luffyCollectItems` | ✓ |
| `_section_is_video: section_type/sectionType/type, vid/video_id/videoId, play_url` | `luffyIsVideo` | ✓ |
| `_make_video_info: id/section_id/sectionId, play_url/playUrl/video_url/videoUrl/url` | `luffyMakeVideoItem` | ✓ |
| `_resolve_play_source: player, auth_info.vid/video_id/videoId, auth_info.play_auth/playAuth/playauth` | `luffyResolvePlaySource`, `luffyResolveAliyun` | ✓ |
| `_decode_aliyun_play_auth: AccessKeyId/AccessKeySecret/SecurityToken/Region/RegionId/AuthInfo/AuthTimeout` | `luffyDecodeAliyunPlayAuth` | ✓ |
| `_extract_aliyun_play_response: PlayInfoList.PlayInfo[].PlayURL/PlayUrl/Size/Format/Definition/Encrypt` | `luffyExtractAliyunPlayResponse` | ✓ |
| `_normalize_url: //, /media/, /, http, media fallback` | `luffyNormalizeURL`, `luffyNormalizeMediaURL` | ✓ |

## 阻塞步骤

无.
