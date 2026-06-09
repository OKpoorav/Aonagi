package ai

import (
	"aonagi/internal/models"
	"aonagi/internal/trips"
	"aonagi/internal/ws"
	"log"
	"sync"
	"time"
)

type TripExtractor interface {
	ExtractTripState(current trips.TripState, history []models.Message, activePollCategories []string, finalizedPollCategories []string) (trips.TripState, []trips.PollSuggestion, []trips.Decision, error)
}

var (
	aiClient       TripExtractor
	clientMu       sync.Mutex
	debounceTimers = make(map[string]*time.Timer)
	debounceMu     sync.Mutex
)

func SetClient(c TripExtractor) {
	clientMu.Lock()
	defer clientMu.Unlock()
	aiClient = c
}

func getClient() (TripExtractor, error) {
	clientMu.Lock()
	defer clientMu.Unlock()
	if aiClient != nil {
		return aiClient, nil
	}
	c, err := NewClient()
	if err != nil {
		return nil, err
	}
	aiClient = c
	return aiClient, nil
}

func Observe(msg models.Message) {
	if msg.SenderID == "system" {
		return
	}

	room := trips.GetOrCreateRoom(msg.GroupID)

	debounceMu.Lock()
	timer, ok := debounceTimers[room.ID]
	if ok {
		timer.Stop()
	}

	// Schedule execution after 3 seconds quiet period
	debounceTimers[room.ID] = time.AfterFunc(3*time.Second, func() {
		runExtraction(room)
	})
	debounceMu.Unlock()
}

func runExtraction(room *trips.Room) {
	currentState := room.GetTripState()
	history := room.GetMessages()

	// Fetch or initialize the client
	client, err := getClient()
	if err != nil {
		log.Printf("AI client initialization error: %v\n", err)
		return
	}

	// Find active poll categories
	activePollCategories := []string{}
	for _, p := range room.GetPolls() {
		if p.Status == "active" {
			activePollCategories = append(activePollCategories, p.Category)
		}
	}

	// Find finalized poll categories
	finalizedPollCategories := []string{}
	for cat, dec := range room.GetDecisions() {
		if dec.Source == "poll" {
			finalizedPollCategories = append(finalizedPollCategories, cat)
		}
	}

	// Call Anthropic to update trip state, decisions and suggestions
	updatedState, suggestions, decisions, err := client.ExtractTripState(currentState, history, activePollCategories, finalizedPollCategories)
	if err != nil {
		log.Printf("AI Observer extraction error for room %s: %v\n", room.ID, err)
		return
	}

	// Update the room state (respecting override protection)
	room.UpdateTripState(updatedState)
	room.UpdateDecisionsAndSuggestions(decisions, suggestions)

	// Broadcast the updated models to the room
	ws.HubInstance.BroadCastTripState(room.ID, room.GetTripState())
	ws.HubInstance.BroadCastDecisions(room.ID, room.GetDecisions())
	ws.HubInstance.BroadCastSuggestions(room.ID, room.GetSuggestions())
}
