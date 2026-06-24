# classin 源码对齐对照

## URL 常量

| .cdc.py 行 | classin.go 行/名 | 一致? |
|---|---|---|
| `Classin_Config.pyc.1shot.cdc.py:46` `CLASSIN_TOKEN_API` | `classin.go:28` `urlM3u8Token` | ✓ |
| `Classin_Config.pyc.1shot.cdc.py:47` `CLASSIN_CDN_BASE` | `classin.go:32` `urlW0sCDN` | ✓ |
| `Classin_Config.pyc.1shot.cdc.py:48-50` record APIs | `classin.go:29-31` | ✓ |
| `Classin_Config.pyc.1shot.cdc.py:44-45,57` uid/key/UA | `classin.go:34-36` | ✓ |

## HTTP 调用

| 源码方法 | Go 函数 | method | 一致? |
|---|---|---|---|
| `Classin_Base._get_m3u8_token` `153-187` | `resolveM3U8Token` `149-171` | POST form | ✓ |
| `Classin_Course._post_classin_form_json` `680-688` | `postFormJSON` `136-147` | POST form + JSON | ✓ |
| `Classin_Course.resolve_live_activity_media` `1400-1467` | `requestRecordPayloads` `114-134` | POST `urlLessonInfo` | ✓ |
| `Classin_Course.resolve_record_activity_media` `1490-1561` | `requestRecordPayloads` `114-134` | POST `urlRecordGet` | ✓ |
| `Classin_Course._get_user_record_classes` `742-780` | `requestRecordPayloads` `114-134` | POST `urlUserRecords` | ✓ |

## JSON 字段映射

| 源码 key 链 | Go parse | 一致? |
|---|---|---|
| `error_info.errno`, `data.token` | `tokenResponse.ErrorInfo.Errno`, `Data.Token` | ✓ |
| `publicKey`, `fileUrl`, `timeStamp`, `x-eeo-*` | `resolveM3U8Token`, `classinSignHeaders` | ✓ |
| `video` JSON string list | `collectPlayables` nested `json.Unmarshal` | ✓ |
| `pm3u8/Pm3u8/m3u8/M3u8/pm3u8_path` | `collectPlayables` + `extractPM3U8Path` | ✓ |
| `Url/url/mp4_url/path` | `collectPlayables` + `playableFromString` | ✓ |

## 阻塞步骤

无。
