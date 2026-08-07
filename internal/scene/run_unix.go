//go:build darwin || linux

package scene

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chouheiwa/articale-to-motion/internal/config"
	"github.com/chouheiwa/articale-to-motion/internal/tools"
)

const terminateGrace = 500 * time.Millisecond

func resolveBinary(name, pathValue string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		return name, nil
	}
	for _, dir := range filepath.SplitList(pathValue) {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("缺少必需工具：%s", name)
}

func environmentList(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func Run(ctx context.Context, s Scene, cfg config.Config, unsafe bool, baseEnv map[string]string, userOutput io.Writer, tolerance float64) error {
	if !isFinite(tolerance) || tolerance < 0 {
		return fmt.Errorf("tolerance 必须是有限且非负的数字")
	}
	renderer := s.Renderer
	if renderer == "" {
		renderer = cfg.Renderer
	}
	skillsDir, err := ResolveSkillsDir(renderer, s.Directory, SkillsEnvironment(baseEnv, cfg.Overlay))
	if err != nil {
		return err
	}
	if skillsDir == "" {
		fmt.Fprintf(userOutput, "警告：未能为渲染工具 %s 定位 %s 技能目录，动效要求改用技能名兜底；可运行 am init 安装，或用 %s 显式指定\n", renderer, AnimationSkillName, SkillsDirEnv)
	}
	prompt, err := BuildPrompt(s, skillsDir)
	if err != nil {
		return err
	}
	argv, err := tools.RendererInvocation(renderer, prompt, unsafe)
	if err != nil {
		return err
	}
	binary, err := resolveBinary(argv[0], baseEnv["PATH"])
	if err != nil {
		return err
	}
	argv[0] = binary
	rawFile, err := os.Create(s.StreamLog())
	if err != nil {
		return err
	}
	defer rawFile.Close()
	stderrFile, err := os.Create(s.StderrLog())
	if err != nil {
		return err
	}
	defer stderrFile.Close()
	userFile, err := os.Create(s.UserLog())
	if err != nil {
		return err
	}
	defer userFile.Close()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = s.Directory
	passthrough := strings.FieldsFunc(baseEnv["AM_PASSTHROUGH_ENV"], func(r rune) bool { return r == ',' || r == ' ' })
	cmd.Env = environmentList(cfg.ChildEnvironment(baseEnv, unsafe, passthrough))
	cmd.Stdin = nil
	cmd.Stderr = stderrFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动渲染器：%w", err)
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			timer := time.NewTimer(terminateGrace)
			select {
			case <-done:
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	}()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(rawFile, line)
		for _, message := range tools.ExtractUserMessages(renderer, line) {
			fmt.Fprintln(userFile, message)
			fmt.Fprintln(userOutput, message)
		}
	}
	waitErr := cmd.Wait()
	close(done)
	if ctx.Err() != nil {
		return fmt.Errorf("已中断：渲染器进程组已终止")
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取渲染器输出：%w", err)
	}
	if waitErr != nil {
		if exit, ok := waitErr.(*exec.ExitError); ok {
			return fmt.Errorf("渲染器退出码 %d", exit.ExitCode())
		}
		return waitErr
	}
	return VerifyOutput(s, baseEnv, tolerance)
}

func VerifyOutput(s Scene, baseEnv map[string]string, tolerance float64) error {
	if math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance < 0 {
		return fmt.Errorf("tolerance 必须是有限且非负的数字")
	}
	path := s.OutputPath()
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("渲染结束但产物不存在：%s", s.Output)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("渲染产物不是非空普通文件：%s", s.Output)
	}
	root, _ := filepath.EvalSymlinks(s.Directory)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("产物不得链接到镜头目录外：%s", s.Output)
	}
	ffprobe, err := resolveBinary("ffprobe", baseEnv["PATH"])
	if err != nil {
		return err
	}
	probe := exec.Command(ffprobe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	probe.Env = environmentList(baseEnv)
	output, err := probe.CombinedOutput()
	if err != nil {
		return fmt.Errorf("无法解码产物：%s（%s）", path, strings.TrimSpace(string(output)))
	}
	actual, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || !isFinite(actual) {
		return fmt.Errorf("ffprobe 未返回有效时长：%s", path)
	}
	if math.Abs(actual-s.DurationSeconds) > tolerance {
		return fmt.Errorf("产物时长不符：期望 %.3f 秒，实测 %.3f 秒", s.DurationSeconds, actual)
	}
	return nil
}
