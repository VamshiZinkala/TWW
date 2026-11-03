package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
)

const maxBodyBytes = 5 << 20 // 5 MB per request

type API struct{ store *Store }

func newMux(store *Store) http.Handler {
	a := &API{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", a.postEvents)
	mux.HandleFunc("GET /campaigns/{campaignID}/stats", a.getStats)
	return mux
}

type itemResult struct {
	Index   int    `json:"index"`
	EventID string `json:"event_id,omitempty"`
	Status  string `json:"status"` // accepted | duplicate | conflict | rejected
	Reason  string `json:"reason,omitempty"`
}

type ingestResponse struct {
	Accepted  int          `json:"accepted"`
	Duplicate int          `json:"duplicate"`
	Conflict  int          `json:"conflict"`
	Rejected  int          `json:"rejected"`
	Results   []itemResult `json:"results"`
}

// postEvents handles a batch. Design:
//   - Body is decoded as []json.RawMessage, so each event is parsed on its
//     own and one bad event cannot fail the rest of the batch.
//   - If the body is a JSON array we always answer 200 with a per-item
//     result. Providers retry on non-2xx, so a 4xx for one bad item would
//     make them resend the whole batch forever.
//   - 400 only when the body is not a JSON array at all; 413 if too large.
func (a *API) postEvents(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)

	var items []json.RawMessage
	if err := dec.Decode(&items); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be a JSON array of events"})
		return
	}
	if items == nil { // body was the literal `null`
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be a JSON array of events"})
		return
	}
	if _, err := dec.Token(); err != io.EOF { // anything after the array
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unexpected data after JSON array"})
		return
	}

	resp := ingestResponse{Results: make([]itemResult, 0, len(items))}
	for i, raw := range items {
		ev, err := parseEvent(raw)
		if err != nil {
			resp.Rejected++
			resp.Results = append(resp.Results, itemResult{Index: i, EventID: peekEventID(raw), Status: "rejected", Reason: err.Error()})
			continue
		}
		res := itemResult{Index: i, EventID: ev.EventID}
		switch a.store.Add(ev) {
		case Accepted:
			resp.Accepted++
			res.Status = "accepted"
		case Duplicate:
			resp.Duplicate++
			res.Status = "duplicate"
		case Conflict:
			resp.Conflict++
			res.Status = "conflict"
			res.Reason = "event_id already stored with different content; first write wins"
			log.Printf("conflict: event_id=%s campaign=%s type=%s", ev.EventID, ev.CampaignID, ev.Type)
		}
		resp.Results = append(resp.Results, res)
	}
	writeJSON(w, http.StatusOK, resp)
}

// peekEventID is best effort, only so a rejected item can be matched up.
func peekEventID(raw json.RawMessage) string {
	var p struct {
		EventID string `json:"event_id"`
	}
	_ = json.Unmarshal(raw, &p)
	return p.EventID
}

func (a *API) getStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.Stats(r.PathValue("campaignID")))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}
