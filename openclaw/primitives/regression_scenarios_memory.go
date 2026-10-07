package primitives

import (
	"context"
	"slices"

	"github.com/futugyou/openclaw/core"
)

type MemoryStoreRoundTripScenario struct {
	BaseScenario
}

func NewMemoryStoreRoundTripScenario() *MemoryStoreRoundTripScenario {
	s := &MemoryStoreRoundTripScenario{
		BaseScenario: NewBaseScenario("memory.store_round_trip", "Memory store round-trip", HarnessRegressionCategoryMemory, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *MemoryStoreRoundTripScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	root, err := HarnessRegressionPathsChild(regCtx.TempWorkspacePath, "memory")
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	store, err := core.NewFileMemoryStore(root, 4, nil, nil, nil)
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}
	key := "harness:regression:note"
	content := "Harness regression memory note."
	store.SaveNote(ctx, key, content)
	loaded, err := store.LoadNote(ctx, key)
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}
	listed, err := store.ListNotesWithPrefix(ctx, "harness:")
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	var hits []core.MemoryNoteHit
	if o, ok := any(store).(core.IMemoryNoteSearch); ok {
		hits, err = o.SearchNotes(ctx, "regression memory", "harness:", 5)
		if err != nil {
			return s.Failed(err.Error(), "", ""), err
		}
	}

	if content != loaded {
		return s.Failed("Memory note did not load with the saved content.", "", ""), nil
	}
	if !slices.Contains(listed, key) {
		return s.Failed("Memory note prefix listing did not include the saved note.", "", ""), nil
	}

	find := false
	for _, v := range hits {
		if v.Key == key {
			find = true
			break
		}
	}

	if !find {
		return s.Failed("Memory note search did not find the saved note.", "", ""), nil
	}

	return s.Passed("High-risk tool descriptors and supervised approval policy remain approval-first.", ""), nil
}
