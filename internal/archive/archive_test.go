package archive

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func repository(t *testing.T) string {
	dir := t.TempDir()
	git(t, dir, "init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("base"), 0o644)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "base")
	git(t, dir, "switch", "-c", "video/test")
	return dir
}

func TestPlanSeparatesCandidatesFromSharedChanges(t *testing.T) {
	dir := repository(t)
	os.WriteFile(filepath.Join(dir, "scene.json"), []byte("scene"), 0o644)
	plan, err := BuildPlan(dir, "main", filepath.Join(t.TempDir(), "archives"), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Candidates) != 1 || plan.Candidates[0] != "scene.json" || len(plan.Shared) != 0 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed"), 0o644)
	plan, _ = BuildPlan(dir, "main", filepath.Join(t.TempDir(), "archives"), "test2")
	if len(plan.Shared) != 1 || plan.Shared[0] != "README.md" {
		t.Fatalf("shared change not detected: %+v", plan)
	}
}

func TestExecuteMovesCandidatesAndDetachesAtMain(t *testing.T) {
	dir := repository(t)
	os.Mkdir(filepath.Join(dir, "scenes"), 0o755)
	os.WriteFile(filepath.Join(dir, "scenes", "one.txt"), []byte("payload"), 0o644)
	archiveRoot := filepath.Join(t.TempDir(), "archives")
	plan, err := BuildPlan(dir, "main", archiveRoot, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := Execute(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(plan.Destination, "files", "scenes", "one.txt")); err != nil {
		t.Fatal(err)
	}
	if got := git(t, dir, "branch", "--show-current"); got != "" {
		t.Fatalf("expected detached HEAD, got %q", got)
	}
}

func TestExecuteRefusesSharedOrIgnoredFiles(t *testing.T) {
	dir := repository(t)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed"), 0o644)
	plan, _ := BuildPlan(dir, "main", filepath.Join(t.TempDir(), "archives"), "test")
	if err := Execute(plan); err == nil {
		t.Fatal("expected shared blocker")
	}
}

func TestBuildPlanRejectsArchiveNameTraversal(t *testing.T) {
	dir := repository(t)
	if _, err := BuildPlan(dir, "main", filepath.Join(t.TempDir(), "archives"), "../escape"); err == nil {
		t.Fatal("expected archive name traversal rejection")
	}
}
