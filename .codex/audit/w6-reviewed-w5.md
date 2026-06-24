# w6 review of w5

抽查站点: `lexueyun`, `wangxiao`.

## 发现 1: lexueyun 漏掉源码里的资料 datum 链
- 证据: 源码 `Lexueyun_Course._get_datum()` 会额外拉 `/happyStudy/proxy/lexuesv/pc/getDatum`, 再写入 `_source_info` 的 `file_list`。见 `Lexueyun_Course.pyc.1shot.cdc.py:619-635, 868-877`。
- Go 现状: `internal/extractor/lexueyun/lexueyun.go:230-271` 只拉 `userInfo`, `myMerchantList`, `getOrdersByMerchant`, `getSubjectDetail`, `getLessonsBySubject`, `getPlayUrl`, `getPlayUrl` 和 `thirdLogin`, 没有调用 `datumPath`，也没有生成对应资料条目。
- 建议: 补 `getDatum` 请求和 `file_list`/资料 Entries 输出, 对齐源码的 `_source_info` 分支。

## 发现 2: wangxiao 没覆盖源码里的讲义/下载页分支
- 证据: 源码 `_get_lesson_payload()` 会从 `Index.aspx` / `play` 页抓 `DownHandOut` / `down.aspx` 链接, 再在 `_get_infos()` 里把 `handout_url`、`file_url`、`file_html` 等资料节点纳入目录。见 `Wangxiao_Course.pyc.1shot.cdc.py:1580-1627, 1379-1459`。
- Go 现状: `internal/extractor/wangxiao/wangxiao.go:110-145` 只走 BokeCC 视频链, `parseLessonRefs()` 只收集可播放 lesson URL, 没有把讲义/附件页解析成独立文件条目。
- 建议: 增加 handout/download 页抓取与文件条目输出, 至少覆盖 `DownHandOut` 和 `down.aspx` 两类链接。

## 结论
- 两个站点都已做到真实 HTTP + JSON/页面解析, 但都还有与源码一致性相关的资料分支缺口。
