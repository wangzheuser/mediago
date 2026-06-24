# youyuan 源码对齐对照

Source: `~/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses/Youyuan/`.

## URL 常量

| .cdc.py 行 | youyuan.go 行/名 | 一致? |
| --- | --- | --- |
| `Youyuan_Base.py:29 referer = 'https://h.yijiayk.com/'` | `youyuan.go:18 refererURL` | ✓ |
| `Youyuan_Course.py:30 course_info_api = 'https://m.yijiayk.com/course-api/app/course/getByCourseId?courseId={}'` | `youyuan.go:19 courseInfoAPI` | ✓ |
| `Youyuan_Course.py:31 chapter_list_api = 'https://m.yijiayk.com/course-api/app/courseChapter/listPresentOrPrevious?courseId={}&annualValue=0'` | `youyuan.go:20 chapterListAPI` | ✓ |
| `Youyuan_Course.py:32 video_token_api = 'https://m.yijiayk.com/course-api/app/courseVideo/getToken?chapterId={}&cacheId=0&clientType=pc'` | `youyuan.go:21 videoTokenAPI` | ✓ |
| `Youyuan_Course.py:33 bjy_api = 'https://www.baijiayun.com/vod/video/getPlayUrl?vid={}&token={}'` | `youyuan.go:22 bjyAPI` | ✓ |

## HTTP 调用

| 源码方法 | Go 函数 | method | 一致? |
| --- | --- | --- | --- |
| `Youyuan_Course.prepare` GET course info | `Extract` / `requestJSON` | GET | ✓ |
| `Youyuan_Course.prepare` GET chapter list | `Extract` / `requestJSON` | GET | ✓ |
| `Youyuan_Course.download` GET video token | `resolveLesson` | GET | ✓ |
| `Youyuan_Course.download` call Baijiayun play URL | `shared.BaijiayunResolveVOD` | GET | ✓ |

## JSON 字段映射

| 源码 key 链 | Go 解析 | 一致? |
| --- | --- | --- |
| course info `code == 200`, `data.courseName` | `Extract` reads `data.courseName` | ✓ |
| chapter list `data[]`, `chapterName`, `courseLessonList`, `chapterId`, `lessonName` | `collectLessons` walks `data`, `courseLessonList`, `chapterName`, `lessonName`, `id/chapterId` | ✓ |
| video token `data.videoId`, `data.token` | `resolveLesson` reads same keys | ✓ |
| Baijiayun result `video_url` / `url` / `play_url` | `shared.BaijiayunResolveVOD` handles play URL fields | ✓ |

## 阻塞步骤

无. The extractor uses the source API chain and delegates final playback resolution to the shared Baijiayun helper.
