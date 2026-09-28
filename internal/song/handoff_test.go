package song

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/mediaprobe"
)

func TestManualHandoffReceiveLifecycle(t *testing.T) {
	root, env := fakeEnvironment(t)
	if e := Configure(root, "manual", "", "", ""); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	// No credentials, Python, media binaries or even PATH are needed to hand off.
	if e := Generate(context.Background(), root, 2, "default", nil, &out); e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{"歌词", "Mandarin educational solo rap", "等待用户提供音频", "暂停"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal(out.String())
		}
	}
	h, dir, e := LoadHandoff(root)
	if e != nil || h.State != "waiting_for_audio" {
		t.Fatal(h, e)
	}
	snapshot, _ := os.ReadFile(filepath.Join(dir, "lyrics.txt"))
	if _, e = os.Stat(filepath.Join(root, Store, "candidates")); !os.IsNotExist(e) {
		t.Fatal("handoff generated candidates")
	}
	if e = Handoff(root, &out); e != nil {
		t.Fatal(e)
	}
	same, _, _ := LoadHandoff(root)
	if same != h {
		t.Fatal("did not reuse snapshot")
	}
	os.WriteFile(filepath.Join(root, "lyrics.txt"), []byte("new unrelated lyrics"), 0644)
	if e = Handoff(root, &out); e == nil {
		t.Fatal("silently replaced pending snapshot")
	}
	after, _, _ := LoadHandoff(root)
	if after != h {
		t.Fatal("changed wait record")
	}
	media, e := mediaprobe.New(env)
	if e != nil {
		t.Fatal(e)
	}
	audio := filepath.Join(t.TempDir(), "downloaded.mp3")
	os.WriteFile(audio, []byte("external audio"), 0644)
	candidate, e := Receive(root, audio, media)
	if e != nil {
		t.Fatal(e)
	}
	candidateDir, _ := CandidateDir(root, candidate.ID)
	b, _ := os.ReadFile(filepath.Join(candidateDir, "lyrics.txt"))
	if !bytes.Equal(b, snapshot) {
		t.Fatal("did not bind original lyrics")
	}
	imported, _, e := LoadHandoff(root)
	if e != nil || imported.State != "imported" || imported.CandidateID != candidate.ID {
		t.Fatal(imported, e)
	}
	again, e := Receive(root, audio, media)
	if e != nil || again.ID != candidate.ID {
		t.Fatal(again, e)
	}
	if _, e = Selected(root); e == nil {
		t.Fatal("selected without user confirmation")
	}
	if e = Select(root, candidate.ID); e != nil {
		t.Fatal(e)
	}
	if e = Handoff(root, &out); e != nil {
		t.Fatal(e)
	} // new material allowed after prior audio received
	next, _, _ := LoadHandoff(root)
	if next.ID == h.ID || next.State != "waiting_for_audio" {
		t.Fatal(next)
	}
}
func TestHandoffRefreshAndTampering(t *testing.T) {
	root, _ := fakeEnvironment(t)
	var out bytes.Buffer
	if e := Handoff(root, &out); e != nil {
		t.Fatal(e)
	}
	old, _, _ := LoadHandoff(root)
	os.WriteFile(filepath.Join(root, "lyrics.txt"), []byte("修正过的知识歌词"), 0644)
	if e := HandoffWithRefresh(root, true, &out); e != nil {
		t.Fatal(e)
	}
	h, dir, _ := LoadHandoff(root)
	if h.ID == old.ID {
		t.Fatal("refresh ignored")
	}
	os.WriteFile(filepath.Join(dir, "style.txt"), []byte("tampered"), 0644)
	if _, _, e := LoadHandoff(root); e == nil {
		t.Fatal("accepted altered snapshot")
	}
	if e := Handoff(root, &out); e == nil {
		t.Fatal("overwrote changed snapshot")
	}
}
func TestHandoffDoesNotFollowSymlinks(t *testing.T) {
	root, _ := fakeEnvironment(t)
	outside := t.TempDir()
	os.MkdirAll(filepath.Join(root, Store), 0755)
	if e := os.Symlink(outside, filepath.Join(root, Store, "handoffs")); e != nil {
		t.Fatal(e)
	}
	if e := Handoff(root, &bytes.Buffer{}); e == nil {
		t.Fatal("followed symlink")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("wrote outside")
	}
}
func TestReceiveFailureKeepsWaiting(t *testing.T) {
	root, env := fakeEnvironment(t)
	Handoff(root, &bytes.Buffer{})
	h, _, _ := LoadHandoff(root)
	m, _ := mediaprobe.New(env)
	if _, e := Receive(root, filepath.Join(root, "missing.mp3"), m); e == nil {
		t.Fatal("accepted missing audio")
	}
	after, _, _ := LoadHandoff(root)
	if after != h {
		t.Fatal("marked missing audio received")
	}
}
