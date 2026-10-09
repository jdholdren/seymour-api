package v1

// PutTimelinePromptReq is the request body for PUT /api/users/{userID}/timeline-prompt.
type PutTimelinePromptReq struct {
	// Prompt is limited to 1000 Unicode characters. An empty prompt clears it.
	Prompt string `json:"prompt" maxLength:"1000" description:"Timeline filtering prompt, at most 1000 Unicode characters. An empty, omitted, or null prompt clears it."`
}

// TimelinePromptResp is the response for GET and PUT /api/users/{userID}/timeline-prompt.
type TimelinePromptResp struct {
	Prompt string `json:"prompt"`
}
