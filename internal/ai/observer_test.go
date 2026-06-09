package ai

import (
	"aonagi/internal/models"
	"aonagi/internal/trips"
	"sync"
	"testing"
	"time"
)

type MockExtractor struct {
	CallCount int
	mu        sync.Mutex
}

func (m *MockExtractor) ExtractTripState(current trips.TripState, history []models.Message, activePollCategories []string, finalizedPollCategories []string) (trips.TripState, []trips.PollSuggestion, []trips.Decision, error) {
	m.mu.Lock()
	m.CallCount++
	m.mu.Unlock()
	return trips.TripState{
		Destination: "Mocked Goa",
		Budget:      "Mocked Budget",
		Summary:     "Consensus updated",
	}, nil, nil, nil
}

func TestObserveDebounce(t *testing.T) {
	mock := &MockExtractor{}
	SetClient(mock)

	// Clean up timers
	debounceMu.Lock()
	for k, timer := range debounceTimers {
		timer.Stop()
		delete(debounceTimers, k)
	}
	debounceMu.Unlock()

	// Clear rooms
	trips.Rooms = make(map[string]*trips.Room)

	// Fire 30 messages rapidly
	for i := 0; i < 30; i++ {
		Observe(models.Message{
			MessageID: "msg-123",
			GroupID:   "goa-trip",
			SenderID:  "user-1",
			Name:      "Alice",
			Body:      "Let's go to Goa",
			TimeStamp: time.Now(),
		})
	}

	// Verify immediately that CallCount is 0 (since it is debounced for 3 seconds)
	mock.mu.Lock()
	immediateCount := mock.CallCount
	mock.mu.Unlock()
	if immediateCount != 0 {
		t.Errorf("Expected immediate call count to be 0 (debounced), got %d", immediateCount)
	}

	// Wait 3.5 seconds for the debounce timer to fire
	time.Sleep(3500 * time.Millisecond)

	mock.mu.Lock()
	finalCount := mock.CallCount
	mock.mu.Unlock()

	if finalCount != 1 {
		t.Errorf("Expected exactly 1 extraction call after debounce quiet period, got %d", finalCount)
	}
}
