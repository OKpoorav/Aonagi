package trips

import (
	"aonagi/internal/models"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"
)

type RoomMetadata struct {
	CreatedAt        time.Time `json:"createdAt"`
	LastUpdated      time.Time `json:"lastUpdated"`
	ParticipantCount int       `json:"participantCount"`
	LastAIUpdate     time.Time `json:"lastAIUpdate"`
}

type Room struct {
	ID          string            `json:"id"`
	Messages    []models.Message  `json:"messages"`
	TripState   TripState         `json:"tripState"`
	Polls       map[string]*Poll  `json:"polls"`
	Decisions   map[string]Decision `json:"decisions"`
	Suggestions []PollSuggestion  `json:"suggestions"`
	Metadata    RoomMetadata      `json:"metadata"`
	mu          sync.RWMutex
}

var (
	Rooms   = make(map[string]*Room)
	roomsMu sync.RWMutex
)

func generatePollID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func GetOrCreateRoom(roomID string) *Room {
	roomsMu.Lock()
	defer roomsMu.Unlock()
	if room, ok := Rooms[roomID]; ok {
		return room
	}
	room := &Room{
		ID:       roomID,
		Messages: []models.Message{},
		TripState: TripState{
			Destination: "TBD",
			Budget:      "TBD",
			DateRange:   "TBD",
			Preferences: []string{},
			Summary:     "No details discussed yet.",
		},
		Polls:       make(map[string]*Poll),
		Decisions:   make(map[string]Decision),
		Suggestions: []PollSuggestion{},
		Metadata: RoomMetadata{
			CreatedAt:    time.Now(),
			LastUpdated:  time.Now(),
			LastAIUpdate: time.Now(),
		},
	}
	Rooms[roomID] = room
	return room
}

func (r *Room) AddMessage(msg models.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Messages = append(r.Messages, msg)
	r.Metadata.LastUpdated = time.Now()
}

func (r *Room) GetMessages() []models.Message {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to avoid race conditions
	msgs := make([]models.Message, len(r.Messages))
	copy(msgs, r.Messages)
	return msgs
}

func (r *Room) UpdateTripState(ts TripState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Apply override protection
	if dec, ok := r.Decisions[CategoryDestination]; ok && dec.Source == "poll" {
		ts.Destination = dec.Value
	}
	if dec, ok := r.Decisions[CategoryBudget]; ok && dec.Source == "poll" {
		ts.Budget = dec.Value
	}
	if dec, ok := r.Decisions[CategoryDates]; ok && dec.Source == "poll" {
		ts.DateRange = dec.Value
	}
	if dec, ok := r.Decisions[CategoryActivities]; ok && dec.Source == "poll" {
		// Ensure the poll-decided activity is in the list
		found := false
		for _, p := range ts.Preferences {
			if p == dec.Value {
				found = true
				break
			}
		}
		if !found && dec.Value != "Decision Unresolved" {
			ts.Preferences = append(ts.Preferences, dec.Value)
		}
	}

	r.TripState = ts
	r.Metadata.LastAIUpdate = time.Now()
}

func (r *Room) GetTripState() TripState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.TripState
}

func (r *Room) CreatePoll(question, category string, options []string, createdBy string) (*Poll, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	category = strings.ToLower(strings.TrimSpace(category))
	if !IsValidCategory(category) {
		return nil, fmt.Errorf("invalid poll category: %s", category)
	}

	// Rule 1: Max 3 active polls per room
	activeCount := 0
	for _, p := range r.Polls {
		if p.Status == "active" {
			activeCount++
		}
	}
	if activeCount >= 3 {
		return nil, fmt.Errorf("maximum of 3 active polls allowed per room")
	}

	// Rule 2: Exclusivity (at most 1 active poll per category)
	for _, p := range r.Polls {
		if p.Status == "active" && p.Category == category {
			return nil, fmt.Errorf("an active poll already exists for category: %s", category)
		}
	}

	poll := &Poll{
		ID:        generatePollID(),
		RoomID:    r.ID,
		Question:  question,
		Category:  category,
		Options:   options,
		Votes:     make(map[string]string),
		Status:    "active",
		CreatedBy: createdBy,
		CreatedAt: time.Now(),
	}

	r.Polls[poll.ID] = poll
	r.Metadata.LastUpdated = time.Now()
	return poll, nil
}

func (r *Room) VotePoll(pollID, userID, option string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.Polls[pollID]
	if !ok {
		return fmt.Errorf("poll not found")
	}
	if p.Status != "active" {
		return fmt.Errorf("cannot vote on a closed poll")
	}

	validOption := false
	for _, opt := range p.Options {
		if opt == option {
			validOption = true
			break
		}
	}
	if !validOption {
		return fmt.Errorf("invalid option selected: %s", option)
	}

	p.Votes[userID] = option
	r.Metadata.LastUpdated = time.Now()
	return nil
}

func (r *Room) ClosePoll(pollID string, closerID string) (*Poll, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.Polls[pollID]
	if !ok {
		return nil, fmt.Errorf("poll not found")
	}
	if p.Status != "active" {
		return nil, fmt.Errorf("poll is already closed")
	}

	// Rule: Only creator can close
	if p.CreatedBy != closerID {
		return nil, fmt.Errorf("only the poll creator can close it")
	}

	p.Status = "closed"
	p.ClosedAt = time.Now()

	// Update TripState and Decisions based on winner
	winner, hasWinner := p.GetWinner()
	if hasWinner {
		switch p.Category {
		case CategoryDestination:
			r.TripState.Destination = winner
		case CategoryBudget:
			r.TripState.Budget = winner
		case CategoryDates:
			r.TripState.DateRange = winner
		case CategoryActivities:
			found := false
			for _, pref := range r.TripState.Preferences {
				if pref == winner {
					found = true
					break
				}
			}
			if !found {
				r.TripState.Preferences = append(r.TripState.Preferences, winner)
			}
		}
		
		r.Decisions[p.Category] = Decision{
			Category:   p.Category,
			Value:      winner,
			Source:     "poll",
			Timestamp:  time.Now(),
			Confidence: 1.0,
		}
		r.Metadata.LastAIUpdate = time.Now()
	} else {
		// Tie or no votes - mark as Decision Unresolved under poll source
		r.Decisions[p.Category] = Decision{
			Category:   p.Category,
			Value:      "Decision Unresolved",
			Source:     "poll",
			Timestamp:  time.Now(),
			Confidence: 0.0,
		}
		switch p.Category {
		case CategoryDestination:
			r.TripState.Destination = "Decision Unresolved"
		case CategoryBudget:
			r.TripState.Budget = "Decision Unresolved"
		case CategoryDates:
			r.TripState.DateRange = "Decision Unresolved"
		}
		r.Metadata.LastAIUpdate = time.Now()
	}

	r.Metadata.LastUpdated = time.Now()
	return p, nil
}

func (r *Room) GetPolls() []*Poll {
	r.mu.RLock()
	defer r.mu.RUnlock()

	polls := make([]*Poll, 0, len(r.Polls))
	for _, p := range r.Polls {
		polls = append(polls, p)
	}
	return polls
}

func (r *Room) UpdateDecisionsAndSuggestions(aiDecisions []Decision, aiSuggestions []PollSuggestion) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Merge decisions
	for _, aiDec := range aiDecisions {
		category := strings.ToLower(strings.TrimSpace(aiDec.Category))
		if !IsValidCategory(category) {
			continue
		}

		existing, hasExisting := r.Decisions[category]
		if hasExisting && existing.Source == "poll" {
			// Do not overwrite poll-finalized decisions
			continue
		}

		// Set or overwrite AI consensus decisions
		aiDec.Category = category
		aiDec.Timestamp = time.Now()
		r.Decisions[category] = aiDec
	}

	// 2. Set suggestions directly
	// Filter out suggestions for categories that already have active polls or poll-finalized decisions
	filteredSuggestions := []PollSuggestion{}
	for _, sugg := range aiSuggestions {
		cat := strings.ToLower(strings.TrimSpace(sugg.Category))
		if !IsValidCategory(cat) {
			continue
		}

		// Check if there is an active poll for this category
		hasActivePoll := false
		for _, p := range r.Polls {
			if p.Category == cat && p.Status == "active" {
				hasActivePoll = true
				break
			}
		}

		// Check if we already have a poll decision for this category
		dec, hasDec := r.Decisions[cat]
		hasPollDec := hasDec && dec.Source == "poll"

		if !hasActivePoll && !hasPollDec {
			sugg.Category = cat
			filteredSuggestions = append(filteredSuggestions, sugg)
		}
	}
	r.Suggestions = filteredSuggestions
	r.Metadata.LastUpdated = time.Now()
}

func (r *Room) GetDecisions() map[string]Decision {
	r.mu.RLock()
	defer r.mu.RUnlock()

	copyDec := make(map[string]Decision)
	for k, v := range r.Decisions {
		copyDec[k] = v
	}
	return copyDec
}

func (r *Room) GetSuggestions() []PollSuggestion {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filtered := []PollSuggestion{}
	for _, sugg := range r.Suggestions {
		cat := sugg.Category

		// Check if there is an active poll for this category
		hasActivePoll := false
		for _, p := range r.Polls {
			if p.Category == cat && p.Status == "active" {
				hasActivePoll = true
				break
			}
		}

		// Check if we already have a poll decision for this category
		dec, hasDec := r.Decisions[cat]
		hasPollDec := hasDec && dec.Source == "poll"

		if !hasActivePoll && !hasPollDec {
			filtered = append(filtered, sugg)
		}
	}
	return filtered
}
