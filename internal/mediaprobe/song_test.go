package mediaprobe

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chouheiwa/articale-to-motion/internal/envutil"
)

func TestSongMuxPadsOnlySubframeTail(t *testing.T) {
	binary, e := exec.LookPath("ffmpeg")
	if e != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, e = exec.LookPath("ffprobe"); e != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	audio := filepath.Join(dir, "song.wav")
	video := filepath.Join(dir, "silent.mp4")
	dest := filepath.Join(dir, "final.mp4")
	run := func(args ...string) {
		t.Helper()
		if b, e := exec.Command(binary, append([]string{"-v", "error", "-y"}, args...)...).CombinedOutput(); e != nil {
			t.Fatalf("%v: %s", e, b)
		}
	}
	run("-f", "lavfi", "-i", "sine=frequency=440:duration=1.01", "-ar", "48000", audio)
	run("-f", "lavfi", "-i", "color=c=blue:s=320x240:r=30", "-frames:v", "31", "-c:v", "libx264", "-pix_fmt", "yuv420p", video)
	tc, e := New(envutil.EnvMap())
	if e != nil {
		t.Fatal(e)
	}
	if e = tc.PrepareAlignment(context.Background(), audio, filepath.Join(dir, "analysis.wav")); e != nil {
		t.Fatal(e)
	}
	analysis, e := tc.Probe(filepath.Join(dir, "analysis.wav"))
	if e != nil || analysis.Audio.SampleRate != 16000 || analysis.Audio.Channels != 1 || math.Abs(analysis.DurationSeconds-1.01) > 0.001 {
		t.Fatal(analysis, e)
	}
	if e = tc.MuxSong(context.Background(), video, audio, dest, 1.01, 30); e != nil {
		t.Fatal(e)
	}
	m, e := tc.Probe(dest)
	if e != nil || m.Audio == nil || m.Video == nil || m.Video.NBFrames != 31 {
		t.Fatal(m, e)
	}
	for _, duration := range []float64{0, -1, math.NaN(), 2} {
		if e = tc.MuxSong(context.Background(), video, audio, dest, duration, 30); e == nil {
			t.Fatal("invalid duration", duration)
		}
	}
	if e = tc.MuxSong(context.Background(), video, audio, video, 1.01, 30); e == nil {
		t.Fatal("overwrite")
	}
	run("-f", "lavfi", "-i", "color=c=blue:s=320x240:r=30", "-frames:v", "30", "-c:v", "libx264", "-pix_fmt", "yuv420p", video)
	if e = tc.MuxSong(context.Background(), video, audio, dest, 1.01, 30); e == nil {
		t.Fatal("truncated master")
	}
}
