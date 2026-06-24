# aishangke source alignment

Decompiler source: `~/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses/Aishangke/Aishangke_Course.pyc.1shot.cdc.py`.

| Source construct | Decompiled line(s) | Go implementation | Status |
| --- | ---: | --- | --- |
| Course detail API `https://loveshangke.com/course/index/getCourseDetailAjax?id={cid}` | 34, 348-363 | `courseDetailURL`, `fetchCourseDetail` in `aishangke.go:18,133-144` | Aligned |
| Series API `https://loveshangke.com/course/index/getMultipleSeriesCourseListAjax?pid={pid}&is_end={is_end}&page={page}&tid=0&sid=0` | 35, 366-391 | `seriesURL`, `fetchSeriesItems` in `aishangke.go:19,146-182` | Aligned |
| Enter course HTML `https://loveshangke.com/course/index/enterCourse?course_id={course_id}` | 36, 509-524 | `enterCourseURL`, `parseCCInfo` in `aishangke.go:20,226-242` | Aligned |
| Public course URL forms `/course/{cid}` and `/course/g{cid}` | 37-38 | `parseCourseID` regexes in `aishangke.go:21-22,97-104` | Aligned |
| `let ccInfo = {...}` regex and key/value extraction, URL-unquotes `viewertoken`-class fields | 1296-1373 | `ccInfoBlockRe`, `ccKeyValueRe`, `url.QueryUnescape` in `aishangke.go:36-37,232-240` | Aligned |
| CSSLCloud replay login/play endpoints | 39-40, 1374-1469 | Uses required shared helper `shared.CssLcloudResolvePlayInfo` in `aishangke.go:189-205`; source endpoint strings retained in `Extra` | Aligned by shared helper |
| CSSLCloud encrypted m3u8 key rewrite | playback helper chain | Uses required shared helper `shared.CssLcloudRewriteM3U8Keys` in `aishangke.go:244-255` | Aligned by shared helper |
| Output entries | `_get_infos` 1257-1295 | `Extract` returns top-level `MediaInfo{Entries: entries}` in `aishangke.go:49-95` | Aligned |

## Self-review

| Check | Result |
| --- | --- |
| Real HTTP calls present | `GetString` for login check, course detail, series pages, enter page, and optional m3u8 manifest. |
| Real parsing present | `json.Unmarshal` for API JSON and regex extraction for `ccInfo`. |
| Stub text absent | No stub sentinel phrases remain. |
| Csslcloud hard rule | `shared.CssLcloudResolvePlayInfo` is used; no inline platform login/play implementation. |
| Known deviation | Cookie validation uses a small Go-side member check before the source-aligned course flow; it does not replace any source endpoint parsing. |
