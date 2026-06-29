package xsteach

import "testing"

func TestFilesFromPeriodParsesResourceURLList(t *testing.T) {
	period := map[string]any{
		"id":   "p1",
		"name": "Lesson",
		"resourceUrl": []any{
			map[string]any{
				"url":      "https://cdn.example.com/handout.pdf",
				"fileName": "讲义.pdf",
				"ext":      "pdf",
			},
			"https://cdn.example.com/extra.docx",
		},
	}
	files := filesFromPeriod(period, xsCourse{title: "Course"})
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	if files[0].url != "https://cdn.example.com/handout.pdf" || files[0].format != "pdf" {
		t.Fatalf("first file = %#v", files[0])
	}
	if files[1].url != "https://cdn.example.com/extra.docx" || files[1].format != "docx" {
		t.Fatalf("second file = %#v", files[1])
	}
}
