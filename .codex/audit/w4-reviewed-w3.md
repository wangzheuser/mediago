# w4 reviewed w3 extractors

Reviewer: w4
Neighbor branch: `work/v2-batch1-w3`
Neighbor HEAD: `ba71fe5`
Sample method: `git diff --name-only 7128449..HEAD -- internal/extractor | ... | shuf -n 2`
Sampled sites: `kaimingzhixue`, `open163`
Verification: `go build ./...` passed in `medigo-w3`; `python3 scripts/verify_full_alignment.py` reports both sampled sites as `PASS (has HTTP+parse)` but the full run still lists unrelated remaining `STUB` sites.

## kaimingzhixue

### Issue 1: course file/material nodes from the Python source are not emitted by the Go extractor

- Source evidence:
  - `Mooc/Courses/Kaimingzhixue/Kaimingzhixue_Course.pyc.1shot.cdc.py:395-439` defines `_file_ext` and `_parse_file_info`, returning `file_fmt`, `file_url`, `file_name`, and `type: 'file'`.
  - `Kaimingzhixue_Course.pyc.1shot.cdc.py:491-543` appends VOD, live playback, and then `datum` / `files` entries through `_parse_file_info`.
  - `Kaimingzhixue_Course.pyc.1shot.cdc.py:578-613` feeds parsed nodes from the course `chapter` / `periods` tree into `self.infos`.
- w3 implementation evidence:
  - `internal/extractor/kaimingzhixue/kaimingzhixue.go:243-253` defines `kzxItem` fields only for video/live playback identifiers, with no file URL/name/format fields.
  - `internal/extractor/kaimingzhixue/kaimingzhixue.go:255-295` walks the tree but only appends `Kind: "video"` or `Kind: "live_playback"`.
  - `internal/extractor/kaimingzhixue/kaimingzhixue.go:97-115` returns only resolved playable VOD/live entries and reports `no playable VOD/live entries` when those are absent.
- Impact: courseware/material attachments that the Python downloader exposes as `file` resources are silently dropped, so a source-aligned course containing only files or files alongside videos is incomplete.
- Suggested fix: extend `kzxItem` with file metadata, mirror `_parse_file_info` for `datum` / `files`, normalize `file_url`, derive `file_fmt`, and append file entries to the returned playlist instead of filtering to VOD/live only.

### Issue 2: public `courseBasis` price/order-price path is declared but not executed

- Source evidence:
  - `Kaimingzhixue_Course.pyc.1shot.cdc.py:226-279` implements `_get_price` / `_get_order_price` by paging `public_course_url` with `page` and `limit`, then setting `price`, `purchased`, and title metadata.
  - `Kaimingzhixue_Course.pyc.1shot.cdc.py:33` defines `public_course_url = 'https://www.lckmzx.com/api/app/courseBasis'`.
- w3 implementation evidence:
  - `internal/extractor/kaimingzhixue/kaimingzhixue.go:34` declares `urlPublicCourse`, but `rg urlPublicCourse` shows no call site beyond the constant.
  - `internal/extractor/kaimingzhixue/kaimingzhixue.go:197-218` loads only `myStudy/{course_type}` course lists and never performs the source `courseBasis` paging fallback.
- Impact: returned metadata can miss the source downloader's public-course price/purchase/title enrichment, and `SOURCE_ALIGN.md` marks the constant as covered without a matching HTTP flow.
- Suggested fix: add a `fetchKaimingPublicCourseMeta` stage matching `_get_price`: GET `courseBasis?page=N&limit=20` up to the source max-pages/`last_page`, match `id == cid`, and merge `price`, `has_buy`, and title metadata.

## open163

### Issue 1: purchased-order list selection via `myOrders.do` is missing

- Source evidence:
  - `Mooc/Courses/Open163/Open163_App.pyc.1shot.cdc.py:33` defines `order_url = 'https://vip.open.163.com/open/trade/pc/pay/order/myOrders.do'`.
  - `Open163_App.pyc.1shot.cdc.py:186-239` implements `_get_course_list`: page through `myOrders.do`, keep paid orders with `status` 2, dedupe, and build selectable course records.
  - `Open163_App.pyc.1shot.cdc.py:259-282` uses `_select_my_course` to choose from that order list and populate `cid`, `course_uid`, title, price, and purchase state when a course id is not already resolved.
- w3 implementation evidence:
  - `internal/extractor/open163/open163.go:26` declares `urlMyOrders`, but `rg urlMyOrders internal/extractor/open163/open163.go` finds no call site beyond the declaration/comment.
  - `internal/extractor/open163/open163.go:65-69` immediately errors with `cannot parse open163 courseId from URL` when `parseOpen163CourseIDs` returns no `cid` / `courseUID`; there is no `myOrders.do` fallback or selectable purchased-course list.
  - `internal/extractor/open163/SOURCE_ALIGN.md:17-20` covers login, `getCourseInfo.do`, chapter parsing, and free-page fetch, but omits the `_get_course_list` / `_select_my_course` order-list flow.
- Impact: URLs or invocations that the Python source can resolve by selecting a purchased course from the authenticated account fail in the Go extractor unless the caller already supplies a concrete course id or course uid.
- Suggested fix: implement `fetchOpen163Orders` using `urlMyOrders` with `page` / `size`, filter `status` 2, normalize `productId` / `courseUid` / `productName`, and use it as the fallback when URL parsing does not provide ids.

### Issue 2: free-course URL/title extraction is less source-faithful than `Open163_Free`

- Source evidence:
  - `Mooc/Courses/Open163/Open163_Free.pyc.1shot.cdc.py:83-100` pairs titles from `/newview/movie/free?pid={pid}&mid=...` anchors with MP4 URLs from the page tail, then decodes each URL using `parse.unquote` plus `unicode_escape`.
- w3 implementation evidence:
  - `internal/extractor/open163/open163.go:107-121` uses the page `<title>` for every entry and extracts raw MP4 strings globally.
  - `internal/extractor/open163/open163.go:276-292` only returns raw HTTP strings or base64-decoded strings; it does not perform the source `unquote` / `unicode_escape` decoding.
- Impact: multi-video free courses can lose per-lesson names, and escaped page URLs can be returned in a non-source-equivalent form.
- Suggested fix: mirror the source regex pairing for `pid` / `mid` anchors, zip titles with MP4 URLs, and apply URL-unquote plus unicode-escape normalization before creating entries.
