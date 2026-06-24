# youdao 源码对齐对照

Source: `~/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses/Youdao/`.
Encrypted/truncated sections were cross-checked with `~/code/xwz-downloader-source-release/decrypted_source/Youdao.py` and `decrypted_full/all_decrypted.json`.

## URL 常量

| .cdc.py 行 | youdao.go 行/名 | 一致? |
| --- | --- | --- |
| `Youdao_Base.py:32 referer = 'https://ydshengxue.com'` | `youdao.go:18 refererURL` | ✓ |
| `_check_cookie`: `https://ke.ydshengxue.com/api/user_status.jsonp` | `youdao.go:19 checkURL` | ✓ |
| `Youdao_Base.py:33 order_gaokao_url = 'https://ai.ydshengxue.com/ai-gw-sale/api/app/v2/order/my-orders'` | `youdao.go:20 orderGaokaoURL` | ✓ |
| `Youdao_Base.py:34 order_zhongkao_url = 'https://ec-server-c.ydlingshi.com/ai-gw-sale/api/app/v2/order/my-orders'` | `youdao.go:21 orderZhongkaoURL` | ✓ |
| `Youdao_Shengxue.py:37 course_gaokao_url = 'https://ai.ydshengxue.com/ai-product/api/app/v1/products/after-sale'` | `youdao.go:22 courseGaokaoURL` | ✓ |
| `Youdao_Shengxue.py:38 course_zhongkao_url = 'https://ec-server-c.ydlingshi.com/ai-product/api/app/v1/products/after-sale'` | `youdao.go:23 courseZhongkaoURL` | ✓ |
| `Youdao_Shengxue.py:39 info_gaokao_url = 'https://ai.ydshengxue.com/ai-product/api/app/v2/products/after-sale/{cid:}'` | `youdao.go:24 infoGaokaoURL` | ✓ |
| `Youdao_Shengxue.py:40 info_zhongkao_url = 'https://ec-server-c.ydlingshi.com/ai-product/api/app/v2/products/after-sale/{cid:}'` | `youdao.go:25 infoZhongkaoURL` | ✓ |
| `Youdao_Shengxue.py:41 key_url = 'https://live.ydshengxue.com/hikari-live/api/consumer/v1/key'` | `youdao.go:26 keyURL` | ✓ |

## HTTP 调用

| 源码方法 | Go 函数 | method | 一致? |
| --- | --- | --- | --- |
| `Youdao_Base._check_cookie` GET `user_status.jsonp` and regex `"success"\s*:\s*true` | `checkCookie` | GET | ✓ |
| `Youdao_Shengxue._get_infos` GET `info_gaokao_url/info_zhongkao_url` | `loadInfo` | GET | ✓ |
| `Youdao_Shengxue._get_m3u8_info` GET m3u8, then GET `key_url` | `rewriteM3U8IfNeeded` / `fetchKey` | GET | ✓ |

## JSON 字段映射

| 源码 key 链 | Go 解析 | 一致? |
| --- | --- | --- |
| `_get_order_price`: `data.orders[].id`, `productTotalPriceOfFen`, `productOriginPriceOfFen` | constants retained for order APIs; extractor does not need price to emit media entries | N/A |
| `_get_course_list`: `data[].title/name/id` | `courseGaokaoURL` / `courseZhongkaoURL` constants retained; direct URL extraction uses explicit URL cid | N/A |
| `_get_infos`: `data.videoPackageTab`, `questionPackage`, `skillPackage`, `servicePackage`, `videoLiveTab` | `collectVideos` walks the same keys | ✓ |
| `_parse_package_dict`: `subOutlines`, `videos`, `downloadUrl`, `id`, `cardPackageId`, `liveCenterId`, `clarityInfoList` | `collectVideos` extracts `downloadUrl`, `id`, `cardPackageId`, `liveCenterId`, descends into `subOutlines/videos/clarityInfoList` | ✓ |
| `_parse_service_dict`: `outlines`, `subOutlines`, `downloadUrl`, `id`, `cardPackageId`, `liveCenterId` | `collectVideos` descends into `outlines/subOutlines` and extracts same fields | ✓ |
| `_get_m3u8_info`: `url`, `cardPackageContentId`, `cardPackageId`, `cid`, `productId`, `liveId` | `fetchKey` sends the same query keys to `keyURL` | ✓ |

## 阻塞步骤

无. The Go extractor returns `Entries` for package/service videos and stores rewritten m3u8 text when encrypted key metadata is present.
