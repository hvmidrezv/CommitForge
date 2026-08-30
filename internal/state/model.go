// Package state handles persistence of CommitForge session state to JSON.
package state

// PersistedState is the on-disk JSON payload saved to <dir>/.commitforge/state.json.
type PersistedState struct {
	Version             int            `json:"version"`
	SelectedDir         string         `json:"selected_dir"`
	SelectedDates       []string       `json:"selected_dates"`
	DateCounts          map[string]int `json:"date_counts"`
	GeneratedDateCounts map[string]int `json:"generated_date_counts,omitempty"`
	Message             string         `json:"message"`
	MessageMode         string         `json:"message_mode"`
	RemoteURL           string         `json:"remote_url,omitempty"`
}
