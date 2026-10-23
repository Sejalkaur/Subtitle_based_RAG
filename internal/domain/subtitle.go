package domain

// Cue represents a single normalized subtitle cue.
type Cue struct {
	CourseID string `json:"courseId"`
	FileID   string `json:"fileId"`
	Index    int    `json:"index"`
	StartMs  int64  `json:"startMs"`
	EndMs    int64  `json:"endMs"`
	Text     string `json:"text"`
}
