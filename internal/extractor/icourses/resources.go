package icourses

import (
	"fmt"
	"strings"
)

func (x *icoursesCtx) cuocResources() ([]resource, error) {
	data, err := x.apiGet(cuoc_resource_api, map[string]string{"courseId": x.cid})
	if err != nil {
		return nil, err
	}
	var out []resource
	for _, item := range pickList(data, "resourcesList", "list", "records") {
		for _, r := range normalizeResourceItem(item, "") {
			if r.Kind == "video" {
				out = append(out, r)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("icourses: empty cuoc resource list")
	}
	return dedupeResources(out), nil
}

func (x *icoursesCtx) moocInfo() (moocInfo, error) {
	data, err := x.apiGet(mooc_chapter_api, map[string]string{"courseId": x.cid})
	if err != nil {
		return moocInfo{}, err
	}
	root := asMap(data)
	var out moocInfo
	chapters := pickList(root, "chapterList", "list", "records")
	if len(chapters) == 0 && len(root) > 0 && (str(root["chapterId"]) != "" || str(root["id"]) != "") {
		chapters = []map[string]any{root}
	}
	for _, ch := range chapters {
		chapterID := firstNonEmpty(str(ch["chapterId"]), str(ch["id"]))
		chapterName := firstNonEmpty(str(ch["chapterName"]), str(ch["name"]), str(ch["title"]), "未命名章节")
		resources := x.getChapterResources(chapterID)
		units := x.flattenUnits(listMaps(ch["children"]), []string{chapterName})
		if len(resources) > 0 || len(units) > 0 {
			out.Chapters = append(out.Chapters, chapter{Name: cleanName(chapterName), ID: chapterID, Resources: resources, Units: units})
		}
	}
	if len(out.Chapters) == 0 {
		for _, item := range pickList(x.detail, "previewList") {
			for _, r := range normalizeResourceItem(item, "课程试看") {
				out.Chapters = append(out.Chapters, chapter{Name: "课程试看", ID: "preview", Resources: []resource{r}})
			}
		}
	}
	papers, sources := x.getOtherResources()
	out.Papers = dedupeResources(append(out.Papers, papers...))
	out.Sources = dedupeResources(append(out.Sources, sources...))
	docs := x.getCourseDocResources()
	out.Sources = dedupeResources(append(out.Sources, docs...))
	return out, nil
}

func (x *icoursesCtx) getChapterResources(chapterID string) []resource {
	if chapterID == "" {
		return nil
	}
	data, err := x.apiGet(mooc_chapter_res_api, map[string]string{"courseId": x.cid, "chapterId": chapterID})
	if err != nil {
		return nil
	}
	root := asMap(data)
	items := pickList(root, "resList", "records", "list", "resourcesList")
	if len(items) == 0 && (str(root["resUrl"]) != "" || str(root["pptResUrl"]) != "" || str(root["resId"]) != "") {
		items = []map[string]any{root}
	}
	var out []resource
	for _, item := range items {
		out = append(out, normalizeResourceItem(item, "")...)
	}
	return dedupeResources(out)
}

func (x *icoursesCtx) flattenUnits(children []map[string]any, pathNames []string) []unit {
	var out []unit
	for _, child := range children {
		if len(child) == 0 {
			continue
		}
		unitID := firstNonEmpty(str(child["chapterId"]), str(child["id"]))
		unitName := firstNonEmpty(str(child["chapterName"]), str(child["name"]), str(child["title"]), "未命名小节")
		currentPath := append(append([]string{}, pathNames...), cleanName(unitName))
		resources := x.getChapterResources(unitID)
		if len(resources) > 0 {
			out = append(out, unit{Name: cleanName(strings.Join(currentPath, " - ")), ID: unitID, Resources: resources})
		}
		if next := listMaps(child["children"]); len(next) > 0 {
			out = append(out, x.flattenUnits(next, currentPath)...)
		}
	}
	return out
}

func (x *icoursesCtx) getOtherResources() ([]resource, []resource) {
	data, err := x.apiGet(mooc_other_res_api, map[string]string{"courseId": x.cid, "curPage": "1", "pageSize": "100"})
	if err != nil {
		return nil, nil
	}
	root := asMap(data)
	items := pickList(root, "records", "list", "items")
	var papers, sources []resource
	for _, item := range items {
		if len(item) == 0 {
			continue
		}
		category := cleanName(str(item["category"]))
		expanded := x.expandShareResources(item, "")
		if len(expanded) == 0 {
			continue
		}
		if category == "习题作业" {
			papers = append(papers, expanded...)
		} else {
			sources = append(sources, expanded...)
		}
	}
	return dedupeResources(papers), dedupeResources(sources)
}

func (x *icoursesCtx) getCourseDocResources() []resource {
	var out []resource
	for _, api := range moocCourseDocAPIs {
		data, err := x.apiGet(api.path, map[string]string{"courseId": x.cid})
		if err != nil {
			continue
		}
		switch v := data.(type) {
		case []any:
			for _, item := range v {
				out = append(out, x.expandShareResources(asMap(item), api.name)...)
			}
		case string:
			out = append(out, x.expandShareResources(map[string]any{"resName": api.name, "resUrl": v}, api.name)...)
		case map[string]any:
			m := cloneMap(v)
			if m["resName"] == nil || str(m["resName"]) == "" {
				m["resName"] = api.name
			}
			out = append(out, x.expandShareResources(m, api.name)...)
		}
	}
	return dedupeResources(out)
}

func (x *icoursesCtx) expandShareResources(item map[string]any, defaultName string) []resource {
	out := normalizeResourceItem(item, defaultName)
	if len(out) > 0 {
		return out
	}
	resID := firstNonEmpty(str(item["resId"]), str(item["id"]))
	if resID == "" {
		return nil
	}
	data, err := x.apiGet(mooc_share_sub_api, map[string]string{"resId": resID})
	if err != nil {
		return nil
	}
	items := pickList(data, "courseSubList", "list", "records", "items")
	for _, sub := range items {
		out = append(out, normalizeResourceItem(sub, defaultName)...)
	}
	return dedupeResources(out)
}

func normalizeResourceItem(item map[string]any, defaultName string) []resource {
	if len(item) == 0 {
		return nil
	}
	resourceName := firstNonEmpty(str(item["resName"]), str(item["name"]), str(item["title"]), str(item["fileName"]), defaultName, str(item["resId"]), "未命名资源")
	resourceName = stripExt(resourceName)
	mediaType := firstNonEmpty(str(item["resMediaType"]), str(item["mediaType"]), str(item["type"]))
	size := parseSizeBytes(firstNonEmptyAny(item["fileSize"], item["resSize"], item["size"]))
	resID := firstNonEmpty(str(item["resId"]), str(item["id"]))
	seen := map[string]bool{}
	var out []resource
	appendURL := func(raw, forcedMediaType string) {
		raw = unwrapResourceURL(str(raw))
		if raw == "" || !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			return
		}
		mtype := mediaType
		if forcedMediaType != "" {
			mtype = forcedMediaType
		}
		ext := guessExt(raw, mtype)
		kind := kindFromExt(ext, mtype)
		key := raw + "|" + kind + "|" + resourceName
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, resource{Name: cleanName(resourceName), URL: raw, Kind: kind, Ext: ext, MediaType: mtype, ResID: resID, Size: size})
	}
	appendURL(str(item["resUrl"]), "")
	appendURL(str(item["url"]), "")
	appendURL(str(item["pptResUrl"]), "ppt")
	return out
}

func dedupeResources(in []resource) []resource {
	seen := map[string]bool{}
	var out []resource
	for _, r := range in {
		key := strings.Join([]string{r.URL, r.Kind, r.Name, r.Ext}, "|")
		if key == "|||" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, r)
	}
	return out
}

func firstNonEmptyAny(vals ...any) any {
	for _, v := range vals {
		if s := str(v); s != "" {
			return v
		}
	}
	return nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
