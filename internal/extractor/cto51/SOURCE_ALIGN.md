# cto51 源码对齐对照

Python 参考:

- `/home/sophomores/code/xwz-downloader-source-release/restored_source/Mooc/Courses/Cto51/Cto51_Course.py`
- `/mnt/e/LEL/dis_all_output/` 中的 Cto51 函数级反编译输出

## 入口与路由

| Python 逻辑 | Go 实现 | 覆盖点 |
|---|---|---|
| `_COURSE_RE`, `_LESSON_RE`, `_TRAIN_RE`, `_TRAIN_LESSON_RE` | `parseRoute`, `courseIDFromURL`, `lessonIDFromURL`, `trainCourseIDFromURL` | 普通课程, 单课, 训练营, `id=trainCourse_lesson` 组合课时 |
| `_get_course_list` | `resolveMyCourses`, `fetchMyCoursePayloads`, `courseRefsFromPayloads`, `courseRefsFromHTML` | 课程列表 API, 类型分页, 订单兜底, study HTML 兜底 |
| 单课/课程/训练营分发 | `Extract`, `resolveCourse`, `resolveLesson`, `resolveTraining`, `resolveTrainingLesson` | `ListOnly` 与实际下载解析分离 |

## 课程目录与章节

| Python 逻辑 | Go 实现 | 覆盖点 |
|---|---|---|
| `_fetch_lesson_page_payloads` | `fetchCoursePayloads`, `fetchPagedJSONPayloads` | `lesson-list`, `index-api`, 多页去重 |
| `_extract_lesson_list_from_payloads`, `_extract_outline_from_json` | `lessonsFromPayloads`, `lessonsFromAny`, `lessonRefFromMap` | `lesson_id/lessonId/id`, chapter title 继承, live/replay, training lesson |
| HTML 目录兜底 | `parseLessonLinks` | `/lesson/{id}.html` 链接兜底 |

## 视频源

| Python 逻辑 | Go 实现 | 覆盖点 |
|---|---|---|
| `_ALIPLAYPARAM_RE`, `_request_vod_play_auth` | `parseAliPlayParam`, `requestVodPlayAuth`, `authFromVodPayload` | `sign/vod_video_id_auth/playAuth`, 多参数候选, 响应 auth 覆盖请求 auth |
| `_request_qcloud_play_info`, `_rsa_encrypt_overlay` | `qcloudPlayParams`, `rsaEncryptOverlay`, `resolveAuth` | QCloud `getplayinfo/v4`, `cipheredOverlayKey/Iv`, `keyId`, `psign` |
| `_request_aliyun_play_info_by_rand` | `resolveAuth` + `shared.AliyunResolvePlayInfo` | Aliyun STS GetPlayInfo, `Rand`, m3u8 拉取, AliyunVoDEncryption key rewrite |
| 直接 URL/播放页兜底 | `collectMedia`, `resolvePlayPage`, `mediaFromText`, `videoFromText` | m3u8/mp4/flv/audio, live/play/replay 页面 |

## 文件资料

| Python 逻辑 | Go 实现 | 覆盖点 |
|---|---|---|
| `_fetch_lesson_file_list_payloads`, `_fetch_course_file_list_payloads` | `fetchCoursePayloads`, `fetchTrainingPayloads` | 课程资料, 课时资料, 训练营资料分页 |
| `_extract_files_from_json` | `filesFromPayloads`, `filesFromAny`, `fileRefsFromMap` | `fileUrl/downloadUrl/downUrl/attachUrl`, pack file, lesson/chapter metadata |
| `_extract_file_entries_from_html` | `filesFromHTML` | HTML anchor 与裸文件 URL |
| 下载项构造 | `fileEntry` | 文件 stream, format, size, headers, `Extra.type=file` |

## 验证

- `go test -count=1 ./internal/extractor/cto51/...`
- `go vet ./internal/extractor/cto51/...`
