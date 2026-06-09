package trips

type TripState struct {
	Destination string   `json:"destination"`
	Budget      string   `json:"budget"`
	DateRange   string   `json:"dateRange"`
	Preferences []string `json:"preferences"`
	Summary     string   `json:"summary"`
}
