package models

import (
	"strconv"
	"strings"
	"time"
)

type RetrievalLevel uint32

const (
	RetrievalLevelOff        RetrievalLevel = 0
	RetrievalLevelMemoryOnly RetrievalLevel = 1
	RetrievalLevelVectorDb   RetrievalLevel = 2
	RetrievalLevelHybrid     RetrievalLevel = 3
)

func (r RetrievalLevel) Name() string {
	switch r {
	case RetrievalLevelOff:
		return "Off"
	case RetrievalLevelMemoryOnly:
		return "MemoryOnly"
	case RetrievalLevelVectorDb:
		return "VectorDb"
	case RetrievalLevelHybrid:
		return "Hybrid"
	}

	return "Unkown"
}

func StringToRetrievalLevel(value string) RetrievalLevel {
	value = strings.ToLower(value)
	switch value {
	case "off":
		return RetrievalLevelOff
	case "memoryonly":
		return RetrievalLevelMemoryOnly
	case "Vectordb":
		return RetrievalLevelVectorDb
	case "hybrid":
		return RetrievalLevelHybrid
	default:
		if n, err := strconv.Atoi(value); err == nil {
			return RetrievalLevel(n)
		}
		return 0
	}
}

type ProfileKind uint32

const (
	ProfileKindStandard   ProfileKind = 0
	ProfileKindSystem     ProfileKind = 1
	ProfileKindToolTester ProfileKind = 2
)

func (r ProfileKind) Name() string {
	switch r {
	case ProfileKindStandard:
		return "Standard"
	case ProfileKindSystem:
		return "System"
	case ProfileKindToolTester:
		return "ToolTester"
	}

	return "Unkown"
}

func StringToProfileKind(value string) ProfileKind {
	value = strings.ToLower(value)
	switch value {
	case "standard":
		return ProfileKindStandard
	case "system":
		return ProfileKindSystem
	case "tooltester":
		return ProfileKindToolTester
	default:
		if n, err := strconv.Atoi(value); err == nil {
			return ProfileKind(n)
		}
		return 0
	}
}

type AgentProfile struct {
	Name                string
	DisplayName         string
	Provider            string
	Model               string
	Endpoint            string
	ApiKey              string
	DeploymentName      string
	AuthMode            string
	Instructions        string
	EnabledTools        string
	Temperature         float32
	MaxTokens           int
	IsDefault           bool
	LastTestedAt        *time.Time
	LastTestSucceeded   bool
	LastTestError       string
	RetrievalLevel      RetrievalLevel
	Kind                ProfileKind
	RequireToolApproval bool
	IsEnabled           bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func DefaultAgentProfile() *AgentProfile {
	return &AgentProfile{
		RetrievalLevel:      RetrievalLevelOff,
		Kind:                ProfileKindStandard,
		RequireToolApproval: true,
		IsEnabled:           true,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
}
