# minshi 源码对齐对照

## URL 常量

| .cdc.py 行 | minshi.go 行/名 | 一致? |
|---|---|---|
| Minshi_Base.py:39 origin = 'https://vip.minshiedu.com' | minshi.go:17 origin | ✓ |
| Minshi_Base.py:40 referer = 'https://vip.minshiedu.com/#/course/courseHome' | minshi.go:18 referer | ✓ |
| Minshi_Base.py:43 platform_proxy = 'am9pbmVhc3QtYXBw' | minshi.go:19 platform_proxy | ✓ |
| Minshi_Base.py:44 system_id = '82' | minshi.go:20 system_id | ✓ |
| Minshi_Course.py:28 course_list_api = '/api/learning/ext/course/my' + api_host | minshi.go:22 course_list_api | ✓ |
| Minshi_Course.py:30 course_info_api = '/api/learning/ext/courseDetails/new/courseTableInfo/{cid}' | minshi.go:24 course_info_api | ✓ |
| Minshi_Course.py:31 course_detail_api = '/api/learning/ext/courseDetails/new/courseTableDetail/{course_table_id}' | minshi.go:25 course_detail_api | ✓ |
| Minshi_Course.py:32 material_api = '/api/learning/ext/class/material/list' | minshi.go:26 material_api | ✓ |
| Minshi_Course.py:33 video_encrypted_api = '/api/learning/ext/course/videoEncryptedInfo/{target_id}' | minshi.go:27 video_encrypted_api | ✓ |
| Minshi_Base.py:45 polyv_secure_url = 'https://player.polyv.net/secure/{vid}.json' | minshi.go:28 polyv_secure_url + polyv.go native `getPolyvM3U8`, fallback shared.PolyvResolveSecure | ✓ |

## HTTP 调用

| 源码方法 (line) | Go 函数 (line) | method | 一致? |
|---|---|---|---|
| Minshi_Course._get_course_list line 195 -> _request_api_data('POST', course_list_api) | Extract line 63 | POST | ✓ |
| Minshi_Course._get_course_info line 399 -> _request_api_data('GET', course_info_api) | Extract line 70 | GET | ✓ |
| Minshi_Course._get_course_table_detail line 418 -> _request_api_data('GET', course_detail_api) | Extract lines 89-94 | GET | ✓ |
| Minshi_Course._get_material_list line 619 -> _request_api_data('POST', material_api) | collectFiles lines 194-215 | POST | ✓ |
| Minshi_Course._get_play_token line 792 -> _request_json('GET', video_encrypted_api) | getPlayToken lines 117-133 | GET | ✓ |
| Minshi_Base.get_polyv_m3u8 line 937 -> request_get_raw(polyv_secure_url) | `resolvePolyv` + `polyv.go:getPolyvM3U8` | GET secure + GET m3u8/key | ✓ |

## JSON 字段映射

| 源码 key 链 | Go parser/tag | 一致? |
|---|---|---|
| response.get('data') | apiResp.Data `json:"data"` lines 44-48 | ✓ |
| course node get('courseTableId'), get('id') | collectLessons lines 182-191 | ✓ |
| course node get('videoId'), get('vid') | collectLessons lines 182-191; getPlayToken line 126 | ✓ |
| title/name/courseName/catalogueName/catalogName/chapterName/tableName | collectLessons line 188; Extract line 75 | ✓ |
| videoEncryptedInfo get('playsafe') / get('playSafe') | getPlayToken line 126 | ✓ |
| material get('path'/'filePath'/'url'/'fileUrl'/'downloadUrl') | collectFiles lines 205-211 | ✓ |

## 阻塞步骤

无. Polyv 播放链先按 Minshi 源码实现 playsafe token key URL, secure body AES 解密, m3u8 绝对化和 key inline; 不可用时 fallback 到 `shared.PolyvResolveSecure` / `shared.PolyvPickBestManifest`.
