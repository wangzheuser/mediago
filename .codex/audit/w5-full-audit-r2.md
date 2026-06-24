# w5 full extractor audit r2

复核范围: `renrenjiang sanjieke shanxiang sier smartedu speiyou tmooc unipus wallstreets wangxiao wangxiao233 wendao wowtiku xiaoeapp xiaoetech`。

复核方法:
- 先比对当前 HEAD 对这些 extractor 路径的变更。
- 再重看第一轮报告里的 CRITICAL 站点。
- 最后补查 nil panic, response body leak, unchecked error。

当前结论:
- `git diff --name-status HEAD -- internal/extractor/{...}` 对这些目标路径无输出, 说明第一轮后没有相关 extractor 修复落到当前树。
- 第一轮 5 个 CRITICAL 仍然都在。

## renrenjiang

- Source alignment: `get_user_info` auth probe 仍未按源码单独复制, 但 cookie/token/referer 主链保持一致。
- Code review: 多处 `requestJSON` / `getPagedItems` 返回值被忽略 (`renrenjiang.go:56,65,73,107,113,164-165`), 属于 unchecked error, 但未见 nil panic 或 body leak。

## sanjieke

- Source alignment: `attachment/list` 分支仍只保留 URL 常量, 没有真正调用 `_get_attachment_list` (`sanjieke.go:29`; source `Sanjieke_Course.pyc.1shot.cdc.py:541-557`)。
- Code review: 未见新的 nil panic 或 response body leak。

## shanxiang

- Source alignment: 源码 `_check_cookie` 直接 GET `https://www.sx1211.com/user/course.html`, Go 仍只是把它当 Referer 去拉 study 页, 不是同一条显式探测链 (`shanxiang.go:89-90`; source `Shanxiang_Base.pyc.1shot.cdc.py:198-205`)。
- CSSLcloud 继续走 `shared.CssLcloudResolvePlayInfo`, 这一点仍是对的。

## sier

- Source alignment: `web/product/detail` 与 `web/course/getProductByCourseId` 两个源码分支仍未进入 Go (`sier.go:17-30`, `252-282`; source `Sier_Course.pyc.1shot.cdc.py:261-275`)。
- Code review: `fetchCourseList`, `extractNormalCourse`, `resolveVideo` 里还有多处 `_` 丢弃错误; 当前未见 nil panic 或 body leak。

## smartedu

- Source alignment: 源码三组 host 常量 `host_private / host_public / host_oversea` 仍未字节级保留, Go 仍只保留单个 `r1-ndr-private` 常量并做泛化替换 (`smartedu.go:18-24`, `helpers.go:219-225`; source `Smartedu_Base.pyc.1shot.cdc.py:71-73`)。
- Code review: `tchMaterialContent` 和 thematic 的两次 `getFirst` 仍然是 best-effort, 错误被忽略 (`smartedu.go:94,200-201`)。

## speiyou

- Code review: `subject_api` 探测返回值仍被忽略 (`speiyou.go:53`), 失败会延后暴露。
- 其余未见新的 nil panic 或 body leak。

## tmooc

NO ISSUE。

## unipus

- Code review: `joinCourse()` 里 `PostForm` / `GetString` 两个 side-effect 请求仍然丢弃错误 (`unipus.go:156-157`)。
- 其余未见 nil panic 或 body leak。

## wallstreets

- Source alignment: `_get_m3u8_text` 仍未落成 Go 里的实际 m3u8 重写, Go 只记录 `key_decode` 元数据并返回 playlist URL (`wallstreets.go:371-387`; source `Wallstreets_Course.pyc.1shot.cdc.py:350-366`)。
- Code review: `classroom_list_url`, `classroom_esbar_url`, `keyURL` 的读取仍有未检查 error (`wallstreets.go:197,200,382`)。

## wangxiao

- Source alignment: 讲义/下载页分支仍未进入 Go, `DownHandOut` / `down.aspx` 仅保留为常量, 没有落成独立文件条目 (`wangxiao.go:26-29,110-145`; source `Wangxiao_Course.pyc.1shot.cdc.py:1616-1626`)。
- Code review: 未见新的 nil panic 或 body leak。

## wangxiao233

- [CRITICAL] 仍未修复. 源码 Aliyun 路径仍包含 `getPlayInfoAndAuth` -> `decode playAuth` -> `https://vod.{region}.aliyuncs.com/?...Action=GetPlayInfo...AuthInfo=...` -> `https://mts.{region}.aliyuncs.com/?` license 链 (`Wangxiao233_Course.pyc.1shot.cdc.py:1277-1300,1545-1554,1646-1654`), Go 仍只在 `urlPlayAuth` 之后直接找 media URL (`wangxiao233.go:241-246`)。
- Code review: `apiPost()` 仍然忽略 `json.Marshal` 和 `io.ReadAll` 的 error (`wangxiao233.go:139,145`)。

## wendao

- Code review: `requestJSON()` 仍忽略 `json.Marshal` / `io.ReadAll` error (`wendao.go:146,153`)。
- 未见 nil panic 或 response body leak。

## wowtiku

- [CRITICAL] 仍未修复. 源码 Aliyun 播放链仍包含 private-rand helper, `PlayConfig={"EncryptType":"AliyunVoDEncryption"}`, 签名 `GetPlayInfo`, 以及 `GetLicense` POST (`Wowtiku_Course.pyc.1shot.cdc.py:251-281,1133-1157,1229-1252,1296-1316`), Go 仍只走简化版 STS `GetPlayInfo` (`wowtiku.go:205-229`)。
- Code review: 继续未见 body leak, 但 `subset` / `platform` 探测失败被直接吞掉。

## xiaoeapp

- [CRITICAL] 仍未修复. 源码 protected/private lookback 仍然需要 `/_alive/v3/get_lookback_list`, 私有 m3u8 解密, 片段/Key 重写, 以及 `/app/xe.vod.privatekey.get/1.0.0` (`Xiaoeapp_Course.pyc.1shot.cdc.py:1185,1277,1346-1370,1426-1508,1614-1639,1809-1811`), Go 仍只返回 `/app/alive/xe.alive.lookbackurl.get/1.0.0` 的首个 URL (`xiaoeapp.go:137-145`)。
- Code review: `postAppAPI()` 仍忽略 `io.ReadAll` error (`xiaoeapp.go:187-192`)。

## xiaoetech

- [CRITICAL] 仍未修复. 源码对 `text / ebook / file / audio / video / live / richtext iframe` 走多条 endpoint, Go 仍把多类资源统一压到 `infoURL = ...column.items.get` 一条线上, `textURL` / `fileURL` 依然未用 (`xiaoetech.go:24,33-34`; `helpers.go:83-85`; source `Xiaoetech_Course.pyc.1shot.cdc.py:52-58,1344-1353`)。
- [CRITICAL] 仍未修复. protected/private live 的 m3u8 归一化, segment 重写和私钥内联仍未实现, Go `liveMediaURL()` 只返回 JSON 里的第一个 URL (`helpers.go:123-142`; source `Xiaoetech_Course.pyc.1shot.cdc.py:979,1072,1141-1165,1212-1223,1252-1287`)。
- Code review: 暂未见新的 nil panic 或 body leak, 但 rich-text iframe audio/video 路径仍缺失。

## 总结

- 第一轮报告里的 5 个 CRITICAL 在当前树里都仍然存在, 没有看到其他 worker 的修复落地到这些 extractor 路径。
- 额外问题主要还是 unchecked error 和少量 source alignment 缺口, 没有发现新的 nil panic 或 response body leak 级别问题。
