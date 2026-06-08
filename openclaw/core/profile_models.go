package core

import "time"

const (
	ProfilesConfigDefaultEnabled        = true
	ProfilesConfigDefaultInjectRecall   = true
	ProfilesConfigDefaultMaxRecallChars = 2000
)

type ProfilesConfig struct {
	Enabled        bool `json:"enabled"`
	InjectRecall   bool `json:"inject_recall"`
	MaxRecallChars int  `json:"max_recall_chars"`
}

func DefaultProfilesConfig() ProfilesConfig {
	return ProfilesConfig{
		Enabled:        ProfilesConfigDefaultEnabled,
		InjectRecall:   ProfilesConfigDefaultInjectRecall,
		MaxRecallChars: ProfilesConfigDefaultMaxRecallChars,
	}
}

// ==========================================
// UserProfileFact
// ==========================================

type UserProfileFact struct {
	Key              string    `json:"key"`
	Value            string    `json:"value"`
	Confidence       float32   `json:"confidence"`
	SourceSessionIds []string  `json:"source_session_ids"`
	UpdatedAtUtc     time.Time `json:"updated_at_utc"`
}

func DefaultUserProfileFact() UserProfileFact {
	return UserProfileFact{
		SourceSessionIds: make([]string, 0),
		UpdatedAtUtc:     time.Now().UTC(),
	}
}

// ==========================================
// UserProfile
// ==========================================

type UserProfile struct {
	ActorId        string            `json:"actor_id"`
	ChannelId      string            `json:"channel_id"`
	SenderId       string            `json:"sender_id"`
	Summary        string            `json:"summary"`
	Tone           string            `json:"tone"`
	Facts          []UserProfileFact `json:"facts"`
	Preferences    []string          `json:"preferences"`
	ActiveProjects []string          `json:"active_projects"`
	RecentIntents  []string          `json:"recent_intents"`
	UpdatedAtUtc   time.Time         `json:"updated_at_utc"`
}

func DefaultUserProfile() UserProfile {
	return UserProfile{
		Facts:          make([]UserProfileFact, 0),
		Preferences:    make([]string, 0),
		ActiveProjects: make([]string, 0),
		RecentIntents:  make([]string, 0),
		UpdatedAtUtc:   time.Now().UTC(),
	}
}
