package haozaixian

import (
	"fmt"
	"sort"
	"strings"
)

func (x *hzCtx) buildCourseMap(courseType string) []hzCourse {
	rows := x.requestCourseList(courseType)
	var out []hzCourse
	for _, row := range rows {
		courseInfo := asMap(row["clCourseInfo"])
		courseID := str(courseInfo["courseId"])
		cardInfo := asMap(row["clCardInfo"])
		title := cleanName(str(cardInfo["title"]))
		teacherInfo := asMap(cardInfo["clTeacherInfo"])
		teachers := listAt(teacherInfo, "clMainTeacherList")
		teacherNames := []string{""}
		if len(teachers) > 0 {
			teacherNames = teacherNames[:0]
			for _, teacher := range teachers {
				if name := cleanName(str(teacher["teacherName"])); name != "" {
					teacherNames = append(teacherNames, name)
				}
			}
		}
		if courseID == "" || title == "" {
			continue
		}
		if len(teacherNames) == 0 {
			teacherNames = []string{""}
		}
		nCourseType := str(courseInfo["nCourseType"])
		for _, teacher := range teacherNames {
			fullTitle := title
			if teacher != "" {
				fullTitle = fmt.Sprintf("%s-%s", title, teacher)
			}
			out = append(out, hzCourse{CourseID: courseID, Title: cleanName(fullTitle), TeacherName: teacher, CourseType: courseType, NCourseType: nCourseType, Raw: row})
		}
	}
	return out
}

func (x *hzCtx) requestCourseList(courseType string) []map[string]any {
	x.setCourseType(courseType)
	var out []map[string]any
	lastOffset := ""
	for page := 0; page < 200; page++ {
		if strings.TrimSpace(courseType) == SYSTEM_COURSE_TYPE {
			filter := `[{"key": "subject", "value": -1}, {"key": "status", "value": -1}]`
			root, err := x.postFormJSON(system_course_list_url, []kv{
				{k: "cuid", v: "4c-5f-70-95-cc-75"},
				{k: "cancelKey", v: "go-to-class-v4-list"},
				{k: "filter", v: filter},
				{k: "limit", v: "20"},
				{k: "lastOffsetFlag", v: lastOffset},
				{k: "tabId", v: "181014"},
				{k: "appId", v: "winhaoke"},
				{k: "deviceType", v: "pc"},
				{k: "vcname", v: x.vcname},
				{k: "vc", v: x.vc},
				{k: "os", v: "stuwin"},
			})
			if err != nil {
				break
			}
			data := asMap(root["data"])
			rows := listAt(data, "clCourseList")
			if len(rows) == 0 {
				break
			}
			out = append(out, rows...)
			if !truthy(data["hasMore"]) {
				break
			}
			lastOffset = str(data["offsetFlag"])
			if lastOffset == "" {
				break
			}
			continue
		}
		root, err := x.requestJSON(queryURL(special_course_list_url,
			kv{k: "na__zyb_source__", v: x.naSource},
			kv{k: "appId", v: "winhaoke"},
			kv{k: "cuid", v: "4c-5f-70-95-cc-75"},
			kv{k: "vcname", v: x.vcname},
			kv{k: "vc", v: x.vc},
			kv{k: "os", v: "stuwin"},
			kv{k: "limit", v: "40"},
			kv{k: "lastOffsetFlag", v: lastOffset},
			kv{k: "subject", v: "-1"},
			kv{k: "status", v: "-1"},
			kv{k: "nCourseTypes", v: "43,72,33,0,12,13,66,1,4,32,63,19,181015"},
		), nil)
		if err != nil {
			break
		}
		if str(root["errNo"]) != "0" {
			break
		}
		data := asMap(root["data"])
		rows := listAt(data, "clCourseList")
		if len(rows) == 0 {
			break
		}
		out = append(out, rows...)
		if !truthy(data["hasMore"]) {
			break
		}
		lastOffset = str(data["offsetFlag"])
		if lastOffset == "" {
			break
		}
	}
	return out
}

func (x *hzCtx) getTitle() error {
	if x.cid == "" {
		return nil
	}
	root, err := x.requestJSON(queryURL(course_full_url, kv{k: "courseId", v: x.cid}, kv{k: "appId", v: "winhaoke"}), nil)
	if err != nil {
		return err
	}
	data := asMap(root["data"])
	if x.title == "" {
		for _, key := range []string{"courseName", "title", "name"} {
			if title := cleanName(str(data[key])); title != "" {
				x.title = title
				break
			}
		}
		if x.title == "" {
			base := asMap(data["courseBaseInfo"])
			for _, key := range []string{"courseName", "title", "name"} {
				if title := cleanName(str(base[key])); title != "" {
					x.title = title
					break
				}
			}
		}
	}
	if nct := firstNonEmpty(str(data["nCourseType"]), str(asMap(data["courseBaseInfo"])["nCourseType"]), str(asMap(data["courseBaseInfo"])["courseType"])); nct != "" {
		x.nCourseType = nct
	}
	return nil
}

func (x *hzCtx) getInfos() ([]hzLesson, []hzMaterial, error) {
	if x.cid == "" {
		return nil, nil, fmt.Errorf("haozaixian: missing course id")
	}
	x.title = cleanName(x.title)
	root, err := x.requestJSON(queryURL(course_full_url, kv{k: "courseId", v: x.cid}, kv{k: "appId", v: "winhaoke"}), nil)
	if err != nil {
		return nil, nil, err
	}
	data := asMap(root["data"])
	if x.nCourseType == "" {
		x.nCourseType = firstNonEmpty(str(data["nCourseType"]), str(asMap(data["courseBaseInfo"])["nCourseType"]), str(asMap(data["courseBaseInfo"])["courseType"]))
	}
	if x.nCourseType == AI_COURSE_TYPE {
		return x.getAIInfos()
	}
	lessonList := listAt(asMap(data["subItemInfo"]), "lessonList")
	if len(lessonList) == 0 {
		lessonList = listAt(data, "lessonList")
	}
	sort.SliceStable(lessonList, func(i, j int) bool {
		return intVal(firstExisting(lessonList[i]["index"], asMap(lessonList[i]["lessonInfo"])["index"])) < intVal(firstExisting(lessonList[j]["index"], asMap(lessonList[j]["lessonInfo"])["index"]))
	})
	lessons := make([]hzLesson, 0, len(lessonList))
	var courseMaterials []hzMaterial
	for seq, item := range lessonList {
		lessonInfo := asMap(item["lessonInfo"])
		lessonID := firstNonEmpty(str(lessonInfo["lessonId"]), str(item["lessonId"]))
		lessonName := cleanName(firstNonEmpty(str(lessonInfo["lessonName"]), str(lessonInfo["title"]), str(item["lessonName"]), str(item["title"]), str(item["name"])))
		if lessonID == "" || lessonName == "" {
			continue
		}
		liveRoomID := firstNonEmpty(str(asMap(asMap(lessonInfo["integrateRoomInfo"])["roomInfo"])["liveRoomId"]))
		if liveRoomID == "" {
			roomInfo := listAt(asMap(lessonInfo["integrateRoomInfo"]), "roomInfo")
			if len(roomInfo) > 0 {
				liveRoomID = firstNonEmpty(str(roomInfo[0]["liveRoomId"]), str(roomInfo[0]["roomId"]))
			}
		}
		if liveRoomID == "" {
			if liveRooms := listAt(item, "liveRoomList"); len(liveRooms) > 0 {
				liveRoomID = firstNonEmpty(str(liveRooms[0]["liveRoomId"]), str(liveRooms[0]["roomId"]))
			}
		}
		l := hzLesson{
			Name:       fmt.Sprintf("[%d.1]--%s", seq+1, lessonName),
			LessonName: lessonName,
			LessonID:   lessonID,
			LiveRoomID: liveRoomID,
			Type:       "video",
			Seq:        seq + 1,
		}
		if mats := x.requestLessonMaterials(lessonID); len(mats) > 0 {
			for i, m := range mats {
				if m.URL == "" || m.Name == "" {
					continue
				}
				l.Materials = append(l.Materials, hzMaterial{Name: fmt.Sprintf("(%d.%d)--%s", seq+1, i+1, cleanName(m.Name)), URL: m.URL, Kind: m.Kind})
			}
		}
		lessons = append(lessons, l)
	}
	if mats := x.requestCourseMaterials(); len(mats) > 0 {
		courseMaterials = append(courseMaterials, mats...)
	}
	if len(lessons) == 0 && len(courseMaterials) == 0 {
		return nil, nil, fmt.Errorf("haozaixian: no lessons found")
	}
	return lessons, courseMaterials, nil
}

func (x *hzCtx) requestLessonMaterials(lessonID string) []hzMaterial {
	root, err := x.requestJSON(queryURL(lesson_material_url,
		kv{k: "na__zyb_source__", v: x.naSource},
		kv{k: "appId", v: "winhaoke"},
		kv{k: "lessonId", v: lessonID},
		kv{k: "vcname", v: x.vcname},
		kv{k: "vc", v: x.vc},
		kv{k: "os", v: "stuwin"},
	), nil)
	if err != nil {
		return nil
	}
	rows := listAt(asMap(root["data"]), "materialList")
	var out []hzMaterial
	for _, item := range rows {
		if url := firstNonEmpty(str(item["fileUrl"]), str(item["resourceUrl"]), str(item["url"])); url != "" {
			out = append(out, hzMaterial{Name: cleanName(firstNonEmpty(str(item["title"]), str(item["name"]), url)), URL: url, Kind: "material"})
		}
	}
	return out
}

func (x *hzCtx) requestCourseMaterials() []hzMaterial {
	root, err := x.requestJSON(queryURL(course_material_url, kv{k: "na__zyb_source__", v: x.naSource}, kv{k: "appId", v: "winhaoke"}, kv{k: "courseId", v: x.cid}), nil)
	if err != nil {
		return nil
	}
	rows := listAt(asMap(root["data"]), "courseMaterialList")
	if len(rows) == 0 {
		rows = listAt(asMap(root["data"]), "materialList")
	}
	return x.flattenMaterialRows(rows, nil)
}

func (x *hzCtx) requestFolderMaterials(folder map[string]any) []hzMaterial {
	return x.flattenMaterialRows(x.requestFolderRows(folder), nil)
}

func (x *hzCtx) requestFolderRows(folder map[string]any) []map[string]any {
	root, err := x.requestJSON(queryURL(file_material_url,
		kv{k: "na__zyb_source__", v: x.naSource},
		kv{k: "appId", v: "winhaoke"},
		kv{k: "courseId", v: x.cid},
		kv{k: "lessonId", v: str(folder["lessonId"])},
		kv{k: "pid", v: str(folder["pid"])},
		kv{k: "vcname", v: x.vcname},
		kv{k: "vc", v: x.vc},
		kv{k: "os", v: "stuwin"},
	), nil)
	if err != nil {
		return nil
	}
	return listAt(asMap(root["data"]), "materialList")
}

func (x *hzCtx) flattenMaterialRows(rows []map[string]any, prefix []int) []hzMaterial {
	var out []hzMaterial
	for i, item := range rows {
		idx := append(append([]int{}, prefix...), i+1)
		name := cleanName(firstNonEmpty(str(item["title"]), str(item["name"]), str(item["resourceName"])))
		if name == "" {
			name = fmt.Sprintf("material_%d", i+1)
		}
		if str(item["folderType"]) == "1" {
			if children := x.requestFolderRows(item); len(children) > 0 {
				out = append(out, x.flattenMaterialRows(children, idx)...)
			}
			continue
		}
		url := firstNonEmpty(str(item["resourceUrl"]), str(item["fileUrl"]), str(item["url"]))
		if url == "" {
			continue
		}
		label := name
		if len(idx) > 0 {
			parts := make([]string, len(idx))
			for j, n := range idx {
				parts[j] = fmt.Sprint(n)
			}
			label = fmt.Sprintf("(%s)--%s", strings.Join(parts, "."), name)
		}
		out = append(out, hzMaterial{Name: label, URL: url, Kind: "material"})
	}
	return out
}
