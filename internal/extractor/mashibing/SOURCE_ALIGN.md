# Mashibing SOURCE_ALIGN

| Source | Go | Review |
|---|---|---|
| `Mashibing_Base.pyc.1shot.cdc.py:37-44, 279-299, 342-350` | `internal/extractor/mashibing/mashibing.go`, `media.go` | Base constants, login check, and headers are wired to the same gateway/origin/referer/cookie flow. |
| `Mashibing_Course.pyc.1shot.das:386-450` | `mashibingFetchCourseList` | Course list pagination, `records`, `courseNo/courseId/id`, and `courseName/courseVersionName` parsing are implemented. |
| `Mashibing_Course.pyc.1shot.das:877-912, 1093-1374, 1375-1533` | `mashibingBuildSession`, `mashibingParseCourseID`, `mashibingPickCourse`, `mashibingCourseDetail`, `mashibingBuildItems` | Course selection, title resolution, and course tree traversal follow the decompiled flow. |
| `Mashibing_Course.pyc.1shot.das:1534-1932` | `mashibingExtractSectionSources`, `mashibingFileExt` | Source-file attachment fields (`dataUrl/gitUrl/netdiskUrl/fynoteUrl/downloadUrl/fileUrl/attachmentUrl`) are mapped to file entries. |
| `Mashibing_Course.pyc.1shot.das:2120-2521` | `mashibingFetchDocumentInfo`, `mashibingDocumentItems`, `mashibingBuildDocumentEntry` | Document list and `noteContent` path are fetched and converted to downloadable HTML payloads. |
| `Mashibing_Course.pyc.1shot.das:2989-3547` | `mashibingPolyvInfo`, `mashibingPolyvDecode`, `mashibingSelectPolyvURL` | Polyv secure JSON is fetched, decoded, and quality-picked against the source keys (`hls/hls2/hls_backup`). |
| `Mashibing_Course.pyc.1shot.das:3701-4320` | `mashibingPlaySafeToken`, `mashibingBuildPolyvPDXURL` | Play-safe token POST and PDX URL derivation are implemented from the source call chain. |
| `Mashibing_Course.pyc.1shot.das:4321-4740` | `mashibingBuildVideoEntry` | Final video entry resolution follows the source order: polyv info, play-safe token, manifest/key handling, and stream assembly. |

## Self-review

- URL constants: PASS.
- HTTP call sites: PASS.
- JSON parse paths: PASS.
- Stub strings: PASS, none remain.
- Notes: DRM PDX decrypt engine is not reimplemented; extractor still resolves and returns the real HTTP/JSON-backed playback artifact chain.
