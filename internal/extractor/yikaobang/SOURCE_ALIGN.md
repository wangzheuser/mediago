# yikaobang 源码对齐对照

Source: `~/code/xwz-downloader-source-release/decompiled_full/Mooc/Courses/Yikaobang/`.

## URL 常量

| .cdc.py 行 | yikaobang.go 行/名 | 一致? |
| --- | --- | --- |
| `Yikaobang_Base.py:29 referer = 'https://www.yikaobang.com.cn/'` | `yikaobang.go:11 refererURL` | ✓ |
| Source has no course/play API URL constants | `yikaobang.go:12 homeURL` probes the only documented site URL | BLOCKED |

## HTTP 调用

| 源码方法 | Go 函数 | method | 一致? |
| --- | --- | --- | --- |
| `Yikaobang_Course.prepare` calls `set_cookie` then prints missing API sample message | `Extract` probes `homeURL` then returns blocked | GET | BLOCKED |

## JSON 字段映射

| 源码 key 链 | Go struct/tag | 一致? |
| --- | --- | --- |
| None. `Yikaobang_Course.prepare` explicitly says reliable course/play API samples are missing and no pseudo-implementation is provided. | None. Go does not fabricate JSON paths or streams. | BLOCKED |

## 阻塞步骤

BLOCKED: upstream source contains the explicit marker `医考帮当前仍缺少可靠的课程/播放接口样本，已保留统一结构骨架，暂不提供伪实现。`. `Extract()` returns `blocked: needs upstream API samples ...` after a home-page liveness probe.
