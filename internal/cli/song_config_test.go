package cli

import (
	"bytes"
	"github.com/chouheiwa/articale-to-motion/internal/song"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSongPlatformGuideAndConfigure(t *testing.T) {
	t.Setenv("FAL_KEY", "do-not-print-this")
	var out, err bytes.Buffer
	if code := Execute([]string{"song", "providers", "fal"}, &out, &err); code != 0 {
		t.Fatal(err.String())
	}
	if strings.Contains(out.String(), "do-not-print-this") || !strings.Contains(out.String(), "AM_PASSTHROUGH_ENV") {
		t.Fatal(out.String())
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "song.yaml"), []byte(song.DefaultConfig), 0644)
	out.Reset()
	err.Reset()
	if code := Execute([]string{"song", "configure", "--project-root", root, "--provider", "elevenlabs"}, &out, &err); code != 0 {
		t.Fatal(err.String())
	}
	c, e := song.LoadConfig(root)
	if e != nil || c.Provider != "elevenlabs" || !strings.Contains(out.String(), "ELEVENLABS_API_KEY") {
		t.Fatal(c, e, out.String())
	}
}

func TestMurekaAndLyriaCLISetup(t *testing.T) {
	for _, tc := range []struct{ provider, key, model string }{{"mureka", "MUREKA_API_KEY", "mureka-9.5"}, {"lyria", "GEMINI_API_KEY", "lyria-3.5"}} {
		t.Setenv(tc.key, "private-value-not-for-output")
		root := t.TempDir()
		os.WriteFile(filepath.Join(root, "song.yaml"), []byte(song.DefaultConfig), 0644)
		var out, err bytes.Buffer
		if code := Execute([]string{"song", "configure", "--project-root", root, "--provider", tc.provider}, &out, &err); code != 0 {
			t.Fatal(err.String())
		}
		if strings.Contains(out.String(), "private-value-not-for-output") || !strings.Contains(out.String(), tc.key) || !strings.Contains(out.String(), "AM_PASSTHROUGH_ENV") {
			t.Fatal(out.String())
		}
		c, e := song.LoadConfig(root)
		if e != nil || c.Model != tc.model {
			t.Fatal(c, e)
		}
	}
}

func TestManualHandoffCLI(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "song.yaml"), []byte(song.DefaultConfig), 0644)
	os.WriteFile(filepath.Join(root, "lyrics.txt"), []byte("太阳照着水面\n水汽升上蓝天"), 0644)
	var out, err bytes.Buffer
	for _, args := range [][]string{
		{"song", "configure", "--provider", "manual", "--project-root", root},
		{"song", "handoff", "--project-root", root},
	} {
		out.Reset()
		err.Reset()
		if code := Execute(args, &out, &err); code != 0 {
			t.Fatal(err.String())
		}
	}
	if !strings.Contains(out.String(), "太阳照着水面") || !strings.Contains(out.String(), "等待用户提供音频") {
		t.Fatal(out.String())
	}
}
