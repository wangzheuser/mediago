package download

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

)

//go:embed cctv_h5e_decrypt.js
var cctvH5eDecryptJS []byte

const cctvWorkerURL = "https://js.player.cntv.cn/creator/live.worker.js"

type cctvH5eJob struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

func (e *Engine) decryptCCTVH5E(ctx context.Context, segmentPaths []string) error {
	nodeExe, err := findNode()
	if err != nil {
		return fmt.Errorf("cctv h5e decrypt requires Node.js: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "mediago-cctv-h5e-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	decoderPath := filepath.Join(tmpDir, "cctv_h5e_decrypt.js")
	if err := os.WriteFile(decoderPath, cctvH5eDecryptJS, 0o644); err != nil {
		return err
	}

	workerPath, err := e.ensureCCTVWorker(ctx, tmpDir)
	if err != nil {
		return err
	}

	jobs := make([]cctvH5eJob, 0, len(segmentPaths))
	for _, p := range segmentPaths {
		jobs = append(jobs, cctvH5eJob{Input: p, Output: p})
	}
	jobJSON, err := json.Marshal(jobs)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, nodeExe, "--stack-size=65536", decoderPath, workerPath)
	cmd.Stdin = strings.NewReader(string(jobJSON))
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cctv h5e decrypt failed: %w\n%s", err, output)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		var result struct {
			OK    bool   `json:"ok"`
			File  string `json:"file"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue
		}
		if !result.OK {
			return fmt.Errorf("cctv h5e decrypt failed for %s: %s", result.File, result.Error)
		}
	}

	return nil
}

func (e *Engine) ensureCCTVWorker(ctx context.Context, tmpDir string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = tmpDir
	}
	cachedPath := filepath.Join(cacheDir, "mediago", "cctv_live_worker.js")

	if info, err := os.Stat(cachedPath); err == nil && info.Size() > 1000000 {
		return cachedPath, nil
	}

	os.MkdirAll(filepath.Dir(cachedPath), 0o755)
	body, err := e.client.GetBytes(cctvWorkerURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to download CCTV worker.js: %w", err)
	}
	if len(body) < 1000000 {
		return "", fmt.Errorf("CCTV worker.js too small (%d bytes)", len(body))
	}

	if err := os.WriteFile(cachedPath, body, 0o644); err != nil {
		fallback := filepath.Join(tmpDir, "live.worker.js")
		if err := os.WriteFile(fallback, body, 0o644); err != nil {
			return "", err
		}
		return fallback, nil
	}
	return cachedPath, nil
}

func findNode() (string, error) {
	if p, err := exec.LookPath("node"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("nodejs"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("node/nodejs not found in PATH")
}
