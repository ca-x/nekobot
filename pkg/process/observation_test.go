package process

import "testing"

func TestLooksLikeAwaitingInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"y/n bracket", "Do you want to continue? [y/n]", true},
		{"y/N bracket", "Proceed? [y/N]", true},
		{"continue y/n", "Do you want to continue? [y/n]", true},
		{"press enter", "Press enter to continue", true},
		{"hit any key", "Hit any key to proceed", true},
		{"yes/no", "Overwrite file? yes/no", true},
		{"confirm", "Please confirm: ", true},
		{"confirm question", "Do you want to confirm?", true},
		{"retry", "Operation failed. Retry?", true},
		{"overwrite", "File exists. Overwrite?", true},
		{"delete", "Delete this file? ", true},
		{"remove", "Remove directory? ", true},
		{"proceed", "Do you want to proceed?", true},
		{"is this ok", "Is this OK? [Y/n]", true},
		{"are you sure", "Are you sure you want to delete?", true},
		{"would you like", "Would you like to continue?", true},
		{"normal output", "Processing file... Done.", false},
		{"completed", "Operation completed successfully.", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeAwaitingInput(tt.input)
			if got != tt.expected {
				t.Errorf("looksLikeAwaitingInput(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLooksLikeMenuPrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"numbered options 1. 2.", "Select option:\n1. Option A\n2. Option B", true},
		{"numbered options 1) 2)", "Choose:\n1) Option A\n2) Option B", true},
		{"select option", "Please select option:", true},
		{"choose an option", "Please choose an option:", true},
		{"reply select", "Reply /select to choose", true},
		{"select colon", "Select: ", true},
		{"choose colon", "Choose: ", true},
		{"options colon", "Options: ", true},
		{"menu colon", "Menu: ", true},
		{"pick an option", "Pick an option:", true},
		{"enter your choice", "Enter your choice: ", true},
		{"select one", "Please select one: ", true},
		{"which option", "Which option do you prefer?", true},
		{"bracket numbers", "Select [1], [2], or [3]", true},
		{"normal output", "Processing...", false},
		{"single option", "1. Option A only", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeMenuPrompt(tt.input)
			if got != tt.expected {
				t.Errorf("looksLikeMenuPrompt(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLooksLikeErrorPrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"error colon", "Error: file not found", true},
		{"failed colon", "Failed: connection refused", true},
		{"permission denied", "Permission denied", true},
		{"fatal", "Fatal: out of memory", true},
		{"panic", "Panic: runtime error", true},
		{"exception", "Exception: null pointer", true},
		{"cannot", "Cannot open file", true},
		{"unable to", "Unable to connect", true},
		{"not found", "File not found", true},
		{"does not exist", "Directory does not exist", true},
		{"is required", "Field is required", true},
		{"invalid", "Invalid input", true},
		{"unrecognized", "Unrecognized command", true},
		{"unknown command", "Unknown command: foo", true},
		{"syntax error", "Syntax error at line 5", true},
		{"connection refused", "Connection refused", true},
		{"timed out", "Operation timed out", true},
		{"timeout", "Request timeout", true},
		{"normal output", "Processing complete", false},
		{"success", "Success: file created", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeErrorPrompt(tt.input)
			if got != tt.expected {
				t.Errorf("looksLikeErrorPrompt(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestClassifyObservation(t *testing.T) {
	tests := []struct {
		name          string
		chunks        []string
		expectedState string
	}{
		{
			name:          "awaiting input",
			chunks:        []string{"Do you want to continue? [y/n] "},
			expectedState: "awaiting_input",
		},
		{
			name:          "menu prompt",
			chunks:        []string{"Select option:\n1. Option A\n2. Option B"},
			expectedState: "menu_prompt",
		},
		{
			name:          "error prompt",
			chunks:        []string{"Error: file not found"},
			expectedState: "error_prompt",
		},
		{
			name:          "idle",
			chunks:        []string{"Processing complete. Output saved."},
			expectedState: "idle",
		},
		{
			name:          "empty",
			chunks:        []string{},
			expectedState: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := classifyObservation(tt.chunks)
			if obs.State != tt.expectedState {
				t.Errorf("classifyObservation(%v).State = %q, want %q", tt.chunks, obs.State, tt.expectedState)
			}
		})
	}
}
