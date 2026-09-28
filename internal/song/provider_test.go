package song

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestACEProtocolAndDownload(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private" {
			t.Error("auth missing")
		}
		switch r.URL.Path {
		case "/release_task":
			requests++
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			if req["use_format"] != false || req["sample_mode"] != false || req["lyrics"] != "歌词" || req["batch_size"] != float64(1) {
				t.Error(req)
			}
			w.Write([]byte(`{"code":200,"data":{"task_id":"t1"}}`))
		case "/query_result":
			w.Write([]byte(`{"code":200,"data":[{"task_id":"t1","status":1,"result":"[{\"file\":\"/v1/audio?path=a\"}]"}]}`))
		case "/v1/audio":
			w.Write([]byte("audio"))
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer srv.Close()
	a := ACEClient{Endpoint: srv.URL, Key: "private", PollInterval: time.Millisecond}
	id, e := a.Submit(context.Background(), Config{Model: "m", Style: "s"}, "歌词")
	if e != nil || id != "t1" {
		t.Fatal(id, e)
	}
	dest := filepath.Join(t.TempDir(), "audio.mp3")
	if e = a.Resume(context.Background(), id, dest); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "audio" || requests != 1 {
		t.Fatal(string(b), requests)
	}
}
func TestACERejectsLeakAndFailure(t *testing.T) {
	for _, body := range []string{`{"code":500,"error":"private"}`, `{"code":200,"data":{"task_id":""}}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		a := ACEClient{Endpoint: s.URL, Key: "private"}
		_, e := a.Submit(context.Background(), Config{}, "a")
		if e == nil || strings.Contains(e.Error(), "private") {
			t.Fatal(e)
		}
		s.Close()
	}
	a := ACEClient{Endpoint: "http://example.test", Key: "private"}
	if e := a.download(context.Background(), "https://elsewhere.test/audio", filepath.Join(t.TempDir(), "a")); e == nil {
		t.Fatal("cross origin")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := a.Resume(ctx, "t", filepath.Join(t.TempDir(), "a")); e == nil {
		t.Fatal("cancel")
	}
}

func TestACEInterruptedDownloadAndRedirect(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Content-Length", "100")
			w.Write([]byte("short"))
			return
		}
		w.Write([]byte("complete"))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "audio.mp3")
	a := ACEClient{Endpoint: srv.URL}
	if e := a.download(context.Background(), "/audio", dest); e == nil {
		t.Fatal("interrupted download accepted")
	}
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("partial file published")
	}
	if e := a.download(context.Background(), "/audio", dest); e != nil {
		t.Fatal(e)
	}
	otherCalls := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { otherCalls++ }))
	defer other.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, http.StatusFound) }))
	defer redirect.Close()
	a = ACEClient{Endpoint: redirect.URL, Key: "private"}
	if e := a.download(context.Background(), "/audio", dest); e == nil || strings.Contains(e.Error(), "private") {
		t.Fatal(e)
	}
	if otherCalls != 0 {
		t.Fatal("credential-bearing redirect followed")
	}
}
