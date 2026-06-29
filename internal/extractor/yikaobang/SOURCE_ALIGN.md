# yikaobang 源码对齐对照

Python source: `/home/sophomores/code/xwz-downloader-source-release/restored_source/Mooc/Courses/Yikaobang/`.
APK evidence: `yikaobang` Android 2.8.5.4 `NetworkRequestsURL.java` and course beans.

## Python 可见逻辑

- `Yikaobang_Base.py` 只提供 `referer = https://www.yikaobang.com.cn/` 与 cookie/token header 骨架.
- `Yikaobang_Course.prepare()` 显式标记缺少可靠课程/播放接口样本, 不提供伪实现.

## Go 补全策略

Go 实现保留 Python cookie/token 约束, 并基于 APK 中的 release endpoint/bean 字段补全运行链:

| 能力 | Go 实现 | 证据 |
| --- | --- | --- |
| 课程列表 | `course/main/courseList`, `course/center/list`, `course/main/search`, legacy `vidteaching/main/list` | APK `NetworkRequestsURL.courseMainList/courseLearnCenterList` |
| 课程详情/章节 | `course/main/detail`, `course/main/coursePackage`, `course/main/listAndUserPermission`, `course/center/catalogue`, legacy `vidteaching/main/chapter` | APK `CourseDirectoryItemData`, `CourseCalalogueBean` |
| 视频源 | 解析 `playUrl/videoUrl/m3u8/vid/free_watch_vid`, 无直链时用 `Course/CourseV2/getCourseAk` / `vidteaching/main/video` 补取 Aliyun STS | APK `CourseChapterBean`, `CourseAkBean`, `VideoDownTempBean` |
| 文件下载 | 解析 `course/center/handout` 和 payload 中 `url/fileUrl/downloadUrl/suffix/size_byte` | APK `VideoHandout` |

## 边界

- 不调用 `Course/Course/clock` 等打卡/统计写接口.
- 未从 APK 课程接口发现额外业务签名; Aliyun VOD 播放签名复用 `internal/extractor/shared` 的 HMAC-SHA1 OpenAPI 链.
