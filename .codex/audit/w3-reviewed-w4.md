# w3 audit of w4 extractors

Auditor: worker-3  
Target worktree: `/home/sophomores/code/medigo-w4`  
Reviewed sites: `qihang`, `meeting`  
Source baseline: `/home/sophomores/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses/`

## Summary

| site | result | notes |
|---|---|---|
| `qihang` | no material blocker found | URL/API constants, GET endpoints, BokeCC, and CSSLcloud helper usage line up with decompiled source. |
| `meeting` | blocker found | w4 sends several Tencent Meeting API calls as form posts, but the Python source calls `request_json`, which posts a JSON body. `SOURCE_ALIGN.md` currently labels these as POST form and should be corrected. |

## qihang

Checked files:

- Source: `Qihang/Qihang_Course.pyc.1shot.cdc.py`, `Qihang/Qihang_Base.pyc.1shot.cdc.py`, `Mooc_Config.pyc.1shot.cdc.py`
- Go: `internal/extractor/qihang/qihang.go`
- Review doc: `internal/extractor/qihang/SOURCE_ALIGN.md`

Findings:

- URL regex coverage matches the source families from `Mooc_Config`: `/learn/<id>`, `/record/.../<id>`, `/playback/.../<id>`, and `courseId=<id>`.
- HTTP constants match the source: `course-list`, `course/catalog`, `lecture/curriculum/node`, `product`, `live/replay`, BokeCC `getvideofile`, and CSSLcloud replay endpoints.
- Go uses `shared.BokeCCResolve` and `shared.CssLcloudResolvePlayInfo`; this matches the helper requirement and avoids inlining CSSLcloud flow.
- JSON field mapping for course list, catalog nodes, resource `vid/resourceId`, and live `replayUrl` matches the source.
- Non-blocking note: the Python source contains file/courseware handling via `lectureUrl`/`_parse_file_info`; w4 currently returns playable media only. If the extractor contract later requires courseware artifacts, this should be added, but I did not count it as a blocker for the current media extractor scope.

## meeting

Checked files:

- Source: `Meeting/Meeting_Course.pyc.1shot.cdc.py`, `Meeting/Meeting_Config.pyc.1shot.cdc.py`, `Courses/Course_Others.pyc.1shot.das`, `Component/Mooc_Request.pyc.1shot.cdc.py`
- Go: `internal/extractor/meeting/meeting.go`
- Review doc: `internal/extractor/meeting/SOURCE_ALIGN.md`

### Blocker: API POST encoding does not match source

Evidence:

- Source `Component/Mooc_Request.pyc.1shot.cdc.py` defines `request_json(url, data, headers)` with `requests.post(url, json=data, headers=headers, verify=False)`.
- Source `Course_Others.pyc.1shot.das` uses `request_json` for Tencent Meeting API calls, including:
  - `liveportal/v2/query_live_stream`
  - `liveportal/v2/query_meeting_room_live_replay_info`
  - `meetlog/public/detail/common-record-info?c_instance_id=5`
- w4 `meeting.go` uses `c.PostForm(...)` for those same APIs and for `record-info`/`shareSignPostURL`, which sends `application/x-www-form-urlencoded`.
- `internal/extractor/meeting/SOURCE_ALIGN.md` says these are `POST form`, but that contradicts the decompiled `request_json` implementation.

Impact:

- Tencent Meeting endpoints that expect JSON bodies can reject or ignore form-encoded requests, causing `meeting` to return no media URLs even though URL parsing and JSON walking are otherwise present.

Recommended fix for w4:

- Add or reuse a JSON POST helper that sends `Content-Type: application/json` with `json.Marshal(data)`.
- Replace `PostForm` with JSON POST for the `request_json`-aligned Meeting endpoints.
- Update `internal/extractor/meeting/SOURCE_ALIGN.md` from `POST form` to `POST JSON` for these rows.

### Secondary differences

- Source `Meeting_Course.set_cookie` allows missing cookies with a warning; w4 hard-fails when `opts.Cookies == nil`. If public share URLs are expected to work without login, this should be relaxed.
- Source supports batch sentinel/text ingestion (`腾讯会议批量`, `meeting_batch`, `meeting course`) via `Meeting_Config.parse_meeting_batch_text`; w4 is a single-URL extractor. This may be acceptable for the Go extractor API, but it is a documented feature gap versus the Python source.
