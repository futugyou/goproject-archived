package pluginkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type EntityRef struct {
	Type string
	ID   string
}

func ParseEntityRef(s string) (EntityRef, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return EntityRef{}, fmt.Errorf("invalid entity format '%s', expected 'type:id'", s)
	}
	return EntityRef{Type: parts[0], ID: parts[1]}, nil
}

func (e EntityRef) String() string {
	return e.Type + ":" + e.ID
}

type TemporalTriple struct {
	Subject   EntityRef
	Predicate string
	Object    EntityRef
	ValidFrom time.Time
	ValidTo   *time.Time
	CreatedAt time.Time
}

type TimelineEvent struct {
	At        time.Time
	Direction string // "outgoing" | "incoming"
	Predicate string
	Other     EntityRef
}

type IKnowledgeGraph interface {
	Add(ctx context.Context, triple TemporalTriple) (int64, error)
	Query(ctx context.Context, subject, predicate, object *EntityRef, at *time.Time) ([]TemporalTriple, error)
	Timeline(ctx context.Context, entity EntityRef, from, to *time.Time) ([]TimelineEvent, error)
}
