# xuetang 源码对齐对照

## URL 常量

| Python 源码 | Go 实现 | 一致? |
|---|---|---|
| `Xuetang_Course.py:23` `/course/{sign:}/{cid:}` | `xuetang.go` `urlCourseRe` | ✓ |
| `Xuetang_Course.py:24` `/api/v1/lms/learn/product/info?cid={}&sign={}` | `extractCourse` title request | ✓ |
| `Xuetang_Course.py:25` `/api/v1/lms/learn/course/chapter?cid={}&sign={}` | `fetchCourseChapterPayload` | ✓ |
| `Xuetang_Course.py:26` `/api/v1/lms/product/get_course_detail/?cid={cid}` | `fetchCourseChapterPayload` fallback | ✓ |
| `Xuetang_Course.py:27` `/api/v1/lms/learn/leaf_info/{}/{}/?sign={}` | `resolveLeafSource` | ✓ |
| `Xuetang_Course.py:28` `/api/v1/lms/service/playurl/{}/?appid=10000` | `fetchPlayURLVariants` | ✓ |
| `Xuetang_Course.py:29` `/api/v1/lms/service/video2ccsource/{}/` | `getLiveVideoURL` | ✓ |
| `Xuetang_Course.py:32` `/api/v1/lms/product/sku_pay_detail/?cid={}&sign={}` | not required for extraction, source URL retained by course flow docs | N/A |
| `Xuetang_Course.py:33` `/api/v1/lms/learn/training/camp/classrooms/?sign={}` | `fetchTrainingClassroomID` | ✓ |
| `Xuetang_Live.py:24` `/api/v1/lms/learn/live_info/{}/{}/?sign={}` | `extractLive` | ✓ |

## URL 与 Origin

| Python 源码 | Go 实现 | 一致? |
|---|---|---|
| `Mooc_Config.py:263` training `/training/{sign}/{id}` | `urlTrainingRe`, `TestParseURLSourceExamples` | ✓ |
| `Mooc_Config.py:264` live `/live/{sign}/.../{cid}/{tid}` | `urlLiveRe`, `TestParseURLSourceExamples` | ✓ |
| `Mooc_Config.py:265` course `/course/{sign}/{cid}` and `/learn[/space]/{sign}/.../{cid}` | `urlCourseRe`, `urlLearnRe` | ✓ |
| `Xuetang_Base._origin_from_url`: `gradsmartedu.cn` → `https://www.gradsmartedu.cn`, otherwise `https://www.xuetangx.com` | `xuetangOrigin` | ✓ |

## HTTP 与解析链路

| 源码方法 | Go 函数 | 一致? |
|---|---|---|
| `_get_title` product info + title regex | `extractCourse` | ✓ |
| `_get_infos` chapter tree, fallback course detail | `fetchCourseChapterPayload`, `extractCourseLeaves` | ✓ |
| `_get_signature` leaf info | `resolveLeafSource`, `sourceFromLeafPayload` | ✓ |
| `_get_video_url` playurl quality source | `fetchPlayURLVariants` | ✓ |
| `Xuetang_Live._get_infos` live info + `video2ccsource` fallback | `extractLive`, `getLiveVideoURL` | ✓ |
| `Xuetang_Train._get_cid` training classroom lookup | `fetchTrainingClassroomID` | ✓ |

## JSON 字段映射

| Python key 链 | Go parser | 一致? |
|---|---|---|
| `data.content_data[].section_leaf_list[].leaf_list[]` | `extractCourseLeaves` | ✓ |
| `leaf_type in (0, 2, 3)` | `appendLeafIfPlayable` | ✓ |
| `data.leaf_data.content_info.media.ccid` and nested `ccid` aliases | `sourceFromLeafPayload`, `findFirstKey` | ✓ |
| `data.sources.quality10/quality20/...` | `fetchPlayURLVariants`, `qualityRank` | ✓ |
| `data.video[].quality/playurl` for live video source | `getLiveVideoURL` | ✓ |
| file, subtitle, HTML content nodes | `extractFiles`, `extractSubtitles`, `findHTMLText` | ✓ |

## 验证

- `go vet ./internal/extractor/xuetang/...`
- `go test -count=1 ./internal/extractor/xuetang/...`
- target source alignment helper: Xuetang `coverage=100%`, `missing=0`, `stubs=0`, `fabricated_hosts=0`
