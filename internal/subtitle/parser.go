package subtitle

import (
	"bufio"
	"path/filepath"
	"regexp"
	"strings"

	"subtitle-rag/internal/domain"
)

var (
	htmlTagRegex       = regexp.MustCompile(`<[^>]+>`)
	newlineRegex       = regexp.MustCompile(`[\r\n]+`)
	multipleSpaceRegex = regexp.MustCompile(`\s+`)
	timestampLineRegex = regexp.MustCompile(`(?:(\d+):)?(\d{2}):(\d{2}[.,]\d{3})\s+-->\s+(?:(\d+):)?(\d{2}):(\d{2}[.,]\d{3})`)
)

// CleanText performs text normalization matching Node.js cleanText:
// strips HTML tags, replaces newlines with space, collapses spaces, normalizes ellipses, and trims.
func CleanText(t string) string {
	if t == "" {
		return ""
	}
	s := htmlTagRegex.ReplaceAllString(t, " ")
	s = newlineRegex.ReplaceAllString(s, " ")
	s = multipleSpaceRegex.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "…", "...")
	return strings.TrimSpace(s)
}

// DeriveCourseID derives courseId from the absolute file path, matching scripts/ingest.js:
// looks for "raw" directory segment; if present, the next segment is the courseId.
func DeriveCourseID(absPath string) string {
	cleaned := filepath.Clean(absPath)
	parts := strings.Split(cleaned, string(filepath.Separator))
	rawIdx := -1
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "raw" {
			rawIdx = i
			break
		}
	}
	if rawIdx >= 0 && rawIdx+1 < len(parts) {
		return strings.ToLower(strings.TrimSpace(parts[rawIdx+1]))
	}
	return "default"
}

// DeriveFileID computes fileId as "<courseId>__<fileBaseName>".
func DeriveFileID(absPath, courseID string) string {
	base := filepath.Base(absPath)
	ext := filepath.Ext(base)
	fileBase := strings.TrimSuffix(base, ext)
	return courseID + "__" + fileBase
}

// rawCue captures a parsed cue before normalization
type rawCue struct {
	startStr string
	endStr   string
	lines    []string
}

// ParseVTT parses WebVTT text into normalized Cue domain objects.
func ParseVTT(vttText, fileID, courseID string) []domain.Cue {
	scanner := bufio.NewScanner(strings.NewReader(vttText))
	var rawCues []rawCue
	var currentCue *rawCue
	inHeader := true

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Header check
		if inHeader {
			if strings.HasPrefix(trimmed, "WEBVTT") {
				continue
			}
			// Header comments or metadata before first empty line
			if trimmed == "" {
				inHeader = false
			}
			continue
		}

		// Check for timestamp arrow
		if strings.Contains(line, "-->") {
			parts := strings.Split(line, "-->")
			if len(parts) == 2 {
				startStr := strings.TrimSpace(parts[0])
				// End timestamp may be followed by cue settings (e.g., "00:02.000 align:start")
				endFields := strings.Fields(parts[1])
				endStr := ""
				if len(endFields) > 0 {
					endStr = endFields[0]
				}

				if currentCue != nil {
					rawCues = append(rawCues, *currentCue)
				}
				currentCue = &rawCue{
					startStr: startStr,
					endStr:   endStr,
					lines:    []string{},
				}
				continue
			}
		}

		if trimmed == "" {
			if currentCue != nil {
				rawCues = append(rawCues, *currentCue)
				currentCue = nil
			}
			continue
		}

		// If we are inside a cue, collect lines
		if currentCue != nil {
			currentCue.lines = append(currentCue.lines, line)
		}
		// Otherwise, it could be a cue identifier (number/name) right before the timestamp line, ignore.
	}

	if currentCue != nil {
		rawCues = append(rawCues, *currentCue)
	}

	cues := make([]domain.Cue, 0, len(rawCues))
	for i, rc := range rawCues {
		text := CleanText(strings.Join(rc.lines, " "))
		startMs := HHMMSSSToMs(rc.startStr)
		endMs := HHMMSSSToMs(rc.endStr)

		cues = append(cues, domain.Cue{
			CourseID: courseID,
			FileID:   fileID,
			Index:    i + 1,
			StartMs:  startMs,
			EndMs:    endMs,
			Text:     text,
		})
	}

	return cues
}
