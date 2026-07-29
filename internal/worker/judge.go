package worker

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

//go:embed system_prompt.txt
var systemPrompt string

//go:embed user_criteria.txt
var userCriteria string

type claudeJudgement struct {
	FeedEntryID string `json:"feed_entry_id"`
	Approved    bool   `json:"approved"`
}

// Use a schema to constrain the output
var (
	outputSchema = map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"feed_entry_id": map[string]any{"type": "string"},
				"approved":      map[string]any{"type": "boolean"},
			},
			"required": []string{"feed_entry_id", "approved"},
		},
	}
	outputFormat = anthropic.BetaJSONSchemaOutputFormat(outputSchema)
)

// JudgeEntries fetches the entries in need of judgement and judges them.
//
// If an active prompt exists, entries are sent to Claude for curation.
// If no active prompt exists, all entries are auto-approved.
func (a activities) JudgeEntries(ctx context.Context) (judgements, error) {
	l := activity.GetLogger(ctx)

	// Need to limit this in case we pull too many results.
	entries, err := a.repo.EntriesNeedingJudgement(ctx, 20)
	if err != nil {
		return nil, fmt.Errorf("error finding needing judgement timeline entries: %s", err)
	}

	l.Info("judging entries", "count", len(entries))

	// If no entries to judge, return empty result
	if len(entries) == 0 {
		return nil, nil
	}

	// Check for an active prompt
	prompt, err := a.repo.ActivePrompt(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching active prompt: %w", err)
	}

	// No active prompt — auto-approve all entries
	if prompt == nil {
		j := make(judgements)
		for _, entry := range entries {
			j[entry.ID] = true
		}
		return j, nil
	}

	// Build the lookup maps and collect feed entry IDs
	var (
		entryIDs            []string
		feedEntryToTimeline = make(map[string]string)
	)
	for _, entry := range entries {
		entryIDs = append(entryIDs, entry.FeedEntryID)
		feedEntryToTimeline[entry.FeedEntryID] = entry.ID
	}

	// Fetch the full feed entries and construct the user message
	feedEntries, err := a.repo.Entries(ctx, entryIDs)
	if err != nil {
		return nil, fmt.Errorf("error fetching feed entries: %w", err)
	}
	byts, _ := json.Marshal(feedEntries)
	userMessage := fmt.Sprintf(userCriteria, prompt.Content, string(byts))

	// Call Claude to judge the entries
	claudeResp, err := a.claudeClient.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model: anthropic.ModelClaudeHaiku4_5,
		Betas: []anthropic.AnthropicBeta{
			"structured-outputs-2025-11-13",
		},
		MaxTokens:    1024,
		OutputFormat: outputFormat,
		System: []anthropic.BetaTextBlockParam{{
			Text: systemPrompt,
		}},
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(userMessage)),
		},
	})
	// Handle Anthropic rate limit errors
	var claudeErr *anthropic.Error
	if errors.As(err, &claudeErr) && claudeErr.StatusCode == http.StatusTooManyRequests {
		return nil, temporal.NewApplicationError("rate limit hit", errTypeRateLimit, err)
	}
	if err != nil {
		return nil, temporal.NewApplicationError("claude error", errTypeInternal, err)
	}

	var claudeJson strings.Builder
	for _, content := range claudeResp.Content {
		claudeJson.WriteString(content.Text)
	}
	var claudeJudgements []claudeJudgement
	if err := json.Unmarshal([]byte(claudeJson.String()), &claudeJudgements); err != nil {
		return nil, fmt.Errorf("error unmarshaling claude json: %s", err)
	}

	j := make(judgements)
	for _, judgement := range claudeJudgements {
		j[feedEntryToTimeline[judgement.FeedEntryID]] = judgement.Approved
	}
	return j, nil
}
