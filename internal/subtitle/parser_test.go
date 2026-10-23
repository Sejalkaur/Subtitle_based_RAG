package subtitle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"subtitle-rag/internal/domain"
)

func TestMsToTimestamp(t *testing.T) {
	tests := []struct {
		ms       int64
		expected string
	}{
		{0, "00:00:00.000"},
		{1000, "00:00:01.000"},
		{61234, "00:01:01.234"},
		{3661234, "01:01:01.234"},
	}

	for _, tc := range tests {
		actual := MsToTimestamp(tc.ms)
		if actual != tc.expected {
			t.Errorf("MsToTimestamp(%d): expected %s, got %s", tc.ms, tc.expected, actual)
		}
	}
}

func TestHHMMSSSToMs(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"00:00:01.000", 1000},
		{"00:01:01.234", 61234},
		{"01:01:01.234", 3661234},
		{"01:01.234", 61234},
		{"00:00:02,500", 2500}, // comma decimal
	}

	for _, tc := range tests {
		actual := HHMMSSSToMs(tc.input)
		if actual != tc.expected {
			t.Errorf("HHMMSSSToMs(%s): expected %d, got %d", tc.input, tc.expected, actual)
		}
	}
}

func TestCleanText(t *testing.T) {
	raw := "<b>Hello</b>\r\nworld  with   multiple   spaces… and tags<c.color>!"
	expected := "Hello world with multiple spaces... and tags !"
	actual := CleanText(raw)
	if actual != expected {
		t.Errorf("CleanText: expected %q, got %q", expected, actual)
	}
}

func TestParseVTT_TestFile(t *testing.T) {
	rawVTT := `WEBVTT

00:00:01.000 --> 00:00:02.000
Hello there!

00:00:02.500 --> 00:00:03.500
General Kenobi.
`
	cues := ParseVTT(rawVTT, "test.vtt__test", "test.vtt")
	if len(cues) != 2 {
		t.Fatalf("expected 2 cues, got %d", len(cues))
	}

	if cues[0].StartMs != 1000 || cues[0].EndMs != 2000 || cues[0].Text != "Hello there!" {
		t.Errorf("cue[0] mismatch: %+v", cues[0])
	}
	if cues[1].StartMs != 2500 || cues[1].EndMs != 3500 || cues[1].Text != "General Kenobi." {
		t.Errorf("cue[1] mismatch: %+v", cues[1])
	}
}

func TestParseVTT_ParityWithNodeParsed(t *testing.T) {
	// Compare with data/parsed/nodejs__01-node-introduction.json if present
	vttPath := filepath.Join("..", "..", "data", "raw", "nodejs", "01 Subtitles(01-26)", "01-node-introduction.vtt")
	parsedJSONPath := filepath.Join("..", "..", "data", "parsed", "nodejs__01-node-introduction.json")

	vttBytes, err := os.ReadFile(vttPath)
	if err != nil {
		t.Skip("raw vtt file not found, skipping parity test")
		return
	}

	jsonBytes, err := os.ReadFile(parsedJSONPath)
	if err != nil {
		t.Skip("node parsed json not found, skipping parity test")
		return
	}

	var expectedCues []domain.Cue
	if err := json.Unmarshal(jsonBytes, &expectedCues); err != nil {
		t.Fatalf("failed to unmarshal node parsed cues: %v", err)
	}

	actualCues := ParseVTT(string(vttBytes), "nodejs__01-node-introduction", "nodejs")
	if len(actualCues) != len(expectedCues) {
		t.Fatalf("cue count mismatch: expected %d, got %d", len(expectedCues), len(actualCues))
	}

	for i := range actualCues {
		exp := expectedCues[i]
		act := actualCues[i]
		if act.Index != exp.Index {
			t.Errorf("cue[%d].Index: expected %d, got %d", i, exp.Index, act.Index)
		}
		if act.StartMs != exp.StartMs {
			t.Errorf("cue[%d].StartMs: expected %d, got %d", i, exp.StartMs, act.StartMs)
		}
		if act.EndMs != exp.EndMs {
			t.Errorf("cue[%d].EndMs: expected %d, got %d", i, exp.EndMs, act.EndMs)
		}
		if act.Text != exp.Text {
			t.Errorf("cue[%d].Text:\nexp: %q\nact: %q", i, exp.Text, act.Text)
		}
	}
}
