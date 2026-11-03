package main

import "time"

// The four event types providers can send.
const (
	TypeSent      = "sent"
	TypeDelivered = "delivered"
	TypeOpened    = "opened"
	TypeClicked   = "clicked"
)

var validTypes = map[string]bool{
	TypeSent: true, TypeDelivered: true, TypeOpened: true, TypeClicked: true,
}

// Event is a validated event. Timestamp is normalised to UTC.
type Event struct {
	EventID    string
	CampaignID string
	ContactID  string
	Type       string
	Timestamp  time.Time
	Metadata   map[string]any
}

// rawEvent is the wire format. Every field is decoded leniently here and
// checked in parseEvent, so a bad value only fails that one event.
type rawEvent struct {
	EventID    string         `json:"event_id"`
	CampaignID string         `json:"campaign_id"`
	ContactID  string         `json:"contact_id"`
	Type       string         `json:"type"`
	Timestamp  string         `json:"timestamp"`
	Metadata   map[string]any `json:"metadata"`
}
