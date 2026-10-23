package retrieval

import (
	"fmt"
	"strings"

	"subtitle-rag/internal/domain"
	"subtitle-rag/internal/subtitle"
)

// Prompt holds system and user message strings.
type Prompt struct {
	System string
	User   string
}

// BuildPrompt constructs the system and user prompt matching Node.js buildPrompt.
func BuildPrompt(query string, chunks []domain.SearchHit) Prompt {
	var contextParts []string
	for i, c := range chunks {
		rangeStr := fmt.Sprintf("%s–%s", subtitle.MsToTimestamp(c.StartMs), subtitle.MsToTimestamp(c.EndMs))
		part := fmt.Sprintf("[#%d] [%s %s]\n%s", i+1, c.FileID, rangeStr, c.Text)
		contextParts = append(contextParts, part)
	}

	contextStr := strings.Join(contextParts, "\n\n")
	if contextStr == "" {
		contextStr = "(empty)"
	}

	system := strings.Join([]string{
		"You are a helpful tutor answering strictly from the provided context.",
		"Rules:",
		"- Use ONLY the context snippets below; do not invent details.",
		`- If the answer is not clear from the context, say: "I dont know, but I could help in other areas like nodejs and python."`,
		"- Always include citations in square brackets like [fileId start–end] next to the claim they support.",
		"- Prefer concise, step-by-step explanations and in easy language.",
	}, "\n")

	user := strings.Join([]string{
		fmt.Sprintf("User question: %s", query),
		"",
		"Context:",
		contextStr,
	}, "\n")

	return Prompt{
		System: system,
		User:   user,
	}
}
