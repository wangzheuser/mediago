package xsteach

import (
	"fmt"
	"path"
	"strings"

	"github.com/Sophomoresty/mediago/internal/extractor"
)

type xsFile struct {
	url, title, format, periodID string
}

func filesFromPeriod(p map[string]any, co xsCourse) []xsFile {
	pid := periodID(p)
	baseTitle := firstNonEmpty(val(p, "name"), val(p, "title"), co.title, "资料")
	out := []xsFile{}
	out = append(out, parseResourceValue(p["resourceUrl"], baseTitle, pid)...)
	for _, key := range []string{"resourceFiles", "resourceFileList", "resourceList", "resources", "fileList", "files", "attachments", "materials", "coursewares", "homework"} {
		out = append(out, parseResourceValue(p[key], baseTitle, pid)...)
	}
	for _, tc := range listUnder(p, "teachCoachVideos") {
		tcTitle := firstNonEmpty(val(tc, "name"), val(tc, "title"), baseTitle)
		for _, key := range []string{"resourceUrl", "resourceFiles", "resourceFileList", "resources", "fileList", "files", "attachments", "materials", "coursewares"} {
			out = append(out, parseResourceValue(tc[key], tcTitle, pid)...)
		}
	}
	return uniqueFiles(out)
}

func parseResourceValue(v any, baseTitle, periodID string) []xsFile {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if u := normalizeURL(t); u != "" {
			return []xsFile{{url: u, title: fileTitle(baseTitle, u), format: fileFormat("", u), periodID: periodID}}
		}
	case []any:
		out := []xsFile{}
		for i, x := range t {
			title := baseTitle
			if len(t) > 1 {
				title = fmt.Sprintf("%s-%d", baseTitle, i+1)
			}
			out = append(out, parseResourceValueWithTitle(x, title, periodID)...)
		}
		return out
	case map[string]any:
		return parseResourceMap(t, baseTitle, periodID)
	}
	return nil
}

func parseResourceValueWithTitle(v any, baseTitle, periodID string) []xsFile {
	if m, ok := v.(map[string]any); ok {
		return parseResourceMap(m, baseTitle, periodID)
	}
	return parseResourceValue(v, baseTitle, periodID)
}

func parseResourceMap(m map[string]any, baseTitle, periodID string) []xsFile {
	if len(m) == 0 {
		return nil
	}
	urls := []string{}
	for _, key := range []string{"url", "resourceUrl", "fileUrl", "path", "downloadUrl", "downloadURL", "resourceURL", "resource_url", "attachUrl", "filePath", "file_url"} {
		if u := normalizeURL(val(m, key)); u != "" {
			urls = append(urls, u)
		}
	}
	out := []xsFile{}
	name := firstNonEmpty(val(m, "name"), val(m, "title"), val(m, "fileName"), val(m, "filename"), val(m, "resourceName"), baseTitle)
	formatHint := strings.Trim(strings.ToLower(firstNonEmpty(val(m, "ext"), val(m, "suffix"), val(m, "file_fmt"), val(m, "fileFmt"), val(m, "format"))), ".")
	for _, u := range unique(urls) {
		out = append(out, xsFile{url: u, title: fileTitle(name, u), format: fileFormat(formatHint, u), periodID: periodID})
	}
	for _, key := range []string{"list", "items", "children", "files", "resources", "attachments"} {
		out = append(out, parseResourceValue(m[key], name, periodID)...)
	}
	return out
}

func fileTitle(name, rawURL string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "资料" {
		base := path.Base(strings.SplitN(rawURL, "?", 2)[0])
		if base != "." && base != "/" && base != "" {
			name = base
		}
	}
	return strings.TrimSuffix(name, "."+fileFormat("", rawURL))
}

func fileFormat(hint, rawURL string) string {
	if hint != "" {
		return strings.TrimPrefix(strings.ToLower(hint), ".")
	}
	base := strings.SplitN(rawURL, "?", 2)[0]
	if ext := strings.TrimPrefix(path.Ext(base), "."); ext != "" {
		return strings.ToLower(ext)
	}
	return "pdf"
}

func uniqueFiles(in []xsFile) []xsFile {
	out := []xsFile{}
	seen := map[string]bool{}
	for _, f := range in {
		if f.url == "" || seen[f.url] {
			continue
		}
		seen[f.url] = true
		out = append(out, f)
	}
	return out
}

func fileMedia(f xsFile) *extractor.MediaInfo {
	return &extractor.MediaInfo{
		Site:  "xsteach",
		Title: firstNonEmpty(f.title, "资料"),
		Streams: map[string]extractor.Stream{"file": {
			Quality: "source",
			URLs:    []string{f.url},
			Format:  firstNonEmpty(f.format, fileFormat("", f.url)),
			Headers: map[string]string{"Referer": refererURL},
		}},
		Extra: map[string]any{"type": "file", "period_id": f.periodID},
	}
}
