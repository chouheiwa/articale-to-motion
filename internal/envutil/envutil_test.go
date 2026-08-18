package envutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvMapIncludesEnvironment(t *testing.T) {
	t.Setenv("AM_TEST_KEY", "hello")
	m := EnvMap()
	if m["AM_TEST_KEY"] != "hello" {
		t.Fatalf("expected AM_TEST_KEY=hello, got %q", m["AM_TEST_KEY"])
	}
}

func TestEnvListRoundTrips(t *testing.T) {
	input := map[string]string{"A": "1", "B": "2"}
	list := EnvList(input)
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	seen := map[string]bool{}
	for _, entry := range list {
		seen[entry] = true
	}
	if !seen["A=1"] || !seen["B=2"] {
		t.Fatalf("unexpected list: %v", list)
	}
}

func TestParsePassthrough(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"USER,LOGNAME", 2},
		{"USER LOGNAME", 2},
		{"USER, LOGNAME", 2},
		{"", 0},
		{"  ", 0},
	}
	for _, tc := range cases {
		got := ParsePassthrough(tc.input)
		if len(got) != tc.want {
			t.Errorf("ParsePassthrough(%q) = %d items, want %d", tc.input, len(got), tc.want)
		}
	}
}

// TestLookPathUsesGivenPathNotProcessPath 锁住「只查传入的 PATH」这条语义。
// 安全模式下子进程的 PATH 是白名单，按父进程 PATH 找到的二进制子进程未必能跑。
func TestLookPathUsesGivenPathNotProcessPath(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	target := filepath.Join(dir, "am-fake-tool")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 进程自身的 PATH 指向能找到该工具的目录，传入的 PATH 却指向别处。
	t.Setenv("PATH", dir)
	if _, err := LookPath("am-fake-tool", other); err == nil {
		t.Error("LookPath 落到了进程自身的 PATH 上：传入的 PATH 里没有这个工具，应当失败")
	}
	got, err := LookPath("am-fake-tool", other+string(filepath.ListSeparator)+dir)
	if err != nil {
		t.Fatalf("PATH 中确实存在该工具却没找到：%v", err)
	}
	if got != target {
		t.Errorf("解析到 %q，期望 %q", got, target)
	}
}

// TestLookPathSkipsNonExecutable 守着「存在同名文件但没有执行位」这个坑：
// 返回它会让调用方拿到 permission denied，而不是清晰的「工具没装」。
func TestLookPathSkipsNonExecutable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "am-fake-tool"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LookPath("am-fake-tool", dir); err == nil {
		t.Error("不可执行的同名文件被当成了工具")
	}
	if err := os.Mkdir(filepath.Join(dir, "am-fake-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LookPath("am-fake-dir", dir); err == nil {
		t.Error("同名目录被当成了工具")
	}
}

// TestLookPathErrorCarriesExitCode127Marker 是跨包契约测试。
// cli.Execute 靠 MissingToolPrefix 这个子串把错误映射成退出码 127；
// 改了措辞而不改 cli.go，调用方就会把「没装 ffmpeg」当成一般失败。
func TestLookPathErrorCarriesExitCode127Marker(t *testing.T) {
	_, err := LookPath("am-definitely-not-installed", t.TempDir())
	if err == nil {
		t.Fatal("不存在的工具应当返回错误")
	}
	if !strings.Contains(err.Error(), MissingToolPrefix) {
		t.Errorf("错误信息缺少退出码 127 的判定前缀 %q：%s", MissingToolPrefix, err)
	}
	if !strings.Contains(err.Error(), "am-definitely-not-installed") {
		t.Errorf("错误信息没有点名缺失的工具：%s", err)
	}
}

// TestLookPathPassesThroughExplicitPaths：名字里带分隔符时交给 exec 自己解析，
// 不去 PATH 里找，也不做存在性检查。
func TestLookPathPassesThroughExplicitPaths(t *testing.T) {
	got, err := LookPath("/usr/bin/env", "")
	if err != nil || got != "/usr/bin/env" {
		t.Errorf("LookPath(%q) = %q, %v；期望原样返回", "/usr/bin/env", got, err)
	}
}

func TestIsUnsafe(t *testing.T) {
	if IsUnsafe(true) != true {
		t.Error("flag=true should be unsafe")
	}
	if IsUnsafe(false) != false {
		t.Error("flag=false without env should be safe")
	}
	t.Setenv("AM_UNSAFE", "1")
	if IsUnsafe(false) != true {
		t.Error("AM_UNSAFE=1 should be unsafe")
	}
}
