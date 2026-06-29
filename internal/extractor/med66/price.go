package med66

import (
	"net/url"
	"strings"
)

const MED66_CC_REPLAY_VERSION = "3.6.1"

func courseUpgradeReferer(course med66Course, isAI string) string {
	repl := strings.NewReplacer(
		"courseId={}", "courseId="+url.QueryEscape(course.CourseID),
		"classId={}", "classId="+url.QueryEscape(course.ClassID),
		"classType={}", "classType="+url.QueryEscape(course.ClassType),
		"isAi={}", "isAi="+url.QueryEscape(isAI),
	)
	return repl.Replace(COURSE_UPGRADE_REFERER)
}
