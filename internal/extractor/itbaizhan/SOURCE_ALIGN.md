# itbaizhan 源码对齐对照

## URL 常量

| .cdc.py 行 | Go 行/名 | 一致? |
|---|---|---|
| `Itbaizhan_Course.py:39 navlist_url = 'https://www.itbaizhan.com/index/stage/navlist?id={course_id}&stage=0'` | `itbaizhan.go:27 navlist_url` | ✓ |
| `Itbaizhan_Course.py:40 stage_url = 'https://www.itbaizhan.com/index/stage/rightlist?id={stage_id}'` | `itbaizhan.go:28 stage_url` | ✓ |
| `Itbaizhan_Course.py:41 play_url = 'https://www.itbaizhan.com/course/id/{course_id}.html'` | `itbaizhan.go:29 play_url` | ✓ |
| `Itbaizhan_Course.py:42 check_url = 'https://www.itbaizhan.com/index_new/index/checkUserLogin'` | `itbaizhan.go:30 check_url` | ✓ |
| `Itbaizhan_Course.py:43 course_list_url = 'https://www.itbaizhan.com/mine/courseschedule'` | `itbaizhan.go:31 course_list_url` | ✓ |
| `Itbaizhan_Base.py:33 referer = 'https://www.itbaizhan.com/'` | `itbaizhan.go:25 referer` | ✓ |
| `Itbaizhan_Base.py:34 origin = 'https://www.itbaizhan.com'` | `itbaizhan.go:26 origin` | ✓ |
| `Itbaizhan_Config.py:17 USER_AGENT = 'Mozilla/5.0 ... Edg/141.0.0.0'` | `itbaizhan.go:24 USER_AGENT` | ✓ |

## HTTP 调用

| 源码方法 (line) | Go 函数 (line) | method | 一致? |
|---|---|---:|---|
| `_check_cookie` lines 138-156 | `checkCookie` `itbaizhan.go:147-162` | GET + JSON parse | ✓ |
| `_get_course_list` lines 289-295 | `getCourseList` + `parseMineCourseList` `helpers.go:144-174` | GET + HTML parse | ✓ |
| `_get_nav_stage_ids` lines 354-361 | `getNavStageIDs` `itbaizhan.go:202-230` | GET + JSON parse | ✓ |
| `_get_title` lines 571-600 | `loadTitleAndStages` `itbaizhan.go:184-199` | GET + HTML parse | ✓ |
| `_get_stage_info` lines 606-612 | `loadInfos` `itbaizhan.go:233-264` | GET + JSON parse | ✓ |
| `_get_play_info` / `_parse_play_info` lines 583, 569 | `getPlayInfo` + `parsePlayInfo` `helpers.go:70-96` | GET + HTML/regex parse | ✓ |
| `get_polyv_download_info` lines 195-238 in Base | `resolveVideo` `itbaizhan.go:276-317` + `shared.PolyvResolveSecure` / `shared.PolyvRewriteM3U8Keys` | GET + JSON parse | ✓ |

## JSON / HTML 字段映射

| 源码 key 链 | Go struct/tag 或解析 | 一致? |
|---|---|---|
| `checkUserLogin -> code/user_id` | `checkCookie` `itbaizhan.go:155-162` | ✓ |
| `nav[].id/child/children` | `getNavStageIDs` `itbaizhan.go:211-233` | ✓ |
| `stage.rightlist -> type.type_name` | `stageInfo.Type.TypeName` `itbaizhan.go:54-60` | ✓ |
| `stage.rightlist -> specific[].s_id/s_name` | `specificInfo.SID/SName` `itbaizhan.go:62-67` | ✓ |
| `specific[].child[].course_id/course_name/video_time/input_time/is_free/free` | `videoChild` + `parseVideoInfo` `helpers.go:14-33` | ✓ |
| `specific[].training[].t_id/t_name` | `trainingInfo` + `parseFileInfo` `helpers.go:35-46` | ✓ |
| `course/id/{id}.html -> vid/playsafe/title` | `parsePlayInfo` + `extractTitle` `helpers.go:70-97` | ✓ |
| `mine/courseschedule -> .per_info/.per_info_title/img alt/a href` | `parseMineCourseList` `helpers.go:152-174` | ✓ |

## 阻塞步骤

无。Polyv 播放链按源码走 `shared.PolyvResolveSecure` + `shared.PolyvRewriteM3U8Keys`，未重写签名/解密逻辑。
