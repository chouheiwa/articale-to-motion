package song

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mockResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func TestHostedProviders(t *testing.T) {
	for _, id := range []string{"bailian", "elevenlabs", "fal"} {
		t.Run(id, func(t *testing.T) {
			c := Config{Provider: id, Model: providerInfo(id).Model, Workspace: "workspace", Style: "Mandarin rap", BPM: 100, TargetSeconds: 180}
			h := newHosted(c, map[string]string{providerInfo(id).Key: "secret-test"})
			h.interval = time.Millisecond
			lyrics := "科学知识唱起来\n水汽遇冷凝成云"
			posts, downloads := 0, 0
			h.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "storage.googleapis.com" || strings.HasSuffix(r.URL.Host, ".oss-cn-beijing.aliyuncs.com") {
					downloads++
					if r.Header.Get("Authorization") != "" || r.Header.Get("xi-api-key") != "" {
						t.Fatal("key sent to download host")
					}
					if r.URL.Scheme != "https" {
						t.Fatal("not TLS")
					}
					return mockResponse("audio-bytes"), nil
				}
				if id == "elevenlabs" {
					if r.Header.Get("xi-api-key") != "secret-test" {
						t.Fatal("missing ElevenLabs key")
					}
				} else {
					want := "Bearer secret-test"
					if id == "fal" {
						want = "Key secret-test"
					}
					if r.Header.Get("Authorization") != want {
						t.Fatal("wrong auth")
					}
				}
				if r.Method == "POST" {
					posts++
					var b map[string]any
					if e := json.NewDecoder(r.Body).Decode(&b); e != nil {
						t.Fatal(e)
					}
					switch id {
					case "bailian":
						if r.URL.Path != "/api/v1/services/audio/music/generation" {
							t.Fatal(r.URL.Path)
						}
						input := b["input"].(map[string]any)
						if input["lyrics"] != lyrics || input["prompt"] != nil || input["is_instrumental"] != false {
							t.Fatal(b)
						}
						return mockResponse(`{"request_id":"req-1","output":{"audio":{"url":"http://result.oss-cn-beijing.aliyuncs.com/song.mp3?signature=secret"}}}`), nil
					case "elevenlabs":
						if r.URL.Path != "/v1/music" || b["prompt"] != nil || b["model_id"] != "music_v2_5" {
							t.Fatal(b)
						}
						chunks := b["composition_plan"].(map[string]any)["chunks"].([]any)
						total := 0.
						var texts []string
						for _, ch := range chunks {
							m := ch.(map[string]any)
							total += m["duration_ms"].(float64)
							texts = append(texts, m["text"].(string))
						}
						if total != 180000 || strings.Join(texts, "\n") != lyrics {
							t.Fatal(chunks)
						}
						res := mockResponse("audio-bytes")
						res.Header.Set("song-id", "song-1")
						return res, nil
					case "fal":
						if b["lyrics"] != lyrics || b["tags"] != c.Style || r.URL.Path != "/fal-ai/ace-step" {
							t.Fatal(b)
						}
						return mockResponse(`{"request_id":"task-1"}`), nil
					}
				}
				if strings.HasSuffix(r.URL.Path, "/status") {
					return mockResponse(`{"status":"COMPLETED"}`), nil
				}
				if r.URL.Path != "/fal-ai/ace-step/requests/task-1" {
					t.Fatal(r.URL.Path)
				}
				return mockResponse(`{"audio":{"url":"https://storage.googleapis.com/falserverless/song.wav"}}`), nil
			})
			dest := filepath.Join(t.TempDir(), "audio.mp3")
			if id == "fal" {
				task, e := h.Submit(context.Background(), c, lyrics)
				if e != nil {
					t.Fatal(e)
				}
				if e = h.Resume(context.Background(), task, dest); e != nil {
					t.Fatal(e)
				}
			} else {
				task, e := h.Generate(context.Background(), lyrics, dest)
				if e != nil || task == "" {
					t.Fatal(task, e)
				}
			}
			b, e := os.ReadFile(dest)
			if e != nil || string(b) != "audio-bytes" || posts != 1 {
				t.Fatal(string(b), e, posts)
			}
			if id != "elevenlabs" && downloads != 1 {
				t.Fatal(downloads)
			}
		})
	}
}
func TestHostedErrorsAndDownloads(t *testing.T) {
	c := Config{Provider: "fal", Model: "fal-ai/ace-step"}
	h := newHosted(c, map[string]string{"FAL_KEY": "super-secret"})
	calls := 0
	h.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("super-secret"))}, nil
	})
	_, e := h.Submit(context.Background(), c, "lyrics")
	if e == nil || strings.Contains(e.Error(), "super-secret") || !strings.Contains(e.Error(), "403") || calls != 1 {
		t.Fatal(e, calls)
	}
	for _, url := range []string{"http://127.0.0.1/", "https://fal.media.evil.test/x", "https://user:secret@fal.media/x", "file:///tmp/x", "https://fal.media:8443/x"} {
		if e = h.download(context.Background(), url, "unused"); e == nil {
			t.Fatal(url)
		}
	}
	if calls != 1 {
		t.Fatal("untrusted downloads issued requests")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.interval = time.Hour
	h.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return mockResponse(`{"status":"IN_QUEUE"}`), nil })
	if h.Resume(ctx, "task-1", "unused") == nil {
		t.Fatal("ignored cancellation")
	}
}

type brokenAudio struct{}

func (brokenAudio) Read([]byte) (int, error) { return 0, fmt.Errorf("download interrupted") }
func TestAtomicHostedDownload(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "audio.mp3")
	os.WriteFile(dest, []byte("old"), 0600)
	if saveAudio(brokenAudio{}, dest) == nil {
		t.Fatal("accepted partial download")
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "old" {
		t.Fatal(string(b))
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal(files)
	}
}
func TestPlatformConfigurationAndSecrets(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "song.yaml")
	original := DefaultConfig + "# user comment\n"
	os.WriteFile(path, []byte(original), 0644)
	if e := Configure(root, "bailian", "", "bad/workspace", ""); e == nil {
		t.Fatal("bad workspace")
	}
	b, _ := os.ReadFile(path)
	if string(b) != original {
		t.Fatal("failed configure changed file")
	}
	if e := Configure(root, "bailian", "", "workspace-1", ""); e != nil {
		t.Fatal(e)
	}
	c, e := LoadConfig(root)
	if e != nil || c.Model != "fun-music-v1" || c.Lyrics != "lyrics.txt" {
		t.Fatal(c, e)
	}
	if e = CheckCredential(c, map[string]string{"DASHSCOPE_API_KEY": "sk-sp-private"}); e == nil || strings.Contains(e.Error(), "private") {
		t.Fatal(e)
	}
	if e = Configure(root, "fal", "", "", ""); e != nil {
		t.Fatal(e)
	}
	c, e = LoadConfig(root)
	if e != nil || c.Model != "fal-ai/ace-step" || c.Workspace != "" {
		t.Fatal(c, e)
	}
	b, _ = os.ReadFile(path)
	if !bytes.Contains(b, []byte("# user comment")) {
		t.Fatal("lost comments")
	}
	var out bytes.Buffer
	PlatformGuide(&out, "", map[string]string{"FAL_KEY": "super-secret"})
	if strings.Contains(out.String(), "super-secret") || !strings.Contains(out.String(), "已设置") {
		t.Fatal(out.String())
	}
	if e = CheckCredential(c, nil); e == nil || !strings.Contains(e.Error(), "am song providers fal") {
		t.Fatal(e)
	}
}
func TestHostedLyricsLimits(t *testing.T) {
	if validateLyrics(Config{Provider: "bailian"}, strings.Repeat("字", 351)) == nil {
		t.Fatal("long Chinese lyrics")
	}
	if validateLyrics(Config{Provider: "elevenlabs", TargetSeconds: 10}, strings.Repeat("a", 201)) == nil {
		t.Fatal("long line")
	}
	if validateLyrics(Config{Provider: "elevenlabs", TargetSeconds: 10}, strings.Repeat("a\n", 200)) == nil {
		t.Fatal("too many short sections")
	}
}

// Exercise the actual candidate state machine: no paid POST on continuation,
// no submission marker for missing credentials, no retry of ambiguous sync calls.
func TestHostedCandidateLifecycle(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	for _, id := range []string{"bailian", "elevenlabs", "fal"} {
		t.Run(id, func(t *testing.T) {
			root, env := fakeEnvironment(t)
			workspace := ""
			if id == "bailian" {
				workspace = "workspace"
			}
			if e := Configure(root, id, "", workspace, ""); e != nil {
				t.Fatal(e)
			}
			os.WriteFile(filepath.Join(root, "lyrics.txt"), []byte("科学知识唱起来\n水汽遇冷凝成云"), 0644)
			var out bytes.Buffer
			if Generate(context.Background(), root, 1, "first", env, &out) == nil {
				t.Fatal("missing key accepted")
			}
			if _, e := os.Stat(filepath.Join(root, Store, "candidates")); !os.IsNotExist(e) {
				t.Fatal("created candidate without credentials")
			}
			env[providerInfo(id).Key] = "do-not-persist"
			posts := 0
			fail := true
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method == "POST" {
					posts++
					if id == "fal" {
						return mockResponse(`{"request_id":"task-1"}`), nil
					}
					if fail {
						return nil, fmt.Errorf("network failure do-not-persist")
					}
					return mockResponse("audio"), nil
				}
				if fail {
					return nil, fmt.Errorf("poll interrupted do-not-persist")
				}
				if strings.HasSuffix(r.URL.Path, "/status") {
					return mockResponse(`{"status":"COMPLETED"}`), nil
				}
				if strings.Contains(r.URL.Path, "/requests/") {
					return mockResponse(`{"audio":{"url":"https://fal.media/song.wav"}}`), nil
				}
				return mockResponse("audio"), nil
			})
			e := Generate(context.Background(), root, 1, "first", env, &out)
			if e == nil || strings.Contains(e.Error(), "do-not-persist") {
				t.Fatal(e)
			}
			fail = false
			e = Generate(context.Background(), root, 1, "first", env, &out)
			if id == "fal" {
				if e != nil {
					t.Fatal(e)
				}
				if e = Generate(context.Background(), root, 1, "first", env, &out); e != nil {
					t.Fatal(e)
				}
			} else if e == nil {
				t.Fatal("retried ambiguous submission")
			}
			if posts != 1 {
				t.Fatal("duplicate generation", posts)
			}
			filepath.WalkDir(filepath.Join(root, Store), func(path string, d os.DirEntry, e error) error {
				if e != nil {
					t.Fatal(e)
				}
				if !d.IsDir() {
					b, _ := os.ReadFile(path)
					if bytes.Contains(b, []byte("do-not-persist")) {
						t.Fatal("persisted key", path)
					}
				}
				return nil
			})
		})
	}
}

func TestConfigureRejectsMultipleDocuments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "song.yaml")
	original := DefaultConfig + "---\nprovider: minimax\n"
	os.WriteFile(path, []byte(original), 0644)
	if Configure(root, "fal", "", "", "") == nil {
		t.Fatal("accepted multiple documents")
	}
	b, _ := os.ReadFile(path)
	if string(b) != original {
		t.Fatal("changed invalid configuration")
	}
}
