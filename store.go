package main

import (
	"sync"
	"time"
)

// Outcome says what happened when an event was offered to the store.
type Outcome string

const (
	Accepted  Outcome = "accepted"  // new event, counted
	Duplicate Outcome = "duplicate" // same event_id and same content: a retry, not counted
	Conflict  Outcome = "conflict"  // same event_id but different content: first write wins, not counted
)

type campaignStats struct {
	sent, delivered, opened, clicked int
	openedBy, clickedBy              map[string]struct{} // distinct contacts
	lastEventAt                      time.Time           // newest provider timestamp
	lastIngestedAt                   time.Time           // when we last accepted something
}

// Store keeps everything in memory. One RWMutex protects both maps.
// Data is lost on restart (documented trade-off, see NOTES.md).
type Store struct {
	mu        sync.RWMutex
	events    map[string]Event // event_id -> first accepted event; this IS the dedup check
	campaigns map[string]*campaignStats
	now       func() time.Time
}

func NewStore() *Store {
	return &Store{
		events:    make(map[string]Event),
		campaigns: make(map[string]*campaignStats),
		now:       time.Now,
	}
}

// Add is the only write path. The "have I seen this event_id?" check, the
// insert and the counter updates all happen while holding the write lock,
// so two concurrent requests can never both count the same event.
func (s *Store) Add(ev Event) Outcome {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prev, ok := s.events[ev.EventID]; ok {
		if sameEvent(prev, ev) {
			return Duplicate
		}
		return Conflict
	}
	s.events[ev.EventID] = ev

	cs, ok := s.campaigns[ev.CampaignID]
	if !ok {
		cs = &campaignStats{openedBy: map[string]struct{}{}, clickedBy: map[string]struct{}{}}
		s.campaigns[ev.CampaignID] = cs
	}
	switch ev.Type {
	case TypeSent:
		cs.sent++
	case TypeDelivered:
		cs.delivered++
	case TypeOpened:
		cs.opened++
		cs.openedBy[ev.ContactID] = struct{}{}
	case TypeClicked:
		cs.clicked++
		cs.clickedBy[ev.ContactID] = struct{}{}
	}
	if ev.Timestamp.After(cs.lastEventAt) {
		cs.lastEventAt = ev.Timestamp
	}
	cs.lastIngestedAt = s.now().UTC()
	return Accepted
}

// sameEvent decides retry vs conflict. Metadata is ignored on purpose: a
// retry that differs only in metadata is still the same event.
func sameEvent(a, b Event) bool {
	return a.CampaignID == b.CampaignID &&
		a.ContactID == b.ContactID &&
		a.Type == b.Type &&
		a.Timestamp.Equal(b.Timestamp)
}

// Stats is the dashboard response.
type Stats struct {
	CampaignID     string     `json:"campaign_id"`
	Sent           int        `json:"sent"`
	Delivered      int        `json:"delivered"`
	OpenedTotal    int        `json:"opened_total"`
	OpenedUnique   int        `json:"opened_unique"`
	ClickedTotal   int        `json:"clicked_total"`
	ClickedUnique  int        `json:"clicked_unique"`
	LastEventAt    *time.Time `json:"last_event_at"`    // newest provider timestamp (null if no events)
	LastIngestedAt *time.Time `json:"last_ingested_at"` // freshness: when we last accepted an event
}

// Stats returns a snapshot. Counters are kept up to date on write, so this
// is O(1) and never scans events. An unknown campaign returns all zeros.
func (s *Store) Stats(campaignID string) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := Stats{CampaignID: campaignID}
	cs, ok := s.campaigns[campaignID]
	if !ok {
		return out
	}
	out.Sent, out.Delivered = cs.sent, cs.delivered
	out.OpenedTotal, out.OpenedUnique = cs.opened, len(cs.openedBy)
	out.ClickedTotal, out.ClickedUnique = cs.clicked, len(cs.clickedBy)
	le, li := cs.lastEventAt, cs.lastIngestedAt
	out.LastEventAt, out.LastIngestedAt = &le, &li
	return out
}
