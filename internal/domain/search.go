package domain

// SearchOptions represents options passed to retrieval service.
type SearchOptions struct {
	Query     string
	CourseID  *string
	FileID    *string
	TopK      int
	MMRK      int
	UseMMR    bool
	StrictMMR bool
	MinScore  float64
}

// SearchHit represents a formatted search match.
type SearchHit struct {
	FileID      string    `json:"fileId"`
	CourseID    string    `json:"courseId"`
	StartMs     int64     `json:"startMs"`
	EndMs       int64     `json:"endMs"`
	Start       string    `json:"start"`
	End         string    `json:"end"`
	Preview     string    `json:"preview"`
	Text        string    `json:"text"`
	LineIndices []int     `json:"lineIndices"`
	Score       float64   `json:"score"`
	Vector      []float32 `json:"-"`
}

// SearchMeta represents retrieval metadata.
type SearchMeta struct {
	TopK      int                `json:"topK"`
	MMR       bool               `json:"mmr"`
	MMRKept   *int               `json:"mmrKept,omitempty"`
	Reason    string             `json:"reason,omitempty"`
	BestScore *float64           `json:"bestScore,omitempty"`
	MinScore  *float64           `json:"minScore,omitempty"`
	Filter    map[string]*string `json:"filter"`
}

// SearchResult represents output from the searchChunks service.
type SearchResult struct {
	Query   string      `json:"query"`
	Results []SearchHit `json:"results"`
	Meta    SearchMeta  `json:"meta"`
}

// Citation represents a structured citation returned to clients.
type Citation struct {
	CourseID string  `json:"courseId"`
	FileID   string  `json:"fileId"`
	Start    string  `json:"start"`
	End      string  `json:"end"`
	StartMs  int64   `json:"startMs"`
	EndMs    int64   `json:"endMs"`
	Score    float64 `json:"score"`
}

// AskRequest represents input options for question answering.
type AskRequest struct {
	Query     string   `json:"query"`
	CourseID  *string  `json:"courseId"`
	TopK      *int     `json:"topK"`
	MMRK      *int     `json:"mmrK"`
	MinScore  *float64 `json:"minScore"`
	StrictMMR *bool    `json:"strictMMR"`
}

// AskUsed captures the parameters and retrieval metadata used to answer.
type AskUsed struct {
	Query     string     `json:"query"`
	CourseID  *string    `json:"courseId"`
	TopK      int        `json:"topK"`
	MMRK      int        `json:"mmrK"`
	MinScore  float64    `json:"minScore"`
	StrictMMR bool       `json:"strictMMR"`
	Meta      SearchMeta `json:"meta"`
}

// AskResponse represents the response body for POST /api/ask.
type AskResponse struct {
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
	Used      AskUsed    `json:"used"`
}
