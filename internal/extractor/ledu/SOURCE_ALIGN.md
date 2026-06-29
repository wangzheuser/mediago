# ledu 源码对齐对照

参考源码: `/home/sophomores/code/xwz-downloader-source-release/restored_source/Mooc/Courses/Ledu/`.

## 入口与课程列表

| Python 源码 | Go 实现 | 状态 |
|---|---|---|
| `Ledu_Base.get_subject_list` | `Extract` cookie 验证调用 `/course/v1/student/course/subject-list` | ✓ |
| `Ledu_Base.get_class_list` / `get_all_classes` | `fetchClasses` 分状态分页拉取 `/course/v1/student/course/list`, 并用 `fetchH5Classes` 对齐 H5/app fallback | ✓ |
| `Ledu_Course._get_cid` / `_find_course` | `parseClassID` / `chooseClass` | ✓ |

## 章节与课时

| Python 源码 | Go 实现 | 状态 |
|---|---|---|
| `get_course_detail_list` / `get_all_course_details` | `fetchCourseDetails` 按 type 1-4 分页拉取 `/course/v1/student/course/user-live-list` | ✓ |
| `_iter_curriculum_contexts` | `detailItemsFromClassInfo` 从 classInfo 内嵌 curriculum/regist 列表补课时上下文 | ✓ |
| `get_curriculum_lessons` | `fetchQueryLessons` 优先 H5 `/wx-aggregation/.../getCurriculumList`, fallback cloudlearn `/homepage/lessonDetailV0812/queryLessons` | ✓ |
| `get_lesson_detail` | `fetchLessonDetail` 优先 H5 `/wx-aggregation/.../lessonDetail`, fallback cloudlearn `/homepage/lessonDetailV0812/queryLessonDetail` | ✓ |
| `extract_video_info` | `buildEntries` 遍历 `sceneObject/sceneList`, 合并 scene/resource/task/live 字段并水合 playback/record/preview | ✓ |

## 视频源解析

| Python 源码 | Go 实现 | 状态 |
|---|---|---|
| `get_classroom_init_auth` / `get_classroom_init_student` | `classroomInitAuth`, `applyLeduInitContext`, `applyLeduResponseHeaders` | ✓ |
| `get_video_info` | `buildEntries` 调 `/playback/v4/video/init?from=YUNXUEXI` | ✓ |
| `get_real_record_init` | `fetchRealRecordInit` 调 `/classroom/basic/v1/real-record/init/auth` | ✓ |
| `get_record_resources` / `get_legacy_record_resources` | `fetchRecordResources` 调 v3, remote `resourcesUrl/resourceUrl`, legacy v1 | ✓ |
| `get_preview_video_url` / `_get_preview_video_source_url` | `fetchPreviewMedia` 调 preview behavior/source 接口 | ✓ |
| `_extract_all_m3u8_candidates` / `_extract_all_mp4_candidates` | `mediaURL`, `hasMediaCandidates`, `prepareLeduM3U8` | ✓ |

## 文件与签名

| Python 源码 | Go 实现 | 状态 |
|---|---|---|
| `get_course_materials` | `fetchCourseMaterials` H5 + cloudlearn 双路径 | ✓ |
| `download_material_item`, `get_note_url`, `get_paper_link`, `get_handout_pdf` | `resolveLeduMaterialURL` 解析直接 URL, note, paper link, handout PDF | ✓ |
| `_make_absolute_m3u8`, `process_key_or_iv` | `rewriteLeduM3U8`, `leduProcessKeyOrIV` 输出 data URL manifest | ✓ |
| `LeduAppCrypto`, H5/app signed headers, PC token refresh | `signing.go`: AES-CBC/PKCS7, HmacMD5, HmacSHA1, TAL sign, signed request headers, encrypted response解析 | ✓ |

## 验证

- `go test ./internal/extractor/ledu`
- `go vet ./internal/extractor/ledu/...`
