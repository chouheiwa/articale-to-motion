package song

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
	"gopkg.in/yaml.v3"
)

func fakeEnvironment(t *testing.T) (string, map[string]string) {
	t.Helper()
	root := t.TempDir()
	bin := t.TempDir()
	scripts := map[string]string{
		"ffprobe": `#!/bin/sh
printf '%s' '{"format":{"duration":"10"},"streams":[{"codec_type":"audio","sample_rate":"48000","channels":2}]}'
`,
		"ffmpeg": `#!/bin/sh
for arg do last="$arg"; done
case "$last" in *.wav) printf 'wav' > "$last";; esac
`,
		"mmx": `#!/bin/sh
case "$*" in *--help*) echo --lyrics-file; exit 0;; esac
while [ "$#" -gt 0 ]; do
 if [ "$1" = --out ]; then shift; printf 'audio' > "$1"; exit 0; fi
 shift
done
exit 1
`,
		"python3": `#!/bin/sh
case "$*" in
 *beat_track*) echo '[1,2,3]'; exit 0;;
 *' align '*)
 while [ "$#" -gt 0 ]; do
 if [ "$1" = --output-dir ]; then shift; out="$1"; fi
 shift
 done
 printf '%s' '{"lines":[{"index":0,"text":"歌词","start":1,"end":8,"status":"aligned","tokens":[{"text":"歌词","start":1,"end":8}]}]}' > "$out/alignment.json"
 printf '{}' > "$out/report.json"
 exit 0;;
 *) exit 0;;
esac
`,
	}
	for name, body := range scripts {
		if e := os.WriteFile(filepath.Join(bin, name), []byte(body), 0755); e != nil {
			t.Fatal(e)
		}
	}
	os.WriteFile(filepath.Join(root, "song.yaml"), []byte(DefaultConfig), 0644)
	os.WriteFile(filepath.Join(root, "lyrics.txt"), []byte("[Verse]\n歌词\n"), 0644)
	return root, map[string]string{"PATH": bin, "HOME": t.TempDir()}
}
func TestMiniMaxPrepareAndCueLifecycle(t *testing.T) {
	root, env := fakeEnvironment(t)
	ctx := context.Background()
	var out bytes.Buffer
	if e := Doctor(ctx, root, env, &out); e != nil {
		t.Fatal(e)
	}
	if e := Generate(ctx, root, 2, "first", env, &out); e != nil {
		t.Fatal(e)
	}
	if e := Generate(ctx, root, 2, "first", env, &out); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), "复用候选") {
		t.Fatal(out.String())
	}
	dirs, _ := os.ReadDir(filepath.Join(root, Store, "candidates"))
	if len(dirs) != 2 {
		t.Fatal(dirs)
	}
	if e := Prepare(ctx, root, "", false, env, &out); e == nil {
		t.Fatal("selection required")
	}
	if e := Select(root, dirs[0].Name()); e != nil {
		t.Fatal(e)
	}
	if e := Prepare(ctx, root, "", false, env, &out); e != nil {
		t.Fatal(e)
	}
	tl, e := LoadTimeline(root)
	if e != nil || tl.Reviewed || len(tl.Beats) != 3 {
		t.Fatal(tl, e)
	}
	if e = Prepare(ctx, root, filepath.Join(root, Store, "timeline.json"), true, env, &out); e != nil {
		t.Fatal(e)
	}
	tl, _ = LoadTimeline(root)
	if e = ExportPhrases(root, 30); e != nil {
		t.Fatal(e)
	}
	var phrases map[string]any
	if e = ReadJSON(filepath.Join(root, "production", "phrase-timeline.json"), &phrases); e != nil {
		t.Fatal(e)
	}
	if phrases["total_frames"] != float64(300) {
		t.Fatal(phrases)
	}
	dir := filepath.Join(root, "scenes", "001")
	os.MkdirAll(dir, 0755)
	data := tl.Slice(0, 10)
	if e = WriteJSON(filepath.Join(dir, "song-cues.json"), data); e != nil {
		t.Fatal(e)
	}
	sha, _ := HashFile(filepath.Join(root, Store, "timeline.json"))
	dh, _ := HashFile(filepath.Join(dir, "song-cues.json"))
	cue := Cue{FPS: 30, Timeline: "../../production/song/timeline.json", SHA: sha, Start: 0, Data: "song-cues.json", DataSHA: dh}
	if e = VerifyCue(dir, cue, 10); e != nil {
		t.Fatal(e)
	}
	bad := cue
	bad.SHA = "old"
	if VerifyCue(dir, bad, 10) == nil {
		t.Fatal("stale global")
	}
	bad = cue
	bad.DataSHA = "old"
	if VerifyCue(dir, bad, 10) == nil {
		t.Fatal("stale local")
	}
	bad = cue
	bad.Start = -1
	if VerifyCue(dir, bad, 10) == nil {
		t.Fatal("bad start")
	}
	data.Lines[0].Start = 0
	WriteJSON(filepath.Join(dir, cue.Data), data)
	cue.DataSHA, _ = HashFile(filepath.Join(dir, cue.Data))
	if VerifyCue(dir, cue, 10) == nil {
		t.Fatal("tampered local timing")
	}
	if e = Select(root, dirs[1].Name()); e != nil {
		t.Fatal(e)
	}
	if _, e = LoadTimeline(root); e == nil {
		t.Fatal("new candidate must invalidate timeline even for identical audio")
	}
}
func TestImportAndErrors(t *testing.T) {
	root, env := fakeEnvironment(t)
	media, _ := mediaprobe.New(env)
	source := filepath.Join(t.TempDir(), "song.mp3")
	os.WriteFile(source, []byte("audio"), 0644)
	c, e := Import(root, source, filepath.Join(root, "lyrics.txt"), media)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = LoadCandidate(root, c.ID); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(source, []byte("other"), 0644)
	if _, e = LoadCandidate(root, c.ID); e != nil {
		t.Fatal("import didn't copy", e)
	}
	if _, e = Import(root, "missing", filepath.Join(root, "lyrics.txt"), media); e == nil {
		t.Fatal("missing source")
	}
	os.WriteFile(filepath.Join(root, "empty.txt"), nil, 0644)
	if _, e = Import(root, source, filepath.Join(root, "empty.txt"), media); e == nil {
		t.Fatal("empty lyrics")
	}
	if e = withLock(root, func() error { return withLock(root, func() error { return nil }) }); e == nil {
		t.Fatal("concurrent mutation")
	}
	if e = Generate(context.Background(), root, 0, "x", env, &bytes.Buffer{}); e == nil {
		t.Fatal("count")
	}
	if e = Generate(context.Background(), root, 1, "../x", env, &bytes.Buffer{}); e == nil {
		t.Fatal("batch")
	}
	os.WriteFile(filepath.Join(env["PATH"], "mmx"), []byte("#!/bin/sh\nexit 1\n"), 0755)
	if e = Generate(context.Background(), root, 1, "failed", env, &bytes.Buffer{}); e == nil {
		t.Fatal("provider failure")
	}
	if e = Generate(context.Background(), root, 1, "failed", env, &bytes.Buffer{}); e == nil || !strings.Contains(e.Error(), "状态不明") {
		t.Fatal(e)
	}
}
func TestACEGenerationResumesWithoutResubmitting(t *testing.T) {
	root, env := fakeEnvironment(t)
	submits := 0
	queries := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.Write([]byte("{}"))
		case "/release_task":
			submits++
			w.Write([]byte(`{"code":200,"data":{"task_id":"task"}}`))
		case "/query_result":
			queries++
			if queries == 1 {
				w.WriteHeader(503)
				return
			}
			w.Write([]byte(`{"code":200,"data":[{"task_id":"task","status":1,"result":"[{\"file\":\"/audio\"}]"}]}`))
		case "/audio":
			w.Write([]byte("audio"))
		}
	}))
	defer server.Close()
	cfg, _ := LoadConfig(root)
	cfg.Provider = "acestep"
	cfg.Model = ""
	cfg.Endpoint = server.URL
	b, _ := yaml.Marshal(cfg)
	os.WriteFile(filepath.Join(root, "song.yaml"), b, 0644)
	if e := Doctor(context.Background(), root, env, &bytes.Buffer{}); e != nil {
		t.Fatal(e)
	}
	if e := Generate(context.Background(), root, 1, "resume", env, &bytes.Buffer{}); e == nil {
		t.Fatal("poll failure")
	}
	if e := Generate(context.Background(), root, 1, "resume", env, &bytes.Buffer{}); e != nil {
		t.Fatal(e)
	}
	if submits != 1 || queries != 2 {
		t.Fatal(submits, queries)
	}
}
func TestPrepareReportsAndDegrades(t *testing.T) {
	root, env := fakeEnvironment(t)
	var out bytes.Buffer
	ctx := context.Background()
	Generate(ctx, root, 1, "ok", env, &out)
	dirs, _ := os.ReadDir(filepath.Join(root, Store, "candidates"))
	Select(root, dirs[0].Name())
	if e := Prepare(ctx, root, "", true, env, &out); e == nil {
		t.Fatal("review on fresh automatic alignment")
	}
	script := filepath.Join(env["PATH"], "python3")
	body, _ := os.ReadFile(script)
	body = bytes.ReplaceAll(body, []byte("echo '[1,2,3]'; exit 0"), []byte("exit 1"))
	os.WriteFile(script, body, 0755)
	if e := Prepare(ctx, root, "", false, env, &out); e != nil {
		t.Fatal(e)
	}
	tl, _ := LoadTimeline(root)
	if len(tl.Warnings) == 0 || len(tl.Beats) != 0 {
		t.Fatal(tl)
	}
	body = bytes.ReplaceAll(body, []byte(`"start":1,"end":8,"status":"aligned"`), []byte(`"start":null,"end":8,"status":"missing_timestamps"`))
	os.WriteFile(script, body, 0755)
	if e := Prepare(ctx, root, "", false, env, &out); e == nil || !strings.Contains(e.Error(), "人工修正") {
		t.Fatal(e)
	}
	reports, _ := filepath.Glob(filepath.Join(root, Store, "alignment", "*", "issues.json"))
	if len(reports) != 2 {
		t.Fatal(reports)
	}
	os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0755)
	if e := Prepare(ctx, root, "", false, env, &out); e == nil {
		t.Fatal("version mismatch")
	}
	if e := Doctor(ctx, root, env, &out); e == nil {
		t.Fatal("doctor failure")
	}
}
func TestBoundaryAndAtomicJSON(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(root, "link"))
	for _, p := range []string{"", "../x", "/absolute", "link/a"} {
		if _, e := LocalPath(root, p); e == nil {
			t.Fatal(p)
		}
	}
	if e := WriteJSON(filepath.Join(root, "x.json"), map[string]int{"a": 1}); e != nil {
		t.Fatal(e)
	}
	before, _ := os.Stat(filepath.Join(root, "x.json"))
	time.Sleep(time.Millisecond)
	WriteJSON(filepath.Join(root, "x.json"), map[string]int{"a": 1})
	after, _ := os.Stat(filepath.Join(root, "x.json"))
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("unchanged state touched")
	}
	for _, b := range []string{`{"extra":1}`, `{} {}`, `not json`} {
		os.WriteFile(filepath.Join(root, "bad"), []byte(b), 0644)
		var c Candidate
		if e := ReadJSON(filepath.Join(root, "bad"), &c); e == nil {
			t.Fatal(b)
		}
	}
	if BytesHash([]byte("a")) == BytesHash([]byte("b")) {
		t.Fatal("hash")
	}
	if Enabled(root) {
		t.Fatal("mode detection")
	}
}
func TestACEPollingStates(t *testing.T) {
	for _, status := range []int{2, 3} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": []any{map[string]any{"task_id": "id", "status": status}}})
			}))
			defer srv.Close()
			if e := (ACEClient{Endpoint: srv.URL}).Resume(context.Background(), "id", filepath.Join(t.TempDir(), "a")); e == nil {
				t.Fatal(status)
			}
		})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":200,"data":[{"task_id":"id","status":0}]}`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if e := (ACEClient{Endpoint: srv.URL, PollInterval: time.Millisecond}).Resume(ctx, "id", "unused"); e == nil {
		t.Fatal("timeout")
	}
}

func TestDownloadedMiniMaxCandidateIsVerifiedWithoutNewRequest(t *testing.T) {
	root, env := fakeEnvironment(t)
	probe := filepath.Join(env["PATH"], "ffprobe")
	working, _ := os.ReadFile(probe)
	os.WriteFile(probe, []byte("#!/bin/sh\nexit 1\n"), 0755)
	var out bytes.Buffer
	if e := Generate(context.Background(), root, 1, "verify", env, &out); e == nil {
		t.Fatal("probe failure")
	}
	os.WriteFile(probe, working, 0755)
	// Any attempt to re-call the provider now fails.
	os.WriteFile(filepath.Join(env["PATH"], "mmx"), []byte("#!/bin/sh\nexit 1\n"), 0755)
	if e := Generate(context.Background(), root, 1, "verify", env, &out); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), "已验证本地候选") {
		t.Fatal(out.String())
	}
}
