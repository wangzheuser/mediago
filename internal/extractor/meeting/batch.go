package meeting

import (
	"net/url"
	"regexp"
	"strings"
)

func parseMeetingBatchText(content string) []meetingBatchItem {
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	matches := meetingURLTextRe.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]meetingBatchItem, 0, len(matches))
	for i, loc := range matches {
		rawURL := normalizeMeetingURL(content[loc[0]:loc[1]])
		prevEnd := 0
		if i > 0 {
			prevEnd = matches[i-1][1]
		}
		nextStart := len(content)
		if i+1 < len(matches) {
			nextStart = matches[i+1][0]
		}
		beforeURL := meetingContextTail(content[prevEnd:loc[0]])
		afterURL := meetingContextHead(content[loc[1]:nextStart])
		block := beforeURL + "\n" + afterURL
		password := meetingPasswordFromContext(rawURL, afterURL, block)
		title := meetingTitleFromContext(beforeURL, block)
		key := rawURL + "\x00" + password
		if rawURL == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, meetingBatchItem{URL: rawURL, Password: password, Title: title})
	}
	return out
}

func normalizeMeetingURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	rawURL = strings.Trim(rawURL, `'"“”‘’[]()<>《》「」【】`)
	rawURL = strings.TrimRight(rawURL, `'"“”‘’)]}>】》」,，.。;；!！?？`)
	return rawURL
}

func cleanMeetingTitle(title string) string {
	title = strings.TrimSpace(title)
	title = regexp.MustCompile(`^\s*(?:[①②③④⑤⑥⑦⑧⑨⑩⑪⑫⑬⑭⑮⑯⑰⑱⑲⑳]\s*|[0-9０-９]+\s*[、\):：]\s*|[0-9０-９]+\s*[\.．]\s*|[0-9０-９]+\s+)`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`^\s*录制\s*[:：]?\s*`).ReplaceAllString(title, "")
	title = strings.Trim(title, ` "'“”‘’`)
	return clean(title)
}

func meetingContextTail(text string) string {
	parts := regexp.MustCompile(`\n\s*\n`).Split(text, -1)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func meetingContextHead(text string) string {
	parts := regexp.MustCompile(`\n\s*\n`).Split(text, 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func meetingPasswordFromContext(rawURL, afterURL, block string) string {
	if pwd := passwordFromURL(rawURL); pwd != "" {
		return pwd
	}
	for _, text := range []string{afterURL, block} {
		if m := passwordContextRe.FindStringSubmatch(text); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	for _, line := range strings.Split(afterURL+"\n"+block, "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, `'"“”‘’[]()<>《》「」【】`)
		if regexp.MustCompile(`^[A-Za-z0-9_-]{4,12}$`).MatchString(line) {
			return line
		}
	}
	return ""
}

func meetingTitleFromContext(beforeURL, block string) string {
	for _, lines := range [][]string{reverseLines(beforeURL), strings.Split(block, "\n")} {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || skipTitleLineRe.MatchString(line) {
				continue
			}
			m := titleLabelRe.FindStringSubmatch(line)
			if len(m) < 2 || strings.Contains(strings.ToLower(m[1]), "http") {
				continue
			}
			if title := cleanMeetingTitle(m[1]); title != "" && title != "腾讯会议录制" {
				return title
			}
		}
	}
	for _, line := range reverseLines(beforeURL) {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(strings.ToLower(line), "http") || skipTitleLineRe.MatchString(line) {
			continue
		}
		if regexp.MustCompile(`(?i)^\s*(?:访问密码|文件密码|提取码|访问码|口令|密码|访问|passcode|password)\s*[:：]`).MatchString(line) {
			continue
		}
		if title := cleanMeetingTitle(line); title != "" && title != "腾讯会议录制" {
			return title
		}
	}
	return ""
}

func reverseLines(text string) []string {
	lines := strings.Split(text, "\n")
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines
}

func mergeMeetingBatchTitle(inputTitle, title string) string {
	inputTitle = cleanMeetingTitle(inputTitle)
	title = clean(title)
	if inputTitle == "" {
		return title
	}
	if title == "" || title == "腾讯会议录制" {
		return inputTitle
	}
	if strings.HasPrefix(title, inputTitle+"_") {
		return title
	}
	base, ext := splitTitleExt(title)
	suffix := "录制"
	if m := regexp.MustCompile(`_(录制(?:_\d+)?|屏幕(?:_\d+)?|回放_\d+(?:_\d+)?)$`).FindStringSubmatch(base); len(m) > 1 {
		suffix = m[1]
	}
	return inputTitle + "_" + suffix + ext
}

func splitTitleExt(title string) (string, string) {
	for _, ext := range []string{".mp4", ".m3u8", ".mov", ".flv"} {
		if strings.HasSuffix(strings.ToLower(title), ext) {
			return strings.TrimSuffix(title, title[len(title)-len(ext):]), title[len(title)-len(ext):]
		}
	}
	return title, ""
}

func passwordFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	q := u.Query()
	return first(q.Get("pwd"), q.Get("password"), q.Get("passcode"), q.Get("pw"))
}
