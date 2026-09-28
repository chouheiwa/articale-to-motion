package song

import (
	"fmt"
	"math"
	"path/filepath"
)

// ExportPhrases bridges the song timeline to the existing visible-event review.
// Confidence stays unknown: CTC success is not a measured singing accuracy score.
func ExportPhrases(root string, fps int) error {
	if fps <= 0 {
		return fmt.Errorf("短语时间轴需要正整数 fps")
	}
	t, e := LoadTimeline(root)
	if e != nil {
		return e
	}
	type phrase struct {
		ID         string   `json:"phrase_id"`
		Subtitle   int      `json:"subtitle_index"`
		Text       string   `json:"source_text"`
		Order      int      `json:"semantic_order"`
		StartMS    int      `json:"start_ms"`
		EndMS      int      `json:"end_ms"`
		StartFrame int      `json:"start_frame"`
		EndFrame   int      `json:"end_frame_exclusive"`
		Source     string   `json:"alignment_source"`
		Confidence *float64 `json:"confidence"`
		Reviewed   bool     `json:"reviewed"`
		Events     []string `json:"visible_event_ids"`
	}
	phrases := make([]phrase, 0, len(t.Lines))
	for i, l := range t.Lines {
		phrases = append(phrases, phrase{ID: l.ID, Subtitle: i + 1, Text: l.Text, Order: i + 1, StartMS: int(math.Round(l.Start * 1000)), EndMS: int(math.Round(l.End * 1000)), StartFrame: int(math.Ceil(l.Start*float64(fps) - 1e-9)), EndFrame: int(math.Ceil(l.End*float64(fps) - 1e-9)), Source: t.Source, Reviewed: t.Reviewed, Events: []string{}})
	}
	body := map[string]any{"schema": 1, "fps": fps, "total_frames": t.Frames(fps), "candidate_id": t.CandidateID, "audio_sha256": t.AudioSHA, "lyrics_sha256": t.LyricsSHA, "phrases": phrases}
	for _, rel := range []string{Store + "/phrase-timeline.json", "production/phrase-timeline.json"} {
		p, e := LocalPath(root, filepath.FromSlash(rel))
		if e != nil {
			return e
		}
		if e = WriteJSON(p, body); e != nil {
			return e
		}
	}
	return nil
}
