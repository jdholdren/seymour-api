package worker

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

// judgeBatchSize is how many entries a single judgement pass handles.
const judgeBatchSize = 20

// JudgeEntries fetches the entries in need of judgement and judges them.
//
// There is currently no curation implementation, so every entry is approved.
// Swap the body of the judging loop to introduce a real judge.
func (a activities) JudgeEntries(ctx context.Context) (judgements, error) {
	l := activity.GetLogger(ctx)

	// Need to limit this in case we pull too many results.
	entries, err := a.repo.EntriesNeedingJudgement(ctx, judgeBatchSize)
	if err != nil {
		return nil, fmt.Errorf("error finding needing judgement timeline entries: %s", err)
	}

	l.Info("judging entries", "count", len(entries))

	// If no entries to judge, return empty result
	if len(entries) == 0 {
		return nil, nil
	}

	j := make(judgements, len(entries))
	for _, entry := range entries {
		j[entry.ID] = true
	}

	return j, nil
}
