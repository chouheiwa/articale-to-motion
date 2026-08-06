package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/chouheiwa/articale-to-motion/internal/scene"
)

type Status string

const (
	Succeeded Status = "succeeded"
	Skipped   Status = "skipped"
	Stale     Status = "stale"
	Failed    Status = "failed"
)

type SceneResult struct {
	SceneID  string  `json:"scene_id"`
	Status   Status  `json:"status"`
	Attempts int     `json:"attempts"`
	Seconds  float64 `json:"seconds"`
	Reason   string  `json:"reason"`
}

type Report struct {
	SchemaVersion int            `json:"schema_version"`
	Seconds       float64        `json:"seconds"`
	Interrupted   bool           `json:"interrupted"`
	Code          int            `json:"exit_code"`
	CountValues   map[Status]int `json:"counts"`
	Scenes        []SceneResult  `json:"scenes"`
}

func (r Report) Counts() map[Status]int { return r.CountValues }
func (r Report) ExitCode() int {
	if r.Interrupted {
		return 130
	}
	if r.CountValues[Failed] > 0 {
		return 1
	}
	return 0
}

func (r Report) WriteJSON(path string) error {
	body, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".am-report-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(body); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (r Report) Render() string {
	return fmt.Sprintf("镜头汇总（%d 个）\n  成功 %d  跳过 %d  过期 %d  失败 %d\n\n耗时 %.1f 秒", len(r.Scenes), r.CountValues[Succeeded], r.CountValues[Skipped], r.CountValues[Stale], r.CountValues[Failed], r.Seconds)
}

func Plan(root string) ([]scene.Scene, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("无法读取镜头目录：%w", err)
	}
	var result []scene.Scene
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "scene.json")); os.IsNotExist(err) {
			continue
		}
		s, err := scene.Load(dir)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool { return filepath.Base(result[i].Directory) < filepath.Base(result[j].Directory) })
	return result, nil
}

type Runner func(context.Context, scene.Scene) error

type outcomeError struct {
	status Status
	reason string
}

func (e *outcomeError) Error() string { return e.reason }

func Outcome(status Status, reason string) error {
	return &outcomeError{status: status, reason: reason}
}

func archiveAttempt(s scene.Scene) {
	attemptsRoot := filepath.Join(s.Directory, "attempts")
	_ = os.MkdirAll(attemptsRoot, 0o755)
	entries, _ := os.ReadDir(attemptsRoot)
	number := len(entries) + 1
	destination := filepath.Join(attemptsRoot, fmt.Sprintf("attempt-%02d", number))
	_ = os.MkdirAll(destination, 0o755)
	for _, path := range []string{s.OutputPath(), s.StreamLog(), s.StderrLog(), s.UserLog()} {
		if _, err := os.Lstat(path); err == nil {
			_ = os.Rename(path, filepath.Join(destination, filepath.Base(path)))
		}
	}
}

func RunAll(ctx context.Context, scenes []scene.Scene, jobs, retries int, runner Runner) Report {
	started := time.Now()
	if jobs < 1 {
		jobs = 1
	}
	tasks := make(chan scene.Scene)
	results := make(chan SceneResult)
	var workers sync.WaitGroup
	worker := func() {
		defer workers.Done()
		for s := range tasks {
			begin := time.Now()
			result := SceneResult{SceneID: s.ID, Status: Failed}
			for attempt := 1; attempt <= retries+1; attempt++ {
				result.Attempts = attempt
				err := runner(ctx, s)
				if err == nil {
					result.Status, result.Reason = Succeeded, ""
					break
				}
				var outcome *outcomeError
				if errors.As(err, &outcome) {
					result.Status, result.Reason = outcome.status, outcome.reason
					result.Attempts = 0
					break
				}
				result.Reason = err.Error()
				if ctx.Err() != nil {
					break
				}
				if attempt <= retries {
					archiveAttempt(s)
				}
			}
			result.Seconds = time.Since(begin).Seconds()
			results <- result
		}
	}
	for i := 0; i < jobs; i++ {
		workers.Add(1)
		go worker()
	}
	go func() {
		defer close(tasks)
		for _, s := range scenes {
			select {
			case tasks <- s:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()
	byID := make(map[string]SceneResult)
	for result := range results {
		byID[result.SceneID] = result
	}
	ordered := make([]SceneResult, 0, len(byID))
	counts := map[Status]int{Succeeded: 0, Skipped: 0, Stale: 0, Failed: 0}
	for _, s := range scenes {
		if result, ok := byID[s.ID]; ok {
			ordered = append(ordered, result)
			counts[result.Status]++
		}
	}
	report := Report{SchemaVersion: 1, Seconds: time.Since(started).Seconds(), Interrupted: ctx.Err() != nil, CountValues: counts, Scenes: ordered}
	report.Code = report.ExitCode()
	return report
}
