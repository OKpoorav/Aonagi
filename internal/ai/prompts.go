package ai

import (
	"aonagi/internal/models"
	"aonagi/internal/trips"
	"encoding/json"
	"fmt"
	"strings"
)

const systemPrompt = `You are the Aonagi AI Trip Planner. Your task is to analyze the trip planning conversation history and current trip state to extract the updated Trip State, identify any soft decisions made by consensus, and propose poll suggestions when there is active debate, disagreement, or multiple alternatives discussed without a clear consensus.

You must output ONLY a valid JSON object matching the schema below. Do not output markdown, preambles, or additional text. Do NOT wrap the JSON in markdown code blocks.

JSON Structure:
{
  "tripState": {
    "destination": "Name of the destination. If multiple or undecided, write 'TBD' or list comma-separated.",
    "budget": "The budget details or constraints. If TBD, write 'TBD'.",
    "dateRange": "The dates or timeframes. If TBD, write 'TBD'.",
    "preferences": ["List of preferences like 'Beaches', 'Nightlife', etc."],
    "summary": "A concise, single-sentence summary of the current status."
  },
  "decisions": [
    {
      "category": "destination|budget|dates|accommodation|activities|transportation",
      "value": "The agreed value (e.g., 'Goa', '$500', 'July 10-15')",
      "source": "ai_extraction",
      "confidence": 0.8
    }
  ],
  "suggestions": [
    {
      "category": "destination|budget|dates|accommodation|activities|transportation",
      "question": "A clear question for the poll (e.g., 'Which budget range works best?')",
      "options": ["Option 1", "Option 2", "Option 3"]
    }
  ]
}

Guidelines:
1. ONLY generate decisions under "decisions" if there is a clear consensus or agreement in the conversation.
2. ONLY suggest polls under "suggestions" when there is active debate, uncertainty, or multiple options discussed without a clear consensus.
3. Suggestion options should match the options discussed in the chat. Max 5 options per poll suggestion.
4. If a category already has an active poll or a finalized poll decision (listed in the "Exclude Categories" section), do NOT suggest a poll or output a consensus decision for it.`

func buildUserPrompt(current trips.TripState, history []models.Message, activePollCategories []string, finalizedPollCategories []string) string {
	prefBytes, _ := json.Marshal(current.Preferences)

	var sb strings.Builder
	for _, m := range history {
		if m.SenderID == "system" {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Name, m.Body))
	}

	return fmt.Sprintf(`Current Trip State:
{
  "destination": %q,
  "budget": %q,
  "dateRange": %q,
  "preferences": %s,
  "summary": %q
}

Exclude Categories:
- Already active polls: %s
- Already poll-finalized decisions: %s

Conversation History:
%s

Based on the conversation history, update the Trip State. Also identify any consensus decisions (excluding those in the finalized/active categories) and propose relevant poll suggestions for undecided/debated categories (excluding active/finalized categories). Output ONLY the raw JSON matching the required schema. Do NOT wrap the output in markdown code blocks.`,
		current.Destination,
		current.Budget,
		current.DateRange,
		string(prefBytes),
		current.Summary,
		strings.Join(activePollCategories, ", "),
		strings.Join(finalizedPollCategories, ", "),
		sb.String(),
	)
}

