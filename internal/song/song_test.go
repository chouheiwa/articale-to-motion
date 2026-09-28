package song

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectionAndTimelineLifecycle(t *testing.T) {
	root := t.TempDir()
	if _, err := Selected(root); err == nil {
		t.Fatal("selection required")
	}
	if err := os.WriteFile(filepath.Join(root, "song.yaml"), []byte(DefaultConfig), 0644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, Store, "candidates", "first")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "audio.mp3"), []byte("audio"), 0644)
	os.WriteFile(filepath.Join(dir, "lyrics.txt"), []byte("同一句\n同一句\n"), 0644)
	c := Candidate{ID: "first", Audio: "audio.mp3", Lyrics: "lyrics.txt", Duration: 10, State: "ready"}
	c.AudioSHA, _ = HashFile(filepath.Join(dir, c.Audio))
	c.LyricsSHA, _ = HashFile(filepath.Join(dir, c.Lyrics))
	if err := WriteJSON(filepath.Join(dir, "candidate.json"), c); err != nil {
		t.Fatal(err)
	}
	if err := Select(root, "../first"); err == nil {
		t.Fatal("unsafe id")
	}
	if err := Select(root, "first"); err != nil {
		t.Fatal(err)
	}
	tl := Timeline{Schema: 1, CandidateID: c.ID, AudioSHA: c.AudioSHA, LyricsSHA: c.LyricsSHA, Duration: 10, Lines: []Line{{ID: "l1", Text: "同一句", Start: 1, End: 3}, {ID: "l2", Text: "同一句", Start: 6, End: 9}}}
	if p := tl.Problems(); len(p) > 0 {
		t.Fatal(p)
	}
	if err := SaveTimeline(root, tl); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTimeline(root); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, c.Audio), []byte("changed"), 0644)
	if _, err := LoadTimeline(root); err == nil {
		t.Fatal("modified selection accepted")
	}
}
func TestTimelineProblems(t *testing.T) {
	tl := Timeline{Schema: 1, Duration: 10, Lines: []Line{{ID: "a", Text: "词", Start: 2, End: 4}, {ID: "a", Text: "词", Start: 3, End: 12}}, Beats: []float64{4, 2}}
	if len(tl.Problems()) < 3 {
		t.Fatal(tl.Problems())
	}
}
func TestSliceAndFrames(t *testing.T) {
	tl := Timeline{Duration: 10.01, Lines: []Line{{ID: "repeat-2", Text: "再唱", Start: 3, End: 7, Words: []Word{{Text: "唱", Start: 5, End: 7}}}}, Beats: []float64{3, 4, 6}}
	c := tl.Slice(4, 2)
	if len(c.Lines) != 1 || c.Lines[0].Start != 0 || c.Lines[0].End != 2 || c.Lines[0].Words[0].Start != 1 || len(c.Beats) != 1 || c.Beats[0] != 0 {
		t.Fatalf("%+v", c)
	}
	if tl.Frames(30) != 301 {
		t.Fatal(tl.Frames(30))
	}
}
func TestConfigStrict(t *testing.T) {
	root := t.TempDir()
	for _, body := range []string{"provider: other\n", DefaultConfig + "api_key: secret\n"} {
		os.WriteFile(filepath.Join(root, "song.yaml"), []byte(body), 0644)
		if _, err := LoadConfig(root); err == nil {
			t.Fatal(body)
		}
	}
	os.WriteFile(filepath.Join(root, "song.yaml"), []byte(DefaultConfig), 0644)
	c, err := LoadConfig(root)
	if err != nil || !strings.Contains(c.Model, "3.0") {
		t.Fatal(c, err)
	}
	os.WriteFile(filepath.Join(root, "cast.yaml"), []byte("{}"), 0644)
	if _, err := LoadConfig(root); err == nil {
		t.Fatal("cast+song")
	}
}
