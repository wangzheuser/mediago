# logic-w3-cross: extractor 逻辑流程交叉审计

范围: `bilibili douyin cctv chaoxing enetedu eoffcn gaotu houda htknow itbaizhan lizhiweike`. 备注: 任务文字写 10 站, 实际清单为 11 站.

审计边界: 重点判定认证, API 调用顺序, 签名/加密/解密, JSON 字段链, 以及媒体/资源 URL 获取方式. 纯价格元数据只在影响课程选择或资源输出时计为关键步骤.

## bilibili

### Python 流程 (5 步)
1. `Bilibili_Base._check_cookie` 读取 `SESSDATA`, 调 `https://api.bilibili.com/x/web-interface/nav`, 要求 `data.isLogin == true`.
2. `Bilibili_Course` 的 PUGV 课程流先用 `pugv/pay/web/my/paid?ps=10&pn={page}` 做已购课列表/回退选择, 再按 `ss` 或 `ep` 进入 `pugv/view/web/season/v2?season_id=` 或 `pugv/view/web/season?ep_id=`.
3. 课程详情从 `data.title`, `data.payment.price`, `data.user_status.payed`, `data.sections[].episodes[]` 建立课节列表.
4. 每节课调 `pugv/player/web/playurl?fnval=16&fourk=1&ep_id=`, 检查 `data.drm_type`, 解析 `data.dash.video/audio`; dash 不可用时走 `data.durl`/mp4 fallback, 下载时合并音视频.
5. `Bilibili_Gongfang` 另有商城/工房流: `mall-c/order/detail` -> `mall-c/ship/orderdetails/query` -> `mall-c/ship/orderdetails/querydownloadurl`, 以 `shipOrderDetailsId` 获取文件下载 URL.

### Go 实现
1. `bilibili.go` 实现公开 BV/av/b23 链: `x/web-interface/view` -> `x/player/playurl`, 解析 `dash.video/audio` 和 `durl`.
2. `cheese.go` 实现直接 `ss`/`ep` 的 PUGV 流: `season/v2` 或 `season?ep_id=` -> `player/web/playurl`, 解析 dash video/audio.
3. `cheesePaidList` 只声明, 没有按 Python 的已购列表分页/选择路径执行.
4. 未实现 `drm_type` 早退和 Python 的 mp4 fallback 细节.
5. 未实现 `Bilibili_Gongfang` 的 mall/gongfang 订单文件下载链.

### 判定
- MISSING_STEP: PUGV 已购列表选择, `drm_type`/mp4 fallback 细节, 以及 `Bilibili_Gongfang` 商城文件链未实现. 公开视频分支和直接 cheese `ss/ep` 分支本身存在, 但不是完整 Python 覆盖.

## douyin

### Python 流程 (4 步)
1. Douyin 不在独立 `Courses/Douyin/`, 而在 `Course_Others` 分支; URL 正则来自 `Mooc_Config.Course_Others`.
2. `prepare` 处理 `modal_id`, 跟随短链/原链跳转, 把视频链接规范化为 `https://www.iesdouyin.com/share/video/{id}`, 并设置 `_douyin_cid/_douyin_mid`; 用户主页链接设置 `_douyin_uid`, referer 为 `https://www.douyin.com`.
3. `set_cookie` 对单视频可无登录; 若是主页批量下载, 必须获取 Douyin 登录 cookie.
4. `download` 先用 `_get_douyion_video_url` 解析单条; 对 `_douyin_mid` 调 `_get_douyin_video_url_list`, 对已登录 `_douyin_uid` 调 `_get_douyin_user_video_url_list`, 再逐条下载.

### Go 实现
1. `getTTWID` 调 `https://ttwid.bytedance.com/ttwid/union/register/` 取 `ttwid` cookie.
2. `resolve` 支持 `v.douyin.com` 跳转和普通视频 ID, 请求 `https://www.iesdouyin.com/share/video/{id}/`, 从 `window._ROUTER_DATA` 递归找 `video.play_addr`.
3. `buildStreams` 用 `play_addr.uri` 组装 `https://aweme.snssdk.com/aweme/v1/play/?video_id=...`, 按 `default/1080p/720p/540p/360p` 探测大小并返回 mp4 streams.
4. 未接收或使用登录 cookie 来实现用户主页批量列表.

### 判定
- MISSING_STEP: 单视频无登录路径基本对齐, 但 Python 的 `_douyin_uid` 用户主页批量下载和登录 cookie 分支没有 Go 对应实现.

## cctv

### Python 流程 (5 步)
1. 初始化 header: `cookie`, `Accept: */*`, `Origin: https://www.cctv.cn`, `Referer: https://www.cctv.cn/`, `User-Agent: CCTV_USER_AGENT`.
2. `_request_text` 用同一 header GET 页面, 提取 title, `guid/videoCenterId/pid`, `content_id`.
3. `_request_json` 用同一 header GET `https://vdn.apps.cntv.cn/api/getHttpVideoInfo.do?pid={pid}`.
4. `_candidate_hls_urls` 从 `hls_url`, `video_url`, `chapters4`, `chapters3`, `chapters2`, `chapters` 组候选, 并派生 plain HLS 清晰度 URL.
5. `_inspect_hls_url` / `_select_best_hls_url` 检查 master/variant, 选择最佳 HLS 或视频 URL 后下载.

### Go 实现
1. 页面 GET 只带 `Referer`, 没有复刻 Python 的 `Accept/Origin/User-Agent/cookie` 统一 header.
2. API GET 使用 `nil` headers, 与 Python `_request_json(self.header)` 不一致.
3. 只解析顶层 `title`, `hls_url`, `video_url`, `chapters_url`.
4. 没有实现 `chapters4/chapters3/chapters2/chapters` 候选顺序, plain HLS 派生和 inspect/select best 流程.

### 判定
- MISALIGNED: header 复用和 JSON 候选链与 Python 不一致.
- MISSING_STEP: `chapters*` 分支, HLS 变体探测和最佳流选择缺失.

## chaoxing

### Python 流程 (6 步)
1. 解析普通课程页, `mycourse/stu`, `mooc2-ans`, `xueyinonline` portal, school host 等多种入口, 设置 `course_host/new_course_host/portal_prefix/Referer`.
2. `_get_cid` 从 URL, portal 页, joined 页, old joined 页和 apply-course 流程提取 `courseId/clazzId/enc/cpi/openc`.
3. `_get_title` / `_get_infos` 取中间页和课程页, 解析章节树或 timeline, 必要时探测 `openc`, 并取课程文件列表.
4. 每个章节先 POST `studentstudyAjax`, 再 GET `knowledge/cards`, 从 `mArg.attachments` 或 `data="{...}"` 解析 `objectid`, `mid`, `liveId`, `jobid`, `uuid`, `url`, 类型和标题.
5. 普通视频走 `ananas/status/{objectid}` 并按 `download/httphd/http/httpmd` 选 URL; 直播走 `ananas/live/liveinfo`; 会议/音频走 `getMeetReview4Job` -> `getYunFile`, 或音频接口.
6. 额外处理字幕 `richvideo/subtitle`, Yun 文件 `download`, HTML 图文转 PDF, 以及 file/material 下载.

### Go 实现
1. 只要求 cookie, 从 URL 或页面正则提取 `objectId`.
2. 直接 GET `https://mooc1.chaoxing.com/ananas/status/{objectId}`.
3. 只解析 `filename`, `http`, `hls`, 生成 mp4/m3u8 stream.
4. 未实现课程/portal/joined 页面解析, 章节卡片遍历, live/meet/audio/yun-file/material/html/subtitle 分支.

### 判定
- MISSING_STEP: Go 是 direct-object resolver, 缺失 Python 课程树, 卡片资源解析, live/meeting/audio/file/html/subtitle 等关键下载分支.

## enetedu

### Python 流程 (5 步)
1. Base 设置 `origin/referer/login_url/api_base/token_key`, token 通过 `eneteduToken` 或 Authorization 参与请求.
2. `_get_title` GET `/course/broadcast/glanceAndGet?id=...`, 取 `data.courseId/courseName/name/title`.
3. 直播/回放课走 `/task/homeView`, 再按任务节点 GET `/task/node/get`, 解析 `name/title/realId/id` 和 `playbackUrl/url/sourceAddress`.
4. 学习树走 `/course/learningCourseTreeList`, 解析 `fileName/mediaName/chapterName`, `videoId/mediaId`, `filePath/playUrl/url`.
5. 需要转码时 POST `/file/getMediaTranscodeInfo`, 再 POST playback deal 接口, 解析 `transcodeOutputs/list[].playUrl/url/filePath`.

### Go 实现
1. `Extract` 复刻 detail GET, token/header 由 shared session 构造.
2. `parseLiveTasks` GET `task/homeView`, `resolveNodeURL` GET `task/node/get`.
3. `parseLearningTree` GET 学习树, `walkLearningPayload` 递归提取学习资源.
4. `resolveLearningURL` POST transcode, `dealPlaybackURL` POST playback deal.
5. JSON 字段链覆盖 source 中的 course, task, node, learning tree, transcode outputs 和 playback URL key.

### 判定
- ALIGNED: 媒体 URL 获取链, token/header, GET/POST 顺序和 JSON 字段链与 Python 对齐.

## eoffcn

### Python 流程 (5 步)
1. 登录态下先用 `new_order_url` 获取新商品/课程列表; 另有 `order_url = /api/order/complete` 旧订单列表, 用于 `spuId/systemSn/payMoney/goodsName` 和标题/价格选择.
2. `_get_cid` 可从 URL 或页面提取 `spuId/system_order`, 再通过 course list 选择 `system_order`.
3. 课程包解析顺序包括 `package_url`, `catagory_url`, `course_list_url`, 得到 lesson/package/module 信息.
4. `_get_lesson_info` GET `lesson_url?lesson_id=&package_id=&module_type=&system_order=`, 先找 `data.video_url` / `data.live_url`.
5. 无直接 URL 时走 `pub_key_url` bootstrap 和 `encrypt_url = /api/user/watch_demand`, 从返回 JSON 取 `data.live_url` 等媒体 URL.

### Go 实现
1. 支持从 URL 提取 `system_order/package_id/lesson_id/module_type/coding`.
2. `resolveCourse` 调 `course_list_url`, `catagory_url`, `package_url`, `new_order_url`, 递归收集 lesson 节点.
3. `resolveLesson` 调 `lesson_url`, 递归找 `video_url/live_url`, 缺失时 `requestWatchDemand` 调 `pub_key_url` 后 POST `encrypt_url`.
4. `order_url` 常量存在, 但当前没有 call site, 旧订单 `spuId/systemSn/payMoney/goodsName` 选择链未实现.

### 判定
- MISSING_STEP: 媒体详情和 watch-demand 播放链已覆盖, 但 Python 的 `order_url` 旧订单列表/标题价格/`systemSn` 选择步骤缺失.

## gaotu

### Python 流程 (6 步)
1. 按品牌选择端点: `api.gaotu.cn/p_client=1`, `api.gaotu100.com/p_client=2`, `api.gtgz.cn/p_client=8`, `api.naiyouxuexi.com/p_client=18`, 并切换对应 `interactive.*` host.
2. `_get_course_list` POST `studyPlatform/v1/unit/clazz/list`, 选择 `clazzNumber/title`.
3. `_get_title` 调 `_get_price`, GET `cs/api/product/course/detailButton?productSpuNumber={cid}` 设置价格.
4. `_get_infos` POST `studyCenter/v1/user/pc/clazz/detail`, 从 `clazzDetailChapterPcVO.chapterItemVOList[].lessonCardList[]` 或 `lessonCardList[]` 提取 `userClazzLessonBaseVO.clazzLessonNumber/clazzLessonName`.
5. 视频/直播课节分别 POST `live/zplan/login/videoLive` 和 `live/api/live/zplan/playbackWeb`, 读取 `data.pcUrl` 或直接媒体 URL.
6. `pcUrl` 需要解码到 Wenzai `getPlayUrl` 或 `getPlaybackInfoV4`, 再解析 `data.cdn_list[].url/enc_url`; 另有 `pan/listDir` + `pan/file` 文件资源分支.

### Go 实现
1. `endpointsFor` 已按 `gaotu100/gtgz/naiyouxuexi/gaotu` 切换 API host, interactive host, referer 和 `p_client`.
2. `resolveCourse` 先 POST detail, 无节点时 fallback POST course list, 并递归收集 lesson 节点.
3. `resolveLesson` POST `videoLive` / `playbackWeb`, `mediaFromPayload` 解析直接媒体 URL 或 `pcUrl`.
4. `decodePcURL` 按 `vid` / `room_id` 组装 Wenzai `getPlayUrl` / `getPlaybackInfoV4`, 解析 `url/enc_url`.
5. `sourceURL/fileURL/priceURL` 常量存在, 但 `priceURL`, `sourceURL`, `fileURL` 当前均无执行路径.

### 判定
- MISSING_STEP: 主视频/直播播放 URL 获取链已对齐, 但 Python 的价格接口和 `pan/listDir` + `pan/file` 资料资源分支没有 Go 对应实现.

## houda

### Python 流程 (6 步)
1. `Houda_Base._check_cookie` 调 `api/center/sysUserPower/anon/ifLogin`, 要求 `code == '1'`.
2. `_get_course_list` 登录后 POST `online/myOnline/anon/getLearnFirstPage`, 解析已学课程并支持无 classId 时选择课程.
3. `_get_cid/_get_title` 解析 URL classId 或从课程列表选择, 设置 `cid/title/price/purchased`.
4. `_get_stage_law_data` POST `online/myOnline/getXxStageAndLawList`; `_get_lesson_list` POST `myOnlineCourse/getLearnCourse` 取 `data.liveList`.
5. 每节课从 `roomId/mainRoomId/ccLiveId/recordId` 组合回放信息, GET `live/cc/anon/viewPlayback/{room_id}/{record_id}` 取得 CSSLCloud callback 参数.
6. CSSLCloud 流走 replay `user/login` -> `video/play` -> `data/meta`, m3u8 时重写 key URL; 同时源码还声明 live_detail/material 资源分支.

### Go 实现
1. `checkHoudaCookie` 调 `ifLogin`, 校验 `code == 1`.
2. 只从输入 URL 正则解析 `classId`; 解析不到直接报错, 不走课程列表选择.
3. `fetchHoudaLessons` POST `getLearnCourse`, 解析 `data.liveList`.
4. `resolveHoudaCCCallback` GET `viewPlayback/{roomID}/{recordID}`, 取 `userId/roomId/recordId/viewerToken`.
5. `resolveHoudaCSSL` 使用 `shared.CssLcloudResolvePlayInfo`, `rewriteHoudaM3U8` 使用 `shared.CssLcloudRewriteM3U8Keys`.
6. `urlCourseList/urlStageLaw/urlLiveDetail/urlMaterial` 声明存在, 但课程列表选择, stage-law 详情和 material 分支没有调用.

### 判定
- MISSING_STEP: CSSLCloud 播放解析已按共享 helper 对齐, 但 Python 的课程列表选择, stage-law 详情和 material/live-detail 资源分支未实现; 当前 Go 仅支持已含 `classId` 的路径.

## htknow

### Python 流程 (6 步)
1. Cookie 必须含 `token`, `user`, `custom_id`, `base_KEY`; `_check_cookie` POST `pc_view/learn/list`, 设置 Bearer authorization, `base_KEY`, `custom_id`, `user_id/login_user_id/account_list`.
2. `_get_course_list` POST `learn/list_v2`, 分页/账号列表获取 `product_id/main_product_id/type_desc/title`.
3. 按 `type_desc` 分支 POST `single_detail`, `live/live_wx/playback_list`, `column_course_list`, `series_course_list`.
4. 产品详情走 `column_play_details` 和 `pc_view/course/column_play_details`, 读取 `product_token/pay_content/article_detail/detail`.
5. `_get_video_url` 对 `product_token` 做 base64 JSON 解析, 取 `value/iv`, 使用 cookie 中 `base_KEY` 做 AES-CBC 解密, 明文 URL 必须以 `http` 开头; HTML-only `pay_content` 也被保留.
6. 字节码中还有 answer/quest 分支: `get_quest_tag_list`, `get_quest_num_list`, `get_quest_list`, `create_question_paper`, 构造答题 HTML 并下载.

### Go 实现
1. `newCtx` 从 cookie 取 `token/custom_id/base_KEY/user`, POST `pc_view/learn/list` 校验, 组 Bearer header 和账号列表.
2. `courseList` POST `learn/list_v2`, `sourcesForCourse` 覆盖 single/live/column/series 分支.
3. `fetchProductURL` 覆盖 `column_play_details` 和 `pc_view/course/column_play_details`, 保留 `pay_content`.
4. `videoURL` 完成 product_token base64 JSON -> `value/iv` -> `base_KEY` AES-CBC 解密 -> HTTP URL 校验.
5. `mediaFromSources` 对 HTML-only 内容输出 `data:text/html` document stream.
6. 只声明 answer URL 常量, 没有 `_download_answer_source` 等 answer/quest 调用链.

### 判定
- MISSING_STEP: 视频/音频/图文/专栏/系列核心媒体链已对齐, 但 Python answer/quest HTML 下载分支未实现.

## itbaizhan

### Python 流程 (6 步)
1. `_check_cookie` GET `index_new/index/checkUserLogin`, 检查 `code/user_id`.
2. `_get_course_list` GET `mine/courseschedule`, HTML 解析已购课程; URL 也可解析 `course/id`, `stages/id`, slug 等.
3. `_get_nav_stage_ids` GET `index/stage/navlist?id={course_id}&stage=0`, 递归 `nav[].id/child/children`.
4. `_get_title` GET `course/id/{course_id}.html`, HTML 解析标题和 stage IDs.
5. `_get_stage_info` GET `index/stage/rightlist?id={stage_id}`, 解析 `type.type_name`, `specific[].s_id/s_name`, `child[].course_id/course_name`, `training[].t_id/t_name`.
6. `_get_play_info/_parse_play_info` 从课页取 Polyv `vid/playsafe/title`, 再走 Polyv secure download info 与 m3u8 key rewrite.

### Go 实现
1. `checkCookie` GET `checkUserLogin`, 解析 `code/user_id`.
2. `parseCourseRef`, `courseIDForSlug`, `firstPurchasedCourse`, `getCourseList` 覆盖 URL 和已购课程选择.
3. `getNavStageIDs`, `loadTitleAndStages`, `loadInfos` 对应 navlist, course page, rightlist.
4. `parseVideoInfo` 和 `parseFileInfo` 处理 video child 和 training file.
5. `getPlayInfo` 解析 `vid/playsafe/title`; `resolveVideo` 使用 `shared.PolyvResolveSecure`, `shared.PolyvPickBestManifest`, `shared.PolyvRewriteM3U8Keys`.

### 判定
- ALIGNED: 认证, 课程选择, stage/rightlist, Polyv 播放和资料条目逻辑均有 Go 对应实现.

## lizhiweike

### Python 流程 (6 步)
1. Base 从 cookie 取 `token/id/id_token`, GET `open.lizhiweike.com/oauth2/check_token?token=...`, 要求 `code == 0` 且 `is_valid == true`.
2. `_get_course_list` GET `personal_center/my_weike/{wid}/my_lectures?token=&offset=&limit=10`, 过滤 `status != deleted`, 取 `name/id/liveroom_id/type`.
3. `_get_cid` 对 lecture URL 先 GET `lecture/{vid}/info`, 通过 `data.channel.id` 和 channel `data.object_id` 回到频道; 必要时从课程列表选择.
4. `_get_infos` GET `api/{type}/{cid}/info?token=`, 取 `data.share_info.share_title`, `data.lectures` 或 `data.lecture`.
5. 视频课 GET `lecture/{vid}/info`, 取 `data.video_info.qcloud_video_file_id`, 再 GET `bridge/qcvideo/{vfid}?token=&al=drm`, 从 `data.play_list[].definition/url/size` 选清晰度; 直播 GET `tic/record`, 音频取 `data.audio_info.audio_url`.
6. `default/classroom` 分支分别 GET `message/get/voice` 和 `message/list?new_classroom=1&is_reverse=0&limit=2000`, 取 `data[].audio` 或 `data.messages[].meta.video_url`; channel 可通过 `join_url` 订阅.

### Go 实现
1. `lizhiBuildSession` 复刻 token/id/id_token cookie, `check_token` 校验和 header 构造.
2. `lizhiFetchCourseList` 分页 GET `my_lectures`, 过滤 deleted 并保存 `type/id/liveroom_id/name`.
3. `lizhiResolveTarget` 对 lecture URL 先查 `lecture/{id}/info`, 再查 channel info 的 `object_id`, 并从课程列表匹配或默认选第一门.
4. `Extract` GET `api/{type}/{id}/info`, `lizhiLecturesFromInfo` 解析 `lectures/lecture`.
5. `lizhiBuildEntry` 覆盖 audio, live_v, default, video fallback; `lizhiVideoURL`, `lizhiLiveURL`, `lizhiAudioURL`, `lizhiAudioURLList`, `lizhiVideoURLList` 覆盖 Python 的媒体 URL 字段链.
6. `urlBuyRecord` 只声明未执行, 但该价格元数据不参与当前媒体 URL 解析与资源输出判定.

### 判定
- ALIGNED: 媒体/资源 URL 获取主链, token 校验, 课程列表/lecture-channel 回退, 视频/直播/音频/classroom 分支均与 Python 对齐.
