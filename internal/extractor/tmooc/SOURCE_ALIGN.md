# tmooc 源码对齐对照

## URL 常量

| .cdc.py 行 | tmooc.go 行/名 | 一致? |
|---|---|---|
| Tmooc_Base.py:32-38 referer/tts/home/login/user URLs | `referer`, `tts_referer`, `home_url`, `tts_home_url`, `legacy_course_api`, `base_course_login_api`, `user_info_api` | ✓ |
| Tmooc_Course.py:32-38 TTS course APIs | `course_outline_api`, `my_course_api`, `valid_version_api`, `change_version_api`, `course_login_api`, `video_play_api` | ✓ |
| Tmooc_Course.py:39-44 web course / bokecc APIs | `user_course_api`, `web_check_video_api`, `web_course_detail_url`, `web_player_url`, `bokecc_video_api`, `bokecc_site_id` | ✓ |

## HTTP 调用

| 源码方法 (line) | Go 函数 | method | 一致? |
|---|---|---|---|
| `_request_course_list` 268-285 | `requestCourseList` | GET | ✓ |
| `_request_json` 291-308 | `requestJSON` | GET | ✓ |
| `_request_text` 317-332 | `extractWebCourse` / `GetString` | GET | ✓ |
| `_prepare_web_course` 649-696 | `extractWebCourse` | GET + HTML parse | ✓ |
| `_request_web_video_play_url` 752-804 | `resolveWebVideo` | GET + shared.BokeCCResolve | ✓ |
| `_prepare_tts_course` 702-746 | `extractTTSCourse` | GET | ✓ |
| `video_play_api` 38 / `_download_video` | `resolveTTSVideo` | GET | ✓ |

## JSON 字段映射

| 源码 key 链 | Go struct / map 访问 | 一致? |
|---|---|---|
| `data/list/records/rows/courseList/vailidVersionList` | `extractList` | ✓ |
| `studentClassroomId/studentClassId/stuClassId/id/courseId/...` | `collectIDs` / `firstID` | ✓ |
| `bigStageList/smallStageList/knowledgeList/videoList` | `collectTTSVideos` | ✓ |
| `data.playUrl/videoUrl/url/m3u8Url` | `findURL` / `resolveTTSVideo` | ✓ |
| `obj.guid/obj.ccGuid` | `resolveWebVideo` | ✓ |
| bokecc `vid + siteid` | `shared.BokeCCResolve` | ✓ |

## 阻塞步骤

无.
