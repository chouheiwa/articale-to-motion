package archive

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Plan struct {
	Repository   string
	MainRef      string
	MainCommit   string
	SourceRef    string
	SourceCommit string
	Destination  string
	Candidates   []string
	Shared       []string
	Ignored      []string
}

func gitOutput(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

func splitNUL(body []byte) []string {
	parts := bytes.Split(body, []byte{0})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			result = append(result, string(part))
		}
	}
	return result
}

func existsAt(root, commit, path string) bool {
	cmd := exec.Command("git", "cat-file", "-e", commit+":"+path)
	cmd.Dir = root
	return cmd.Run() == nil
}

func BuildPlan(root, mainRef, archiveRoot, name string) (Plan, error) {
	top, err := gitOutput(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return Plan{}, err
	}
	root = strings.TrimSpace(string(top))
	mainCommitRaw, err := gitOutput(root, "rev-parse", "--verify", mainRef+"^{commit}")
	if err != nil {
		return Plan{}, err
	}
	sourceCommitRaw, err := gitOutput(root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return Plan{}, err
	}
	sourceRefRaw, _ := gitOutput(root, "branch", "--show-current")
	if archiveRoot == "" {
		archiveRoot = filepath.Join(filepath.Dir(root), "_archive")
	}
	if !filepath.IsAbs(archiveRoot) {
		archiveRoot = filepath.Join(root, archiveRoot)
	}
	archiveRoot, _ = filepath.Abs(archiveRoot)
	if archiveRoot == root || strings.HasPrefix(archiveRoot, root+string(filepath.Separator)) {
		return Plan{}, fmt.Errorf("归档目录不得位于仓库内部：%s", archiveRoot)
	}
	if name == "" {
		name = strings.ReplaceAll(strings.TrimSpace(string(sourceRefRaw)), "/", "-")
		if name == "" {
			name = "detached-project"
		}
	}
	if name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, "\r\n\x00") {
		return Plan{}, fmt.Errorf("归档目录名不得包含路径分隔符或控制字符：%q", name)
	}
	plan := Plan{Repository: root, MainRef: mainRef, MainCommit: strings.TrimSpace(string(mainCommitRaw)), SourceRef: strings.TrimSpace(string(sourceRefRaw)), SourceCommit: strings.TrimSpace(string(sourceCommitRaw)), Destination: filepath.Join(archiveRoot, name)}
	diff, err := gitOutput(root, "diff", "--name-status", "-z", plan.MainCommit, "--")
	if err != nil {
		return Plan{}, err
	}
	records := splitNUL(diff)
	for i := 0; i < len(records); {
		status := records[i]
		i++
		count := 1
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			count = 2
		}
		for n := 0; n < count && i < len(records); n, i = n+1, i+1 {
			path := records[i]
			if existsAt(root, plan.MainCommit, path) {
				plan.Shared = append(plan.Shared, path)
			} else if _, err := os.Lstat(filepath.Join(root, path)); err == nil {
				plan.Candidates = append(plan.Candidates, path)
			}
		}
	}
	untracked, _ := gitOutput(root, "ls-files", "-z", "--others", "--exclude-standard")
	plan.Candidates = append(plan.Candidates, splitNUL(untracked)...)
	ignored, _ := gitOutput(root, "ls-files", "-z", "--others", "--ignored", "--exclude-standard")
	plan.Ignored = splitNUL(ignored)
	plan.Candidates = uniqueSorted(plan.Candidates)
	plan.Shared = uniqueSorted(plan.Shared)
	plan.Ignored = uniqueSorted(plan.Ignored)
	return plan, nil
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

type manifestEntry struct {
	Path          string `json:"path"`
	SHA256        string `json:"sha256,omitempty"`
	SymlinkTarget string `json:"symlink_target,omitempty"`
}

func Execute(plan Plan) error {
	if len(plan.Shared) > 0 {
		return fmt.Errorf("main 共享文件有修改，拒绝归档：%s", strings.Join(plan.Shared, ", "))
	}
	if len(plan.Ignored) > 0 {
		return fmt.Errorf("存在 ignored 文件，拒绝归档：%s", strings.Join(plan.Ignored, ", "))
	}
	if _, err := os.Stat(plan.Destination); !os.IsNotExist(err) {
		return fmt.Errorf("归档目标已存在：%s", plan.Destination)
	}
	payload := filepath.Join(plan.Destination, "files")
	if err := os.MkdirAll(payload, 0o755); err != nil {
		return err
	}
	var moved []string
	rollback := func() {
		for i := len(moved) - 1; i >= 0; i-- {
			from := filepath.Join(payload, moved[i])
			to := filepath.Join(plan.Repository, moved[i])
			_ = os.MkdirAll(filepath.Dir(to), 0o755)
			_ = os.Rename(from, to)
		}
		_ = os.RemoveAll(plan.Destination)
	}
	for _, relative := range plan.Candidates {
		from := filepath.Join(plan.Repository, relative)
		to := filepath.Join(payload, relative)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			rollback()
			return err
		}
		if err := os.Rename(from, to); err != nil {
			rollback()
			return fmt.Errorf("移动 %s: %w", relative, err)
		}
		moved = append(moved, relative)
	}
	entries := make([]manifestEntry, 0, len(moved))
	for _, relative := range moved {
		path := filepath.Join(payload, relative)
		info, err := os.Lstat(path)
		if err != nil {
			rollback()
			return err
		}
		entry := manifestEntry{Path: relative}
		if info.Mode()&os.ModeSymlink != 0 {
			entry.SymlinkTarget, _ = os.Readlink(path)
		} else {
			body, err := os.ReadFile(path)
			if err != nil {
				rollback()
				return err
			}
			sum := sha256.Sum256(body)
			entry.SHA256 = hex.EncodeToString(sum[:])
		}
		entries = append(entries, entry)
	}
	manifest := map[string]any{"schema_version": 1, "source_ref": plan.SourceRef, "source_commit": plan.SourceCommit, "main_ref": plan.MainRef, "main_commit": plan.MainCommit, "moved_count": len(moved), "files": entries}
	body, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(plan.Destination, "MANIFEST.json"), append(body, '\n'), 0o644); err != nil {
		rollback()
		return err
	}
	cmd := exec.Command("git", "switch", "--detach", plan.MainCommit)
	cmd.Dir = plan.Repository
	if output, err := cmd.CombinedOutput(); err != nil {
		rollback()
		return fmt.Errorf("复位到 main 失败：%s", strings.TrimSpace(string(output)))
	}
	return nil
}
