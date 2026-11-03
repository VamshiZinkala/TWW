package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// parseEvent decodes and validates ONE event. It never panics and never
// affects any other event in the batch; the error text becomes the
// "reason" returned to the provider for that item.
func parseEvent(data json.RawMessage) (Event, error) {
	var r rawEvent
	if err := json.Unmarshal(data, &r); err != nil {
		return Event{}, fmt.Errorf("not a valid event object: %v", err)
	}
	for _, f := range []struct{ name, val string }{
		{"event_id", r.EventID},
		{"campaign_id", r.CampaignID},
		{"contact_id", r.ContactID},
		{"type", r.Type},
		{"timestamp", r.Timestamp},
	} {
		if strings.TrimSpace(f.val) == "" {
			return Event{}, fmt.Errorf("missing or empty required field %q", f.name)
		}
	}
	// Strict on purpose: "OPENED" is rejected, not normalised (see NOTES.md).
	if !validTypes[r.Type] {
		return Event{}, fmt.Errorf("unknown event type %q (want sent|delivered|opened|clicked)", r.Type)
	}
	ts, err := time.Parse(time.RFC3339, r.Timestamp)
	if err != nil {
		return Event{}, fmt.Errorf("timestamp %q is not RFC3339", r.Timestamp)
	}
	return Event{
		EventID:    r.EventID,
		CampaignID: r.CampaignID,
		ContactID:  r.ContactID,
		Type:       r.Type,
		Timestamp:  ts.UTC(),
		Metadata:   r.Metadata,
	}, nil
}
