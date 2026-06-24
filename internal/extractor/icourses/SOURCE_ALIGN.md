# icourses 源码对齐对照

## URL / API 常量

| .cdc.py 行 | Go 行/名 | 一致? |
|---|---|---|
| Icourse_Base.py:43 `referer = 'https://www.icourses.cn'` | `icourses.go:16` `referer` | ✓ |
| Icourse_Base.py:44 `api_root = 'https://www.icourses.cn/prod/icourse-portal-api'` | `icourses.go:17` `api_root` | ✓ |
| Icourse_Base.py:45 `user_info_url = api_root + '/userCenter/userinfo'` | `icourses.go:18` `user_info_url` | ✓ |
| Icourse_Cuoc.py:26 `detail_api = '/course/getCourseDetailByVideo'` | `icourses.go:22` `cuoc_detail_api` | ✓ |
| Icourse_Cuoc.py:27 `resource_api = '/course/getCourseResByVideo'` | `icourses.go:23` `cuoc_resource_api` | ✓ |
| Icourse_Mooc.py:27-31 share/chapter/resource/sub API paths | `icourses.go:25-29` `mooc_*_api` | ✓ |
| Icourse_Mooc.py:32 `course_doc_apis` five document APIs | `icourses.go:32-38` `moocCourseDocAPIs` | ✓ |
| Mooc_Config.py:299 `Icourse_Mooc` URL regex | `icourses.go:41-42`, `icourses.go:46-47` | ✓ |
| Mooc_Config.py:300 `Icourse_Cuoc` URL regex | `icourses.go:43`, `icourses.go:46-47` | ✓ |

## 认证与 Header

| 源码方法/常量 | Go 函数/行 | 一致? |
|---|---|---|
| Icourse_Base.py:46-50 pseudo cookie names | `api.go:15-20` | ✓ |
| Icourse_Base._parse_cookie_string line 226 and `_cookie_header_from_dict` line 244, skip pseudo cookies | `api.go:76-102`, `icourses.go:140-153` | ✓ |
| Icourse_Base._check_cookie line 122 token aliases `icourses_website_user_token` / `icourses_token` | `icourses.go:143-151` | ✓ |
| Icourse_Base._build_header line 159: Referer, JSON Accept, Cookie, Bearer token | `icourses.go:146-152`, `api.go:22-43` | ✓ |
| Icourse_Base._api_get line 182: build `api_root + path`, URL encode params, GET | `api.go:22-43` | ✓ |
| Icourse_Base._api_get line 199: JSON parse; line 204-220 code/success/data checks | `api.go:46-74` | ✓ |

## HTTP 调用

| 源码方法 (line) | Go 函数 (line) | method | 一致? |
|---|---|---|---|
| Icourse_Cuoc._get_title line 56: `detail_api` with `courseId` | `icourses.go:179-194` | GET JSON | ✓ |
| Icourse_Cuoc._get_infos line 72: `resource_api` with `courseId` | `resources.go:8-25` | GET JSON | ✓ |
| Icourse_Mooc._get_title line 62: `detail_api` with `courseId` | `icourses.go:179-194` | GET JSON | ✓ |
| Icourse_Mooc._get_infos line 308: `chapter_api` with `courseId` | `resources.go:27-60` | GET JSON | ✓ |
| Icourse_Mooc._get_chapter_resources line 134: `chapter_res_api` with `courseId/chapterId` | `resources.go:62-80` | GET JSON | ✓ |
| Icourse_Mooc._expand_share_resources line 100: `share_sub_api` with `resId` fallback | `resources.go:153-171` | GET JSON | ✓ |
| Icourse_Mooc._get_other_resources line 240: `other_res_api` paged records | `resources.go:102-126` | GET JSON | ✓ |
| Icourse_Mooc._get_course_doc_resources line 272: five course doc APIs | `resources.go:128-151` | GET JSON | ✓ |

## JSON 字段映射

| 源码 key 链 / regex | Go parser | 一致? |
|---|---|---|
| Cuoc URL groups `cid1/cid2/cid3`; Mooc groups `cid1..cid4` | `icourses.go:156-177` | ✓ |
| API root `code`, `msg/message`, `success`, `data` | `api.go:46-74` | ✓ |
| `_pick_list` keys `list/records/items/rows/chapterList/resourcesList/courseSubList/data` | `api.go:104-121` | ✓ |
| title fields `courseName`, `schoolName`, `teacherName` | `icourses.go:188-193`, `api.go:123-140` | ✓ |
| resource name keys `resName/name/title/fileName`, default, `resId` | `resources.go:173-181`, `api.go:142-158` | ✓ |
| media type keys `resMediaType/mediaType/type`; size keys `fileSize/resSize/size` | `resources.go:179-180`, `api.go:230-280` | ✓ |
| URL keys `resUrl`, `url`, `pptResUrl`; wrapper query `src` | `resources.go:184-205`, `api.go:160-176` | ✓ |
| extension/kind mapping for video/pdf/ppt/doc/attach | `api.go:178-228` | ✓ |
| Cuoc resource list keys `resourcesList/list/records`; filter `kind == video` | `resources.go:13-24` | ✓ |
| Mooc chapter list keys `chapterList/list/records`, chapter `chapterId/id`, title `chapterName/name/title` | `resources.go:32-46` | ✓ |
| Mooc unit recursion `children`, path joined by ` - ` | `resources.go:82-100` | ✓ |
| Mooc preview fallback `previewList` -> `课程试看` | `resources.go:47-53` | ✓ |
| Other resource `category == 习题作业` -> papers, else sources | `resources.go:102-126` | ✓ |
| Course doc API string/list/map handling with default `resName` | `resources.go:128-151` | ✓ |
| De-dupe tuple `(url, kind, name)` | `resources.go:208-220` | ✓ |

## 返回结构

| 源码行为 | Go 实现 | 一致? |
|---|---|---|
| Cuoc `_download_resource_list` downloads video resources only | `resources.go:8-25`, `media.go:11-23` | ✓ |
| Mooc `_download` groups chapters, units, papers, sources | `media.go:26-56` | ✓ |
| `_format_resource_name`: video `[prefix+index]--name`, non-video `(prefix+index)--name` | `media.go:103-109` | ✓ |
| `_download_resource_item`: m3u8/mp4/pdf/ppt/doc/attach by kind/ext | `media.go:66-93`, `api.go:178-228` | ✓ |
| Download headers carry Icourse referer/cookie | `media.go:95-101` | ✓ |

## 阻塞步骤

无。
