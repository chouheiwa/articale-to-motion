package song

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/chouheiwa/articale-to-motion/internal/envutil"
	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
)

func withLock(root string, fn func() error) error {
	p, e := LocalPath(root, Store+"/.operation-lock")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		return e
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return fmt.Errorf("歌曲操作锁已存在；确认没有运行中命令后才可删除 %s", p)
	}
	f.Close()
	defer os.Remove(p)
	return fn()
}
func copyFile(source, dest string) error {
	f, e := os.Open(source)
	if e != nil {
		return e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Size() > 512<<20 {
		return fmt.Errorf("输入必须是小于 512 MiB 的普通文件")
	}
	b, e := io.ReadAll(f)
	if e != nil {
		return e
	}
	return fsutil.AtomicWrite(dest, b, 0644)
}
func finish(root string, c *Candidate, media mediaprobe.Toolchain) error {
	dir, _ := CandidateDir(root, c.ID)
	p := filepath.Join(dir, c.Audio)
	m, e := media.Probe(p)
	if e != nil {
		return e
	}
	if m.Audio == nil || m.Video != nil || !finite(m.DurationSeconds) || m.DurationSeconds <= 0 {
		return fmt.Errorf("候选必须是有时长、无视频流的音频")
	}
	if e = media.Decode(p); e != nil {
		return e
	}
	c.Duration = m.DurationSeconds
	c.AudioSHA, e = HashFile(p)
	if e != nil {
		return e
	}
	c.LyricsSHA, e = HashFile(filepath.Join(dir, c.Lyrics))
	if e != nil {
		return e
	}
	c.State = "ready"
	return WriteJSON(filepath.Join(dir, "candidate.json"), c)
}
func Import(root, audio, lyrics string, media mediaprobe.Toolchain) (Candidate, error) {
	var c Candidate
	if _, e := LoadConfig(root); e != nil {
		return c, e
	}
	e := withLock(root, func() error {
		var e error
		c, e = importCandidate(root, audio, lyrics, media)
		return e
	})
	return c, e
}
func importCandidate(root, audio, lyrics string, media mediaprobe.Toolchain) (Candidate, error) {
	var c Candidate
	b, e := os.ReadFile(lyrics)
	if e != nil {
		return c, e
	}
	if len(LyricLines(string(b))) == 0 {
		return c, fmt.Errorf("歌词为空")
	}
	c = Candidate{ID: fmt.Sprintf("import-%d", time.Now().UnixNano()), Provider: "import", Audio: "audio" + filepath.Ext(audio), Lyrics: "lyrics.txt", State: "importing"}
	dir, e := CandidateDir(root, c.ID)
	if e != nil {
		return c, e
	}
	if e = os.MkdirAll(dir, 0755); e != nil {
		return c, e
	}
	if e = copyFile(audio, filepath.Join(dir, c.Audio)); e != nil {
		return c, e
	}
	if e = fsutil.AtomicWrite(filepath.Join(dir, c.Lyrics), b, 0644); e != nil {
		return c, e
	}
	e = finish(root, &c, media)
	return c, e
}

// Generate reuses deterministic request slots. Failed/ambiguous submissions are
// never automatically resubmitted. A new --batch explicitly authorizes new work.
func Generate(ctx context.Context, root string, count int, batch string, env map[string]string, out io.Writer) error {
	if count < 1 || count > 8 || !idPattern.MatchString(batch) {
		return fmt.Errorf("--count 必须为 1..8，--batch 必须是安全标识")
	}
	c, e := LoadConfig(root)
	if e != nil {
		return e
	}
	if c.Provider == "manual" {
		return Handoff(root, out)
	}
	p, e := LocalPath(root, c.Lyrics)
	if e != nil {
		return e
	}
	lyrics, e := os.ReadFile(p)
	if e != nil {
		return e
	}
	if len(LyricLines(string(lyrics))) == 0 {
		return fmt.Errorf("歌词为空")
	}
	if e = CheckCredential(c, env); e != nil {
		return e
	}
	if e = validateLyrics(c, string(lyrics)); e != nil {
		return e
	}
	media, e := mediaprobe.New(env)
	if e != nil {
		return e
	}
	audioName := "audio.mp3"
	if c.Provider == "fal" {
		audioName = "audio.wav"
	}
	return withLock(root, func() error {
		requestSHA := Digest(struct {
			Config Config
			Lyrics string
		}{c, string(lyrics)})
		for i := 0; i < count; i++ {
			id := fmt.Sprintf("%s-%s-%02d", batch, requestSHA[:16], i+1)
			dir, e := CandidateDir(root, id)
			if e != nil {
				return e
			}
			candidate := Candidate{ID: id, Provider: c.Provider, Model: c.Model, RequestSHA: requestSHA, Audio: audioName, Lyrics: "lyrics.txt", State: "submitting", StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			record, e := LocalPath(dir, "candidate.json")
			if e != nil {
				return e
			}
			for _, name := range []string{"lyrics.txt", "request.json", audioName, "provider-error.json"} {
				if _, e = LocalPath(dir, name); e != nil {
					return e
				}
			}
			if _, e = os.Stat(record); e == nil {
				if e = ReadJSON(record, &candidate); e != nil {
					return e
				}
				if candidate.RequestSHA != requestSHA || candidate.ID != id || candidate.Provider != c.Provider || candidate.Model != c.Model || candidate.Audio != audioName || candidate.Lyrics != "lyrics.txt" {
					return fmt.Errorf("请求摘要冲突")
				}
				if candidate.State == "ready" {
					if _, e = LoadCandidate(root, id); e != nil {
						return e
					}
					fmt.Fprintf(out, "复用候选 %s：%s\n", id, filepath.Join(dir, candidate.Audio))
					continue
				}
				if candidate.State == "downloaded" {
					if e = finish(root, &candidate, media); e != nil {
						return e
					}
					fmt.Fprintf(out, "已验证本地候选 %s：%s\n", id, filepath.Join(dir, candidate.Audio))
					continue
				}
				if candidate.TaskID == "" || (c.Provider != "acestep" && c.Provider != "fal" && c.Provider != "mureka") || candidate.State != "pending" {
					return fmt.Errorf("候选 %s 提交状态不明或曾失败，未重发；检查服务后用新 --batch 显式生成", id)
				}
			} else if !os.IsNotExist(e) {
				return e
			} else {
				if e = fsutil.AtomicWrite(filepath.Join(dir, candidate.Lyrics), lyrics, 0644); e != nil {
					return e
				}
				if e = WriteJSON(filepath.Join(dir, "request.json"), map[string]any{"config": c, "lyrics_sha256": BytesHash(lyrics), "request_sha256": requestSHA}); e != nil {
					return e
				}
				if e = WriteJSON(record, candidate); e != nil {
					return e
				}
			}
			if candidate.TaskID != "" {
				actual, e := HashFile(filepath.Join(dir, candidate.Lyrics))
				if e != nil {
					return e
				}
				if actual != BytesHash(lyrics) {
					return fmt.Errorf("待完成候选歌词摘要变化，拒绝恢复")
				}
			}
			attemptStart := time.Now()
			fmt.Fprintf(out, "正在处理候选 %s（%s / %s）\n", id, c.Provider, c.Model)
			dest := filepath.Join(dir, candidate.Audio)
			if c.Provider == "minimax" {
				binary, e := envutil.LookPath("mmx", env["PATH"])
				if e != nil {
					return e
				}
				args := []string{"music", "generate", "--non-interactive", "--output", "json", "--model", c.Model, "--prompt", fmt.Sprintf("%s; language %s; target duration %.0f seconds", c.Style, c.Language, c.TargetSeconds), "--bpm", strconv.Itoa(c.BPM), "--lyrics-file", filepath.Join(dir, candidate.Lyrics), "--out", dest}
				cmd := exec.CommandContext(ctx, binary, args...)
				cmd.Env = envutil.EnvList(env)
				var stdout, stderr diagnosticBuffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				// Persist only structured, redacted errors; never raw provider output.
				if e = cmd.Run(); e != nil {
					candidate.State = "submission_unknown"
					candidate.ElapsedSeconds += time.Since(attemptStart).Seconds()
					if exit, ok := e.(*exec.ExitError); ok {
						candidate.ProviderExitCode = exit.ExitCode()
					}
					if writeErr := WriteJSON(record, candidate); writeErr != nil {
						return writeErr
					}
					detail := providerDiagnostic(stderr.data, env)
					if detail.Code == 0 {
						alternative := providerDiagnostic(stdout.data, env)
						if alternative.Code != 0 {
							detail = alternative
						}
					}
					if writeErr := WriteJSON(filepath.Join(dir, "provider-error.json"), detail); writeErr != nil {
						return writeErr
					}
					return fmt.Errorf("MiniMax 生成未完成（候选 %s，退出码 %d）：%s；未自动重试", id, candidate.ProviderExitCode, detail.Message)
				}
			} else if c.Provider == "bailian" || c.Provider == "elevenlabs" || c.Provider == "lyria" {
				task, err := newHosted(c, env).Generate(ctx, string(lyrics), dest)
				candidate.TaskID = task
				if err != nil {
					candidate.State = "submission_unknown"
					candidate.ElapsedSeconds += time.Since(attemptStart).Seconds()
					if e = WriteJSON(record, candidate); e != nil {
						return e
					}
					return err
				}
			} else {
				var a interface {
					Submit(context.Context, Config, string) (string, error)
					Resume(context.Context, string, string) error
				}
				if c.Provider == "fal" || c.Provider == "mureka" {
					a = newHosted(c, env)
				} else {
					a = ACEClient{Endpoint: c.Endpoint, Key: env["ACESTEP_API_KEY"]}
				}
				if candidate.TaskID == "" {
					task, e := a.Submit(ctx, c, string(lyrics))
					if e != nil {
						candidate.State = "submission_unknown"
						candidate.ElapsedSeconds += time.Since(attemptStart).Seconds()
						if writeErr := WriteJSON(record, candidate); writeErr != nil {
							return writeErr
						}
						return e
					}
					candidate.TaskID = task
					candidate.State = "pending"
					if e = WriteJSON(record, candidate); e != nil {
						return e
					}
				}
				if e = a.Resume(ctx, candidate.TaskID, dest); e != nil {
					candidate.ElapsedSeconds += time.Since(attemptStart).Seconds()
					if writeErr := WriteJSON(record, candidate); writeErr != nil {
						return writeErr
					}
					return fmt.Errorf("候选 %s 可通过相同 generate 命令继续轮询：%w", id, e)
				}
			}
			candidate.ElapsedSeconds += time.Since(attemptStart).Seconds()
			candidate.State = "downloaded"
			if e = WriteJSON(record, candidate); e != nil {
				return e
			}
			if e = finish(root, &candidate, media); e != nil {
				return e
			}
			fmt.Fprintf(out, "候选 %s：%s\n", id, dest)
		}
		fmt.Fprintln(out, "请试听后由用户运行 am song select <candidate-id>。未自动选歌。")
		return nil
	})
}
