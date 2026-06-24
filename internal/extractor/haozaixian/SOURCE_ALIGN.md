# haozaixian 源码对齐对照

## URL 常量

| .cdc.py 行 | Go 行/名 | 一致? |
|---|---|---|
| `Haozaixian_Base.pyc.1shot.cdc.py:30 referer = 'https://www.haoke100.com'` | `haozaixian.go:22 referer` | ✓ |
| `Haozaixian_Base.pyc.1shot.cdc.py:31 order_list_url = 'https://c3-sell.zuoyebang.com/order-ui/order/list/v2'` | `haozaixian.go:24 order_list_url` | ✓ |
| `Haozaixian_Base.pyc.1shot.cdc.py:32 check_url = 'https://c3-jx-stable.zuoyebang.com/frontcourse/teach/course/pccoursefull?courseId=0&appId=winhaoke'` | `haozaixian.go:23 check_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:34 system_course_list_url = 'https://c3-jx-stable.zuoyebang.com/mcourse/winhaoke/course/list'` | `haozaixian.go:26 system_course_list_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:35 special_course_list_url = 'https://c4-jx-stable.zuoyebang.com/teachcourse/index/courselist'` | `haozaixian.go:27 special_course_list_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:36 course_full_url = 'https://c3-jx-stable.zuoyebang.com/frontcourse/teach/course/pccoursefull'` | `haozaixian.go:28 course_full_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:37 ai_video_url = 'https://c4-jx-stable.zuoyebang.com/classme/student/aiclassroom/videoInfo'` | `haozaixian.go:29 ai_video_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:38 special_video_url = 'https://c4-jx-stable.zuoyebang.com/liveme/student/classroom/pre'` | `haozaixian.go:30 special_video_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:39 system_video_url = 'https://c3-jx-stable.zuoyebang.com/liveme/student/classroom/pre'` | `haozaixian.go:31 system_video_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:40 lesson_material_url = 'https://jx.zuoyebang.com/frontcourse/public/material/lessonmaterial'` | `haozaixian.go:32 lesson_material_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:41 course_material_url = 'https://c4-jx-stable.zuoyebang.com/mcourse/winhaoke/matearial/course'` | `haozaixian.go:33 course_material_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:42 file_material_url = 'https://c4-jx-stable.zuoyebang.com/mcourse/winhaoke/matearial/file'` | `haozaixian.go:34 file_material_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:43 course_emphasis_detail_url = 'https://c3-jx-stable.zuoyebang.com/frontcourse/public/courseemphasis/courseemphasisdetail'` | `haozaixian.go:36 course_emphasis_detail_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:44 ai_course_info_url = 'https://aiclass.zuoyebang.com/aiclass-course/api/lesson/getcourseinfo'` | `haozaixian.go:37 ai_course_info_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:45 ai_lesson_detail_url = 'https://aiclass.zuoyebang.com/aiclass-course/api/lesson/getdetail'` | `haozaixian.go:38 ai_lesson_detail_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:46 ai_video_by_round_url = 'https://aiclass.zuoyebang.com/aiclass-course/api/lesson/getvideobyroundid'` | `haozaixian.go:39 ai_video_by_round_url` | ✓ |
| `Haozaixian_Course.pyc.1shot.cdc.py:47 lesson_lecture_url = 'https://c3-jx-stable.zuoyebang.com/frontcourse/public/lecture/lessonlecture'` | `haozaixian.go:40 lesson_lecture_url` | ✓ |

## 认证与 Cookie

| 源码方法 (line) | Go 函数 (line) | 一致? |
|---|---|---|
| `Haozaixian_Base.__init__` line 34 | `newCtx` line 106 + `setCourseType` line 138 | ✓ |
| `Haozaixian_Base._set_course_type` line 98 | `setCourseType` line 138 | ✓ |
| `Haozaixian_Base._check_cookie` line 120 | `checkCookie` line 155 | ✓ |
| `Haozaixian_Course._get_cid` line 205 | `parseCourseRef` line 272 + `prepare` line 210 | ✓ |

## HTTP 调用

| 源码方法 (line) | Go 函数 (line) | method | 一致? |
|---|---|---|---|
| `Haozaixian_Course._request_course_list` line 78 | `requestCourseList` line 46 | GET/POST | ✓ |
| `Haozaixian_Course._get_title` line 256 | `getTitle` line 120 | GET JSON | ✓ |
| `Haozaixian_Course._get_infos` line 671 | `getInfos` line 152 | GET JSON | ✓ |
| `Haozaixian_Course._request_lesson_materials` line 591 | `requestLessonMaterials` line 223 | GET JSON | ✓ |
| `Haozaixian_Course._request_course_materials` line 618 | `requestCourseMaterials` line 245 | GET JSON | ✓ |
| `Haozaixian_Course._request_folder_materials` line 642 | `requestFolderMaterials`/`requestFolderRows` line 257 | GET JSON | ✓ |
| `Haozaixian_Course._get_video_address` line 508 | `getVideoAddress` line 84 | GET JSON + plain m3u8 probe | ✓ |
| `Haozaixian_Course._get_ai_infos` line 763 | `getAIInfos` line 10 | GET JSON | ✓ |
| `Haozaixian_Course._get_ai_round_id` line 813 | `getAIRoundID` line 68 | GET JSON | ✓ |
| `Haozaixian_Course._get_ai_video_urls` line 840 | `getAIVideoURLs` line 80 | GET JSON + JSON parse | ✓ |
| `Haozaixian_Course._get_course_emphasis_images` line 909 | `getCourseEmphasisImages` line 157 | GET JSON | ✓ |
| `Haozaixian_Course._get_lesson_lecture_images` line 959 | `getLessonLectureImages` line 186 | GET JSON | ✓ |

## JSON 字段映射

| 源码 key 链 | Go 解析 | 一致? |
|---|---|---|
| `data.clCourseList` | `requestCourseList` line 69-104 | ✓ |
| `clCourseInfo.courseId / nCourseType` | `buildCourseMap` line 13-40 | ✓ |
| `clCardInfo.title / clTeacherInfo.clMainTeacherList.teacherName` | `buildCourseMap` line 15-40 | ✓ |
| `data.courseName/title/name` | `getTitle` line 129-148 | ✓ |
| `data.subItemInfo.lessonList` | `getInfos` line 168-220 | ✓ |
| `lessonInfo.lessonId / lessonName / integrateRoomInfo.roomInfo.liveRoomId` | `getInfos` line 177-203 | ✓ |
| `data.materialList` | `requestLessonMaterials` line 223-242 / `requestFolderRows` line 257-271 | ✓ |
| `data.courseMaterialList` | `requestCourseMaterials` line 245-254 | ✓ |
| `data.videoInfo.videoAddress` | `getVideoAddress` line 111-119 | ✓ |
| `data.preloading.mixRoomVideoInfo.multiClarityPlaybackVideoData` | `getVideoAddress` line 120-132 / `pickMultiClarityUrls` line 61-95 | ✓ |
| `data.preloading.lbk.lbpVideoAddress` | `getVideoAddress` line 133-136 | ✓ |
| `data.lessonList` | `getAIInfos` line 10-43 | ✓ |
| `data.mainCourseInfo.roundId` | `getAIRoundID` line 68-77 | ✓ |
| `data.styleContent.scenes` | `getAIVideoURLs` line 80-126 | ✓ |
| `data.teacherList / myList` | `getCourseEmphasisImages` line 157-183 | ✓ |
| `data.lecture` | `getLessonLectureImages` line 186-211 | ✓ |

## 阻塞步骤

无。
