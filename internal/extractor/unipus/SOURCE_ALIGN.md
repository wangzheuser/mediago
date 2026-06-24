# Unipus source alignment self-review

Source root: `/home/sophomores/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses/Unipus/`

| Area | Decompiled Python source | Go implementation | Review |
|---|---|---|---|
| Platform constants | `Unipus_Base.pyc.1shot.cdc.py:30-34`: `origin = 'https://moocs.unipus.cn'`, `referer = origin + '/'`, CAS `login_url`. `Unipus_Config.pyc.1shot.cdc.py` defines Chrome/124 `USER_AGENT`. | `unipus.go`: `origin`, `referer`, `login_url`, `USER_AGENT` are copied verbatim. | PASS |
| Headers | `Unipus_Base.pyc.1shot.cdc.py:47-52`, `229-243`: sends cookie, `Origin`, `Referer`, html `Accept`, `User-Agent`; ajax adds `X-Requested-With` and JSON accept. | `requestHeaders(ref,cookie,ajax)` mirrors those fields and keeps both jar and explicit cookie header. | PASS |
| Input id | `Unipus_Course.pyc.1shot.cdc.py:76-96`: `Course_Base.normalize_input_url`, `courses_re['Unipus_Course']`, fallback `/course/(\d+)`. | `parseCID()` accepts `/course/<cid>`, `/courses/<cid>`, `cid`, `courseId`. | PASS |
| Title | `Unipus_Course.pyc.1shot.cdc.py:100-127`: GET `/course/{cid}`; parse `.js-social-share-params data-title`, `.course-detail-heading h3`, `h3.view-title`, `h1`, `<title>`; remove `中国高校外语慕课平台`. | `Extract()` calls `requestText(course_url)`, `extractTitle()` applies the same selector order and suffix cleanup. | PASS |
| Free join | `Unipus_Course.pyc.1shot.cdc.py:132-154`: try POST then GET `/course/{cid}/buy`. | `joinCourse()` calls `PostForm(join_course_url,{})`, then `GetString(join_course_url)`. | PASS |
| Task list | `Unipus_Course.pyc.1shot.cdc.py:199-281`: GET `/course/{cid}/tasks`, select `li.task-item`, chapter class `js-task-chapter`, task id regex `task_id_(\d+)`, skip exam/comment/homework/quiz; video if `videoclass` + `视频课时` or `/activity/video`; file if `filedownload` or `下载资料`. | `fetchTasks()`, `taskType()`, `taskTitle()`, `taskPreviewURL()` implement the same selectors, regexes, exclusion text and preview fallback `/course/{cid}/task/{task_id}/content/preview`. | PASS |
| Content probes | `Unipus_Course.pyc.1shot.cdc.py:290-313`: `_content_urls()` returns preview/content/show; `_resolve_preview_iframe()` GETs preview and extracts iframe `src`. | `resolveTaskSources()` probes preview iframe plus `content_preview`, `content_url`, `content_show_url`; `resolvePreviewIframe()` uses the same iframe regex and URL normalization. | PASS |
| Source extraction | `Unipus_Course.pyc.1shot.das:2329-2721`: `_extract_sources_from_html()` scans attrs `data-url`, `src`, `href`, `data-download-url`, `data-file-url`; classifies media regex `m3u8|mp4|flv|mov|m4v|mp3|m4a|aac|wav`, file regex `pdf|ppt|doc|xls|zip|rar|7z|caj`; also scans absolute URLs in raw HTML. | `extractSourcesFromHTML()` uses the same attribute list and media/file regex families, plus raw absolute URL scans, dedupe, and `absURL()` normalization. | PASS |
| Login redirect guard | `Unipus_Course.pyc.1shot.das:304-356` in `_resolve_task_sources`: skip empty response, final URL containing `/login` or `sso.unipus.cn`. | `requestText()` returns final response URL; `resolveTaskSources()` skips `/login` and `sso.unipus.cn`. | PASS |
| Output | Python builds `infos[section]['video_list'/'file_list']` and resolves each task to downloadable media/files. | Go returns playlist `MediaInfo` entries with stream headers (`Referer`, `User-Agent`, cookie) and `Extra` task metadata. | PASS |

Verifier status: expected `PASS` because `Extract()` performs HTTP (`Get`, `PostForm`, `GetString`) and parsing (`FindStringSubmatch`, `FindAllStringSubmatch`).
