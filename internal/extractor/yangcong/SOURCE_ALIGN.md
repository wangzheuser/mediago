# yangcong 源码对齐对照

Source: `~/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses/Yangcong/`.
Encrypted/truncated method bodies are cross-checked with `~/code/xwz-downloader-source-release/decrypted_full/all_decrypted.json` keys under `Courses/Yangcong/`.

## URL 常量

| .cdc.py 行 | yangcong.go 行/名 | 一致? |
| --- | --- | --- |
| `Yangcong_Base.py:29 referer = 'https://school.yangcongxueyuan.com/'` | `yangcong.go:16 refererURL` | ✓ |
| `Yangcong_Base.py:239 request_get('https://school-api.yangcong345.com/me', ...)` and `Yangcong_Course.py:44 me_url` | `yangcong.go:27 meURL` | ✓ |
| `Yangcong_Course.py:36 api_host = 'https://school-api.yangcong345.com'` | `yangcong.go:18 apiHost` | ✓ |
| `Yangcong_Course.py:37 subjects_url = api_host + '/course/subjects'` | `yangcong.go:19 subjectsURL` | ✓ |
| `Yangcong_Course.py:38 chapters_url = api_host + '/course/chapters-with-section/scene'` | `yangcong.go:20 chaptersURL` | ✓ |
| `Yangcong_Course.py:39 special_courses_url = api_host + '/course-tree/special-courses'` | `yangcong.go:21 specialCoursesURL` | ✓ |
| `Yangcong_Course.py:40 special_course_url = api_host + '/course/special-course/{}'` | `yangcong.go:22 specialCourseURL` | ✓ |
| `Yangcong_Course.py:41 topic_details_url = api_host + '/course-business/courseTree/getAnyTopicDetailsByIds'` | `yangcong.go:23 topicDetailsURL` | ✓ |
| `Yangcong_Course.py:42 video_addresses_url = api_host + '/videos/addresses'` | `yangcong.go:24 videoAddressesURL` | ✓ |
| `Yangcong_Course.py:43 order_auth_url = api_host + '/user-auths/order/auth'` | `yangcong.go:25 orderAuthURL` | ✓ |

## HTTP 调用

| 源码方法 | Go 函数 | method | 一致? |
| --- | --- | --- | --- |
| `Yangcong_Base._check_cookie` GET `/me` | `checkCookie` | GET | ✓ |
| `Yangcong_Course._json_get` | `getJSON` | GET | ✓ |
| `Yangcong_Course._json_post` | `postJSON` | POST JSON | ✓ |
| `Yangcong_Course._get_course_list` GET `subjects_url` | `Extract` warm-up call | GET | ✓ |
| `Yangcong_Course._get_infos` GET `chapters_url?...` | `fetchCoursePayload` | GET | ✓ |
| `Yangcong_Course._get_special_infos` GET `special_course_url` | `fetchCoursePayload` | GET | ✓ |
| `Yangcong_Course._get_video_url` POST `video_addresses_url` | `resolveVideo` | POST JSON | ✓ |

## JSON 字段映射

| 源码 key 链 | Go 解析 | 一致? |
| --- | --- | --- |
| `_check_cookie`: response contains `id`, `role` | `checkCookie` parses `id`, `role` from `/me` JSON | ✓ |
| `_get_course_list`: subjects response list | `getJSON` accepts top-level list as `list` | ✓ |
| `_get_infos`: `defaultBook`, `subjectId`, `stageId`, `publisherId`, `semesterId`, `semesterName`, `filterPublished=false` | `fetchCoursePayload` builds the same query keys and reads `defaultBook.name/title` | ✓ |
| `_get_chapter_info`: `sections`, `subsections`, `themes`, `topics`, `name` | `collectVideos` recursively walks the same child keys | ✓ |
| `_parse_video_info`: `videoId`, `video.id`, `id`, `name`, `type`, `pay` | `videoID` / `collectVideos` read `videoId`, nested `video.id`, `id`, `name` | ✓ |
| `_get_video_url`: `videoList`, `refinedExerciseId`, `topicId`, `custom.videoId`, `address` | `resolveVideo` posts the same keys and `pickAddress` walks `address` | ✓ |
| `_select_address`: `format`, `clarity`, `platform`, `url` | `pickAddress` ranks URL candidates by those keys | ✓ |

## 阻塞步骤

无. The Go extractor returns an error only if required course query IDs are missing, auth fails, or no playable `address.url` is returned.
