# w4 full audit: extractor source alignment and code review

Repo: `/home/sophomores/code/medigo`
Source root: `/home/sophomores/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses`
Scope: `lexueyun`, `lizhiweike`, `luffycity`, `magedu`, `mashibing`, `mddclass`, `med66`, `meeting`, `minshi`, `nmkjxy`, `open163`, `orangevip`, `plaso`, `qihang`, `qlchat`.

Checks performed:
- Source alignment: URL constants, HTTP method, request params/body, JSON keys/tags, auth/session flow, and source-only branches.
- Go code review: nil-panic risk, direct response-body close, unchecked errors, dead code/unused imports, and stub/fabricated behavior.
- Baseline verifier before this report: `go build ./...` passed, and `python3 scripts/verify_full_alignment.py` reported `PASS: 91`, `STUB: 0`, `NO_EXTRACT: 0`.

Severity note: `CRITICAL` is reserved for stub/fabricated-URL/nil-panic class defects or branches that are effectively dead despite being part of the declared extractor surface.

## lexueyun

### Issue 1: source datum/courseware files are parsed by Python but not emitted by Go
- Source evidence: `Lexueyun_Course.pyc.1shot.cdc.py:38` defines `/happyStudy/proxy/lexuesv/pc/getDatum`; `:619-635` implements `_get_datum`; `:765-806` normalizes file URLs; `:824`, `:863`, `:871-875` build `file_list`; `:1136-1199` downloads file entries.
- Go evidence: `internal/extractor/lexueyun/lexueyun.go:35` defines `datumPath`, but it has no call site; `:130-169` only appends lesson video entries; `:245-271` resolves one stream media entry per lesson.
- Impact: source-aligned video extraction can pass, but downloadable datum/courseware artifacts present in the Python source are missing from Go `MediaInfo.Entries`.

### Issue 2: unchecked read error in Sunlands thirdLogin response
- Go evidence: `internal/extractor/lexueyun/lexueyun.go:311-318` posts the Sunlands login request, closes the body, then uses `b, _ := io.ReadAll(resp.Body)`.
- Impact: a network/body read failure is hidden and later surfaces as a misleading JSON/session error.

## lizhiweike

### Issue 1: order-price `buy_record` flow is declared but not executed
- Source evidence: `Lizhiweike_Base.pyc.1shot.cdc.py:34` defines `order_url = https://api-v2.lizhiweike.com/user/v1/buy_record`; `:192-196` calls it from `_get_order_price`.
- Go evidence: `internal/extractor/lizhiweike/lizhiweike.go:19` declares `urlBuyRecord`, but `urlBuyRecord` has no call site in the package.
- Impact: media extraction may still work, but the purchase/order-price metadata branch in the source is not byte-aligned in Go.

## luffycity

### Issue 1: VIP-card course discovery path is missing
- Source evidence: `Luffycity_Course.pyc.1shot.cdc.py:234-249` implements `_append_vip_card_courses` by calling `/study/vip-card/` and `/study/vip-card/{id}/`; `:305` includes this branch in course-list assembly.
- Go evidence: `internal/extractor/luffycity/luffycity.go:58` accepts `vip` URLs, but the package has no `vip-card` request; current list/detail calls are degree/module paths such as `internal/extractor/luffycity/luffycity.go:240` and `:252`.
- Impact: VIP-card inventory can be missed even though the URL pattern suggests VIP support.

## magedu

NO ISSUE found in the requested checks. The Go extractor aligns with the inspected Python URL/method/auth/JSON flows, uses shared helpers where appropriate, and has no direct body-leak, nil-panic, unchecked direct-response read, or unused-import issue found by review plus `go build ./...`.

## mashibing

NO ISSUE found in the requested checks. Direct POST handling in `internal/extractor/mashibing/mashibing.go:296-305` closes the response body, checks status, and handles `io.ReadAll` errors; source URL/method/JSON/auth spot checks align with the Go implementation.

## mddclass

### Issue 1: seller, trade-order, and joined-company discovery branches are missing
- Source evidence: `Mddclass_Course.pyc.1shot.cdc.py:300` calls `/seller/user/device/seller_list`; `:355` calls `/user/my_order_list`; `:865` calls `/company/user/join_company_list`; `:897` tags joined-company courses with `_source = join_company_list`.
- Go evidence: `internal/extractor/mddclass/mddclass.go:346-496` only covers direct series/group discovery plus `user/my_group_list` and group-series APIs. There are no Go call sites for `seller_list`, `my_order_list`, or `join_company_list`.
- Impact: courses visible only through seller/order/company membership branches can be omitted.

## med66

NO ISSUE found in the requested checks. The Go implementation uses `shared.CssLcloudResolvePlayInfo` for the csslcloud replay path, closes direct redirect responses, and source URL/method/JSON/auth checks did not reveal a mismatch.

## meeting

NO ISSUE found in the requested checks. The package relies on shared HTTP helpers that close bodies; ignored errors are confined to fallback/probing branches rather than decisive parse branches, and no nil panic, dead import, or stub behavior was found.

## minshi

### Issue 1: material/file artifacts are collected as metadata only, not emitted as downloadable entries
- Source evidence: `Minshi_Course.pyc.1shot.cdc.py:32` defines `/api/learning/ext/class/material/list`; `:619-666` fetches and parses `fileInfoVOS`; `:736-770` appends materials into `file_list`; `:951-1012` downloads file entries.
- Go evidence: `internal/extractor/minshi/minshi.go:107` appends only video entries; `:112` places `collectFiles(...)` under `Extra["materials"]`; `:194-211` only returns maps.
- Impact: material URLs are discoverable in metadata but are not represented as first-class downloadable media/file entries like in the source.

## nmkjxy

### Issue 1: courseware/file artifacts are returned only in `Extra`, not as media entries
- Source evidence: `Nmkjxy_Course.pyc.1shot.cdc.py:33-34` defines courseware APIs; `:791-899` parses grouped and legacy courseware files; `:1126-1259` downloads those file lists.
- Go evidence: `internal/extractor/nmkjxy/nmkjxy.go:91-92` fetches courseware and stores it in `Extra`; `:188-200` returns raw courseware maps without appending downloadable file entries.
- Impact: video extraction can pass, but courseware downloads from the Python source are not exposed as extractor entries.

## open163

### Issue 1: `myOrders.do` purchased-course selection is missing
- Source evidence: `Open163_App.pyc.1shot.cdc.py:33` defines `https://study.163.com/p/myOrders.do`; `:186-239` pages purchased orders; `:259-282` selects a course from orders when no explicit course ID is supplied; `:584` invokes that fallback.
- Go evidence: `internal/extractor/open163/open163.go:26` declares `urlMyOrders`, but it has no call site; `:65-69` returns `cannot parse open163 courseId` when an ID cannot be parsed.
- Impact: authenticated purchased-course discovery from the source is absent in Go.

### Issue 2: free-course media URL/title normalization is weaker than source
- Source evidence: `Open163_Free.pyc.1shot.cdc.py:83-100` pairs each `mid` with its own title and decodes URLs via URL-unescape plus `unicode_escape`.
- Go evidence: `internal/extractor/open163/open163.go:107-121` uses the page title for entries and regexes raw media strings; `:276-292` only base64-decodes matching strings.
- Impact: free-course entries can carry less precise titles and may miss escaped media URLs that the source accepts.

## orangevip

### Issue 1: source cookie-validity check is not performed
- Source evidence: `Orangevip_Base.pyc.1shot.cdc.py:228-237` validates the cookie by calling `https://u.api.orangevip.com/Api/Index/getUserInfo`.
- Go evidence: `internal/extractor/orangevip/orangevip.go:52-60` only checks that a cookie string exists before fetching course data; the package has no `getUserInfo` call.
- Impact: auth failure can be reported later as a course/API parse failure instead of matching the source login-validation flow.

### Issue 2: file/courseware artifacts are stored only in `Extra`
- Source evidence: `Orangevip_Course.pyc.1shot.cdc.py:745-775` parses and downloads file entries.
- Go evidence: `internal/extractor/orangevip/orangevip.go:100` stores `files` in `Extra`; `:192-212` fetches file metadata but does not append file entries.
- Impact: downloadable file artifacts from the source are not exposed as media/file entries.

## plaso

### CRITICAL: local whiteboard/file playback URL is fabricated instead of derived from the source plist/STS flow
- Source evidence: `Plaso_Local.pyc.1shot.cdc.py:1569` derives a storage root from `location_path`; `:1585-1590` loads `info.plist`; `:1768-1787` builds media URLs as `https://filecdn.plaso.com/{root}/{location}/{media_path}`; `:2233-2306` and `:2424` consume the plist-derived media entries.
- Go evidence: `internal/extractor/plaso/plaso.go:198` directly constructs `https://filecdn.plaso.cn/liveclass/plaso/%s/video/1.mp4`, and `:244` constructs `https://file.plaso.cn/teaching/%s`.
- Impact: this is a guessed/fabricated direct URL path and is not byte-aligned with the Python source. Local/classroom recordings can resolve to wrong or nonexistent media.

### Issue 2: Aiwenyun/Jhpy Plaso variants from the source tree are not implemented
- Source evidence: `Aiwenyun_Course.pyc.1shot.cdc.py:39-55` and `Jhpy_Course.pyc.1shot.cdc.py:39-55` define variant hosts and API bases.
- Go evidence: `internal/extractor/plaso/plaso.go:17-37` only defines `www.plaso.cn`/`pclogin.plaso.cn` style hosts and patterns; the package has no `aiwenyun` or `jhpy` handling.
- Impact: source-covered Plaso-family domains are outside the Go extractor surface.

## qihang

### Issue 1: source file nodes are omitted from Go entries
- Source evidence: `Qihang_Course.pyc.1shot.cdc.py:278`, `:292`, `:310`, `:328`, `:346`, and `:364` parse file info from multiple node shapes; `:439-459` normalizes `file_url`, `file_name`, and `file_fmt`; `:617-804` downloads file lists.
- Go evidence: `internal/extractor/qihang/qihang.go:132-142` has node/resource structs without the source file fields; `:169-188` only handles video/live `StudyResourceType` values; `:81` returns those entries only.
- Impact: courseware/file nodes from the source are not emitted.

## qlchat

### CRITICAL: Qianliao train flow is effectively dead/missing in Go
- Source evidence: `Qlchat_Train.pyc.1shot.cdc.py:33-43` defines Qianliao train endpoints such as `/gate/course/myCourseList`, `/gate/learningCalendar/campData`, `/gate/order/getOrderListDerail`, and `/gate/course/playListOfCourse`; `:131`, `:211`, `:247`, `:286`, and `:292` call those endpoints. `Qlchat_Base.pyc.1shot.cdc.py:318-321` also validates Qianliao login via `/gate/user/getUserInfoById`.
- Go evidence: `internal/extractor/qlchat/qlchat.go:44-49` declares several `train_` constants, but they have no call sites; `:52` matches only `qlchat|qianliaoknow` and not `qianliao.net` or `xingqudao.cn`; `:68` always uses `https://m.qlchat.com` as referer.
- Impact: a whole source module is not wired into the Go extractor despite partial constants, so Qianliao train/course URLs cannot follow the Python source behavior.
