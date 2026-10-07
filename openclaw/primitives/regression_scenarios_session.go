package primitives

import (
	"context"

	"github.com/futugyou/openclaw/core"
)

type SessionStoreRoundTripScenario struct {
	BaseScenario
}

func NewSessionStoreRoundTripScenario() *SessionStoreRoundTripScenario {
	s := &SessionStoreRoundTripScenario{
		BaseScenario: NewBaseScenario("sessions.store_round_trip", "Session store round-trip", HarnessRegressionCategorySessions, true),
	}
	s.SetEvaluator(s)
	return s
}

func (s *SessionStoreRoundTripScenario) Evaluate(ctx context.Context, regCtx *HarnessRegressionContext) (HarnessRegressionScenarioResult, error) {
	if ctx.Err() != nil {
		return HarnessRegressionScenarioResult{}, ctx.Err()
	}

	root, err := HarnessRegressionPathsChild(regCtx.TempWorkspacePath, "sessions")
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	store, err := core.NewFileMemoryStore(root, 4, nil, nil, nil)
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}
	session := core.Session{
		Id:        "sess_harness_regression",
		ChannelId: "harness",
		SenderId:  "regression",
	}

	if err := store.SaveSession(ctx, session); err != nil {
		return s.Failed(err.Error(), "", ""), err
	}
	loaded, err := store.GetSession(ctx, session.Id)
	if err != nil {
		return s.Failed(err.Error(), "", ""), err
	}

	if loaded.Id == session.Id &&
		loaded.ChannelId == session.ChannelId &&
		loaded.SenderId == session.SenderId {
		return s.Passed("File session store saved and loaded a session.", ""), nil
	}

	return s.Failed("File session store did not load the saved session.", "", ""), nil
}
