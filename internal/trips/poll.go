package trips

import (
	"time"
)

const (
	CategoryDestination    = "destination"
	CategoryBudget         = "budget"
	CategoryDates          = "dates"
	CategoryAccommodation  = "accommodation"
	CategoryActivities     = "activities"
	CategoryTransportation = "transportation"
)

func IsValidCategory(c string) bool {
	switch c {
	case CategoryDestination, CategoryBudget, CategoryDates, CategoryAccommodation, CategoryActivities, CategoryTransportation:
		return true
	default:
		return false
	}
}

type Poll struct {
	ID        string            `json:"id"`
	RoomID    string            `json:"roomId"`
	Question  string            `json:"question"`
	Category  string            `json:"category"`
	Options   []string          `json:"options"`
	Votes     map[string]string `json:"votes"` // UserID -> ChosenOption
	Status    string            `json:"status"` // "active" | "closed"
	CreatedBy string            `json:"createdBy"`
	CreatedAt time.Time         `json:"createdAt"`
	ClosedAt  time.Time         `json:"closedAt,omitempty"`
}

// GetWinner resolves the winning option.
// It returns (winningOption, hasClearWinner).
// If it's a tie or there are no votes, it returns ("", false).
func (p *Poll) GetWinner() (string, bool) {
	if len(p.Votes) == 0 {
		return "", false
	}

	counts := make(map[string]int)
	for _, opt := range p.Votes {
		counts[opt]++
	}

	maxCount := -1
	var winner string
	isTie := false

	for opt, count := range counts {
		if count > maxCount {
			maxCount = count
			winner = opt
			isTie = false
		} else if count == maxCount {
			isTie = true
		}
	}

	if isTie {
		return "", false
	}

	return winner, true
}
