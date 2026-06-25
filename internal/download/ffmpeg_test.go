package download

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFFmpegKeepsSuccessSilent(t *testing.T) {
	script := writeFFmpegStub(t, `echo "quiet stderr" >&2
exit 0`)
	stderr := captureStderr(t, func() {
		if err := runFFmpeg(exec.Command(script)); err != nil {
			t.Fatalf("runFFmpeg returned error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestRunFFmpegPrintsStderrOnFailure(t *testing.T) {
	script := writeFFmpegStub(t, `echo "ffmpeg build info" >&2
exit 1`)
	stderr := captureStderr(t, func() {
		if err := runFFmpeg(exec.Command(script)); err == nil {
			t.Fatal("runFFmpeg returned nil error")
		}
	})
	if !strings.Contains(stderr, "ffmpeg build info") {
		t.Fatalf("stderr = %q, want ffmpeg stderr to be printed", stderr)
	}
}

func TestMuxDASHWritesPartThenRenamesOnSuccess(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "video.mp4")
	outPath := filepath.Join(dir, "merged.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	script := writeFFmpegStub(t, `for last do :; done
echo merged > "$last"
exit 0`)
	engine := New(Opts{OutputDir: dir, Overwrite: true})
	engine.ffmpeg = script

	if err := engine.muxDASH(videoPath, "", outPath, false); err != nil {
		t.Fatalf("muxDASH returned error: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("merged output missing: %v", err)
	}
	if _, err := os.Stat(outPath + ".part"); !os.IsNotExist(err) {
		t.Fatalf("part file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestMuxDASHCleansPartOnFailure(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "video.mp4")
	outPath := filepath.Join(dir, "merged.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	script := writeFFmpegStub(t, `for last do :; done
echo partial > "$last"
echo ffmpeg failed >&2
exit 1`)
	engine := New(Opts{OutputDir: dir, Overwrite: true})
	engine.ffmpeg = script

	_ = captureStderr(t, func() {
		if err := engine.muxDASH(videoPath, "", outPath, false); err == nil {
			t.Fatal("muxDASH returned nil error")
		}
	})
	if _, err := os.Stat(outPath + ".part"); !os.IsNotExist(err) {
		t.Fatalf("part file still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("final output exists or stat failed unexpectedly: %v", err)
	}
}

func writeFFmpegStub(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ffmpeg-stub.sh")
	content := "#!/bin/sh\nset -e\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return path
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = old
	}()
	defer r.Close()
	defer w.Close()

	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(data)
}
