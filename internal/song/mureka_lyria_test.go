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

const lyriaSuccess = `{"status":"completed","steps":[{"type":"model_output","content":[{"type":"text","text":"provider lyrics must not replace source"},{"type":"audio","mime_type":"audio/mpeg","data":"YXVkaW8="}]}]}`

func TestMurekaLyriaCandidateLifecycle(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	for _, id := range []string{"mureka", "lyria"} {
		t.Run(id, func(t *testing.T) {
			root, env := fakeEnvironment(t)
			if e := Configure(root, id, "", "", ""); e != nil {
				t.Fatal(e)
			}
			env[providerInfo(id).Key] = "secret-test"
			lyrics := "[Verse]\n科学知识唱起来\n[Chorus]\n水汽遇冷凝成云\n水汽遇冷凝成云"
			os.WriteFile(filepath.Join(root, "lyrics.txt"), []byte(lyrics), 0644)
			posts, polls := 0, 0
			failPoll := id == "mureka"
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "cdn.mureka.ai" {
					if r.Header.Get("Authorization") != "" {
						t.Fatal("leaked key on download")
					}
					return mockResponse("audio"), nil
				}
				if id == "mureka" {
					if r.URL.Host != "api.mureka.ai" || r.Header.Get("Authorization") != "Bearer secret-test" {
						t.Fatal("wrong Mureka auth/origin")
					}
				} else {
					if r.URL.Host != "generativelanguage.googleapis.com" || r.Header.Get("x-goog-api-key") != "secret-test" || r.Header.Get("Authorization") != "" {
						t.Fatal("wrong Google auth/origin")
					}
				}
				if r.Method == "POST" {
					posts++
					var b map[string]any
					if e := json.NewDecoder(r.Body).Decode(&b); e != nil {
						t.Fatal(e)
					}
					if id == "mureka" {
						if r.URL.Path != "/v1/song/generate" || b["n"] != float64(1) || b["lyrics"] != lyrics || b["model"] != "mureka-9.5" {
							t.Fatal(b)
						}
						return mockResponse(`{"id":"task-1","status":"preparing"}`), nil
					}
					if r.URL.Path != "/v1beta/interactions" || b["model"] != "lyria-3.5" || b["store"] != false || b["stream"] != false || !strings.Contains(b["input"].(string), lyrics) {
						t.Fatal(b)
					}
					return mockResponse(lyriaSuccess), nil
				}
				polls++
				if r.Method != "GET" || r.URL.Path != "/v1/song/query/task-1" {
					t.Fatal(r.Method, r.URL.Path)
				}
				if failPoll {
					return nil, fmt.Errorf("network interrupted secret-test")
				}
				return mockResponse(`{"id":"task-1","status":"succeeded","choices":[{"url":"https://cdn.mureka.ai/song.mp3?signature=private"}]}`), nil
			})
			var out bytes.Buffer
			if e := Doctor(context.Background(), root, env, &out); e != nil {
				t.Fatal(e)
			}
			if posts != 0 || polls != 0 {
				t.Fatal("doctor contacted provider")
			}
			e := Generate(context.Background(), root, 1, "first", env, &out)
			if id == "mureka" {
				if e == nil || strings.Contains(e.Error(), "secret-test") {
					t.Fatal(e)
				}
				failPoll = false
				e = Generate(context.Background(), root, 1, "first", env, &out)
			}
			if e != nil {
				t.Fatal(e)
			}
			if e = Generate(context.Background(), root, 1, "first", env, &out); e != nil {
				t.Fatal(e)
			}
			if posts != 1 {
				t.Fatal("resubmitted", posts)
			}
			dirs, _ := os.ReadDir(filepath.Join(root, Store, "candidates"))
			if len(dirs) != 1 {
				t.Fatal(dirs)
			}
			c, e := LoadCandidate(root, dirs[0].Name())
			if e != nil || c.Provider != id {
				t.Fatal(c, e)
			}
			dir, _ := CandidateDir(root, c.ID)
			b, _ := os.ReadFile(filepath.Join(dir, c.Lyrics))
			if string(b) != lyrics {
				t.Fatal("changed lyrics")
			}
			if _, e = os.Stat(filepath.Join(root, Store, "selected.json")); !os.IsNotExist(e) {
				t.Fatal("selected automatically")
			}
			filepath.WalkDir(dir, func(path string, d os.DirEntry, e error) error {
				if e != nil {
					t.Fatal(e)
				}
				if !d.IsDir() {
					b, _ := os.ReadFile(path)
					for _, secret := range []string{"secret-test", "signature=private"} {
						if bytes.Contains(b, []byte(secret)) {
							t.Fatal("persisted secret", path)
						}
					}
				}
				return nil
			})
		})
	}
}
func TestLyriaMalformedAudioIsAtomic(t *testing.T) {
	audio := `{"type":"audio","mime_type":"audio/mpeg","data":"YXVkaW8="}`
	for _, body := range []string{
		`{"status":"failed"}`,
		`{"steps":[]}`,
		`{"steps":[{"type":"model_output","content":[` + audio + `,` + audio + `]}]}`,
		strings.Replace(lyriaSuccess, "audio/mpeg", "audio/wav", 1),
		strings.Replace(lyriaSuccess, "YXVkaW8=", "YXVkaW8=!!!!", 1),
		strings.Replace(lyriaSuccess, "YXVkaW8=", "", 1),
		lyriaSuccess + `{}`,
	} {
		dir := t.TempDir()
		dest := filepath.Join(dir, "audio.mp3")
		os.WriteFile(dest, []byte("old"), 0644)
		if _, e := decodeLyria(strings.NewReader(body), dest); e == nil {
			t.Fatal("accepted malformed audio", body)
		}
		b, _ := os.ReadFile(dest)
		if string(b) != "old" {
			t.Fatal("replaced old file")
		}
		files, _ := os.ReadDir(dir)
		if len(files) != 1 {
			t.Fatal("left partial download")
		}
	}
	if _, e := decodeLyria(brokenAudio{}, filepath.Join(t.TempDir(), "audio.mp3")); e == nil {
		t.Fatal("ignored broken reader")
	}
}
func TestMurekaValidationAndPolling(t *testing.T) {
	c := Config{Provider: "mureka", Model: "mureka-9.5", TargetSeconds: 120, Style: "rap", Language: "zh", BPM: 100}
	if validateLyrics(c, strings.Repeat("字", 5001)) == nil {
		t.Fatal("long lyrics")
	}
	c.Style = strings.Repeat("x", 1024)
	if validateLyrics(c, "词") == nil {
		t.Fatal("long prompt")
	}
	c.Style = "rap"
	for _, body := range []string{
		`{"id":"other","status":"succeeded"}`,
		`{"id":"task-1","status":"failed","failed_reason":"secret"}`,
		`{"id":"task-1","status":"timeouted"}`,
		`{"id":"task-1","status":"cancelled"}`,
		`{"id":"task-1","status":"unknown"}`,
		`{"id":"task-1","status":"succeeded","choices":[]}`,
		`{"id":"task-1","status":"succeeded","choices":[{"url":"https://cdn.mureka.ai/one"},{"url":"https://cdn.mureka.ai/two"}]}`,
	} {
		h := newHosted(c, nil)
		h.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return mockResponse(body), nil })
		if e := h.Resume(context.Background(), "task-1", "unused"); e == nil || strings.Contains(e.Error(), "secret") {
			t.Fatal(e)
		}
	}
	h := newHosted(c, nil)
	h.interval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return mockResponse(`{"id":"task-1","status":"running"}`), nil
	})
	if h.Resume(ctx, "task-1", "unused") == nil {
		t.Fatal("ignored cancellation")
	}
	for _, ref := range []string{"https://cdn.mureka.ai.evil.test/x", "https://127.0.0.1/x", "https://cdn.mureka.ai:8443/x"} {
		if h.download(context.Background(), ref, "unused") == nil {
			t.Fatal(ref)
		}
	}
}
func TestNewProviderConfiguration(t *testing.T) {
	for _, id := range []string{"mureka", "lyria"} {
		root, env := fakeEnvironment(t)
		if e := Configure(root, id, "", "", ""); e != nil {
			t.Fatal(e)
		}
		c, e := LoadConfig(root)
		if e != nil || c.Model != providerInfo(id).Model {
			t.Fatal(c, e)
		}
		if e = CheckCredential(c, env); e == nil || !strings.Contains(e.Error(), providerInfo(id).Key) {
			t.Fatal(e)
		}
		var out bytes.Buffer
		PlatformGuide(&out, id, map[string]string{providerInfo(id).Key: "secret-test"})
		if strings.Contains(out.String(), "secret-test") || !strings.Contains(out.String(), providerInfo(id).Key) {
			t.Fatal(out.String())
		}
		if Configure(root, id, "invalid-model", "", "") == nil {
			t.Fatal("accepted unsupported model")
		}
		if Configure(root, id, "", "", "https://other.example") == nil {
			t.Fatal("accepted foreign endpoint")
		}
	}
}
func TestLyriaFailureDoesNotRetry(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	root, env := fakeEnvironment(t)
	Configure(root, "lyria", "", "", "")
	env["GEMINI_API_KEY"] = "secret-test"
	posts := 0
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		posts++
		return &http.Response{StatusCode: 403, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("secret-test"))}, nil
	})
	for i := 0; i < 2; i++ {
		var out bytes.Buffer
		if e := Generate(context.Background(), root, 1, "failed", env, &out); e == nil || strings.Contains(e.Error(), "secret-test") {
			t.Fatal(e)
		}
	}
	if posts != 1 {
		t.Fatal("duplicate POST", posts)
	}
}
