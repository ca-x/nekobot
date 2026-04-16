package workspace

type Contract struct {
	Kind       string                       `json:"kind"`
	Validation ValidationContract          `json:"validation"`
	Artifacts  map[string]string           `json:"artifacts,omitempty"`
	SpawnTasks map[string]SpawnTaskContract `json:"spawn_tasks,omitempty"`
}

type ValidationContract struct {
	OnTurnEnd     []string `json:"on_turn_end,omitempty"`
	OnSourceChange []string `json:"on_source_change,omitempty"`
	OnCompletion  []string `json:"on_completion,omitempty"`
}

type SpawnTaskContract struct {
	Artifacts []string `json:"artifacts,omitempty"`
	OnVerify  []string `json:"on_verify,omitempty"`
	OnFailure []string `json:"on_failure,omitempty"`
}

func DefaultSessionContract() Contract {
	return Contract{
		Kind: "session",
		Validation: ValidationContract{
			OnTurnEnd: []string{
				"workspace_bootstrapped",
				"daily_log_present",
			},
			OnCompletion: []string{
				"heartbeat_state_present",
			},
		},
		Artifacts: map[string]string{
			"daily_log":        "memory/YYYY-MM-DD.md",
			"heartbeat_state":  "memory/heartbeat-state.json",
			"workspace_wiki":   "wiki/index.md",
		},
		SpawnTasks: map[string]SpawnTaskContract{
			"fm_tts": {
				Artifacts: []string{"*.mp3"},
				OnVerify: []string{
					"file_exists:$artifact",
					"file_size_min:$artifact:1024",
				},
				OnFailure: []string{"notify_user:TTS generation failed"},
			},
			"podcast_generate": {
				Artifacts: []string{"**/podcast_full_*.*"},
				OnVerify: []string{
					"file_exists:$artifact",
					"file_size_min:$artifact:4096",
				},
				OnFailure: []string{"notify_user:Podcast generation failed"},
			},
		},
	}
}
