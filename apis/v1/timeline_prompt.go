package v1

// PutTimelinePromptReq is the request body for PUT /api/users/{userID}/timeline-prompt.
type PutTimelinePromptReq struct {
	// Prompt is limited to 16 KiB of decoded UTF-8 bytes. An empty prompt clears it.
	Prompt string `json:"prompt" description:"Timeline filtering prompt, at most 16 KiB of decoded UTF-8 bytes. An empty, omitted, or null prompt clears it."`
}

// TimelinePromptResp is the response for GET and PUT /api/users/{userID}/timeline-prompt.
type TimelinePromptResp struct {
	Prompt string `json:"prompt"`
}
