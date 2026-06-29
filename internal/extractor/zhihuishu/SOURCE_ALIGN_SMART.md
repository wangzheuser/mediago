# zhihuishu smart 源码对齐补充

## 加密通道

| Python 源码 | Go 实现 | 状态 |
|---|---|---|
| `_RSA_PUB_B64`, `_HAS_URL` | `smartRSAPublicKeyB64`, `urlSmartHasAESKey` | ✓ |
| `_get_aes_key`: RSA PKCS1 encrypt `{"module":6}` -> `/c/has` -> public exponent unwrap `rt.sl` -> `cKey` | `smartGetAESKey`, `rsaPublicUnpad` | ✓ |
| `_post_encrypted`: AES-CBC/PKCS7 `secretStr`, `date`, `Content-Type`, `XQJZXHIZ` | `smartSession.postEncrypted` | ✓ |

## 课程与资源

| Python 源码 | Go 实现 | 状态 |
|---|---|---|
| `_get_title`: `url_map_detail`, `url_get_map_uid` | `resolveTitle` | ✓ |
| `_get_infos`: `url_knowledge_dic`, `url_map_knowledge_dic` | `smartNodes`, `smartNodesFromThemes` | ✓ |
| `_get_node_resources`: `url_node_resources`, `url_wisdom_resources` | `nodeResources` | ✓ |
| `_get_resource_tasks`, `_get_task_resources` | `collectTaskEntries` | ✓ |
| `_get_course_resource_list`, `_get_course_file_url` | existing `collectSmartCourseResources`, `getSmartCourseFileURL` | ✓ |
| `_get_video_url`: `initVideoNew`, `changeVideoLine` | `getSmartVideoURL` | ✓ |

## 验证

- `go vet ./internal/extractor/zhihuishu/...`
- `go test ./internal/extractor/zhihuishu -count=1`
