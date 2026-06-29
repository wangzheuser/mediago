# koolearn study 源码对齐补充

## 覆盖范围

| Python 源码 | Go 实现 | 状态 |
|---|---|---|
| `Koolearn_Course.course_url`, `course_kc_data`, `lesson_url` | `course.go` direct study route, lessonStage, category, recursive lesson walk | ✓ |
| `Koolearn_Chuguo.index_url/new_index_url/course-module/lesson/nodes` | `course.go` chuguo route, title/userCourseId, module/subject/node walk | ✓ |
| `Koolearn_Tiny.title_url/course-module/small-class/lesson/nodes` | `course.go` tiny route, module/category/node walk | ✓ |
| `getNewVideoInfo`, roombox `getVideoUrl`, live replay | `study.go` shared `resolveLeafStream`, `fetchNewVideoInfo`, `fetchRoomboxVideoURL`, `resolveLiveURL` | ✓ |

## 入口

- direct roombox URL: `classId/cid/classroomId` 仍走 `koolearn.go` roombox flow.
- direct study course URL: `study.koolearn.com/{tongyong,ky,fer,schedule,...}` / `chuguo` / `tiny-class` 走 `extractStudyCourse`.
- generic `koolearn.com` URL 无 class/course id 时, 保持 `my-data` 课程发现.

## 验证

- `go vet ./internal/extractor/koolearn/...`
- `go test ./internal/extractor/koolearn -count=1`
