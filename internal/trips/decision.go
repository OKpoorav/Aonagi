package trips

import "time"

type Decision struct {
	Category   string    `json:"category"`
	Value      string    `json:"value"`
	Source     string    `json:"source"` // "poll" | "ai_extraction"
	Timestamp  time.Time `json:"timestamp"`
	Confidence float64   `json:"confidence"`
}

type PollSuggestion struct {
	Category string   `json:"category"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
}
