package domain

// Chunk represents a sliding-window grouped subtitle chunk.
type Chunk struct {
	ChunkID     string `json:"chunkId"`
	CourseID    string `json:"courseId"`
	FileID      string `json:"fileId"`
	StartMs     int64  `json:"startMs"`
	EndMs       int64  `json:"endMs"`
	Text        string `json:"text"`
	LineIndices []int  `json:"lineIndices"`
	SourceFile  string `json:"sourceFile,omitempty"`
}
