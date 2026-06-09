package trips

import (
	"aonagi/internal/models"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestGetOrCreateRoom(t *testing.T) {
	// Clear the global store
	roomsMu.Lock()
	Rooms = make(map[string]*Room)
	roomsMu.Unlock()

	room1 := GetOrCreateRoom("room-a")
	if room1 == nil {
		t.Fatal("Expected room-a to be created, got nil")
	}
	if room1.ID != "room-a" {
		t.Errorf("Expected room ID to be 'room-a', got %s", room1.ID)
	}

	room2 := GetOrCreateRoom("room-a")
	if room2 != room1 {
		t.Error("Expected GetOrCreateRoom to return the same room instance on subsequent calls")
	}

	room3 := GetOrCreateRoom("room-b")
	if room3 == room1 {
		t.Error("Expected GetOrCreateRoom to return a different room instance for room-b")
	}
}

func TestRoomMessageHistoryAndSafety(t *testing.T) {
	room := GetOrCreateRoom("test-room-history")

	// Verify initial empty history
	if len(room.GetMessages()) != 0 {
		t.Error("Expected message history to start empty")
	}

	// Concurrent message additions
	const numGoroutines = 50
	const numMessagesPerGoroutine = 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < numMessagesPerGoroutine; j++ {
				room.AddMessage(models.Message{
					MessageID: fmt.Sprintf("msg-%d-%d", routineID, j),
					GroupID:   "test-room-history",
					SenderID:  fmt.Sprintf("user-%d", routineID),
					Name:      fmt.Sprintf("User %d", routineID),
					Body:      fmt.Sprintf("Hello message %d", j),
					TimeStamp: time.Now(),
				})
			}
		}(i)
	}

	wg.Wait()

	// Verify total count
	messages := room.GetMessages()
	expectedTotal := numGoroutines * numMessagesPerGoroutine
	if len(messages) != expectedTotal {
		t.Errorf("Expected total messages to be %d, got %d", expectedTotal, len(messages))
	}
}

func TestRoomTripStateSafety(t *testing.T) {
	room := GetOrCreateRoom("test-room-state")

	initialState := room.GetTripState()
	if initialState.Destination != "TBD" {
		t.Errorf("Expected initial destination 'TBD', got %s", initialState.Destination)
	}

	// Concurrent state updates
	const numUpdates = 100
	var wg sync.WaitGroup

	for i := 0; i < numUpdates; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			room.UpdateTripState(TripState{
				Destination: fmt.Sprintf("Destination %d", idx),
				Budget:      fmt.Sprintf("%d", idx*1000),
				DateRange:   "December",
				Preferences: []string{"Beaches"},
				Summary:     "Test update",
			})
			_ = room.GetTripState() // Read concurrently
		}(i)
	}

	wg.Wait()

	finalState := room.GetTripState()
	if finalState.DateRange != "December" {
		t.Errorf("Expected final state DateRange to be 'December', got %s", finalState.DateRange)
	}
}

func TestDecisionsAndSuggestions(t *testing.T) {
	// Clear the global store
	roomsMu.Lock()
	Rooms = make(map[string]*Room)
	roomsMu.Unlock()

	room := GetOrCreateRoom("test-decisions-room")

	// Verify initial decisions and suggestions are empty
	if len(room.GetDecisions()) != 0 {
		t.Error("Expected decisions to start empty")
	}
	if len(room.GetSuggestions()) != 0 {
		t.Error("Expected suggestions to start empty")
	}

	// 1. Update with AI decisions and suggestions
	aiDecisions := []Decision{
		{Category: CategoryDestination, Value: "Paris", Source: "ai_extraction", Confidence: 0.8},
		{Category: CategoryBudget, Value: "1000", Source: "ai_extraction", Confidence: 0.7},
	}
	aiSuggestions := []PollSuggestion{
		{Category: CategoryDates, Question: "When should we go?", Options: []string{"July", "August"}},
	}

	room.UpdateDecisionsAndSuggestions(aiDecisions, aiSuggestions)

	decs := room.GetDecisions()
	if len(decs) != 2 {
		t.Errorf("Expected 2 decisions, got %d", len(decs))
	}
	if decs[CategoryDestination].Value != "Paris" {
		t.Errorf("Expected Destination to be Paris, got %s", decs[CategoryDestination].Value)
	}

	sugs := room.GetSuggestions()
	if len(sugs) != 1 {
		t.Errorf("Expected 1 suggestion, got %d", len(sugs))
	}

	// 2. Create a poll for CategoryDates and check if its suggestion is filtered out
	_, err := room.CreatePoll("When should we go?", CategoryDates, []string{"July", "August"}, "user1")
	if err != nil {
		t.Fatalf("Failed to create poll: %v", err)
	}

	// Add suggestion for CategoryDates again via update, but it should be dynamically filtered out in GetSuggestions because active poll exists
	room.UpdateDecisionsAndSuggestions(nil, []PollSuggestion{
		{Category: CategoryDates, Question: "When should we go?", Options: []string{"July", "August"}},
	})
	if len(room.GetSuggestions()) != 0 {
		t.Error("Expected suggestions for Dates to be filtered out because there is an active poll")
	}

	// 3. Vote and close poll with clear winner
	polls := room.GetPolls()
	var datesPoll *Poll
	for _, p := range polls {
		if p.Category == CategoryDates {
			datesPoll = p
			break
		}
	}
	if datesPoll == nil {
		t.Fatal("Could not find created dates poll")
	}

	err = room.VotePoll(datesPoll.ID, "user1", "July")
	if err != nil {
		t.Fatalf("Failed to vote: %v", err)
	}
	
	_, err = room.ClosePoll(datesPoll.ID, "user1")
	if err != nil {
		t.Fatalf("Failed to close poll: %v", err)
	}

	// Check that a poll-sourced decision is stored
	decs = room.GetDecisions()
	datesDec, hasDatesDec := decs[CategoryDates]
	if !hasDatesDec {
		t.Fatal("Expected a decision for dates after closing poll")
	}
	if datesDec.Source != "poll" || datesDec.Value != "July" {
		t.Errorf("Expected poll decision of July, got source=%s, value=%s", datesDec.Source, datesDec.Value)
	}

	// Check TripState matches the poll winner
	ts := room.GetTripState()
	if ts.DateRange != "July" {
		t.Errorf("Expected TripState.DateRange to update to July, got %s", ts.DateRange)
	}

	// 4. Test conflict resolution: AI update should NOT overwrite the poll decision
	aiDecisions = []Decision{
		{Category: CategoryDates, Value: "August", Source: "ai_extraction", Confidence: 0.9},
		{Category: CategoryDestination, Value: "London", Source: "ai_extraction", Confidence: 0.9},
	}
	room.UpdateDecisionsAndSuggestions(aiDecisions, nil)

	decs = room.GetDecisions()
	if decs[CategoryDates].Value != "July" {
		t.Errorf("Expected Dates to remain July (poll winner protected), got %s", decs[CategoryDates].Value)
	}
	if decs[CategoryDestination].Value != "London" {
		t.Errorf("Expected Destination to update to London, got %s", decs[CategoryDestination].Value)
	}

	// Also verify that updating the TripState via AI is blocked for Dates
	room.UpdateTripState(TripState{
		Destination: "London",
		DateRange:   "August",
	})
	ts = room.GetTripState()
	if ts.DateRange != "July" {
		t.Errorf("Expected TripState.DateRange to remain July (protected by poll decision), got %s", ts.DateRange)
	}
	if ts.Destination != "London" {
		t.Errorf("Expected TripState.Destination to update to London, got %s", ts.Destination)
	}
}
