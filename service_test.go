package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- helpers ---------------------------------------------------------

func post(t *testing.T, h http.Handler, body string) (int, ingestResponse) {
	t.Helper()
	req := httptest.NewRequest("POST", "/events", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var resp ingestResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp
}

func stats(t *testing.T, h http.Handler, campaign string) Stats {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/campaigns/"+campaign+"/stats", nil))
	if rec.Code != 200 {
		t.Fatalf("stats status = %d", rec.Code)
	}
	var s Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func ev(id, camp, contact, typ string) string {
	return fmt.Sprintf(`{"event_id":%q,"campaign_id":%q,"contact_id":%q,"type":%q,"timestamp":"2026-08-10T09:00:00Z"}`, id, camp, contact, typ)
}

func arr(items ...string) string { return "[" + strings.Join(items, ",") + "]" }

// ---- the behaviours I would be most embarrassed to break --------------

// A provider retry must not change any number.
func TestDuplicateDoesNotDoubleCount(t *testing.T) {
	h := newMux(NewStore())
	e := ev("e1", "c1", "u1", "delivered")
	post(t, h, arr(e))
	code, resp := post(t, h, arr(e, e)) // retry, and a repeat inside one batch
	if code != 200 || resp.Duplicate != 2 || resp.Accepted != 0 {
		t.Fatalf("code=%d resp=%+v", code, resp)
	}
	if s := stats(t, h, "c1"); s.Delivered != 1 {
		t.Fatalf("delivered = %d, want 1", s.Delivered)
	}
}

// One bad event must not affect the other events in the same batch.
func TestMixedBatchSurvivesBadItems(t *testing.T) {
	h := newMux(NewStore())
	body := arr(
		ev("good1", "c1", "u1", "sent"),
		`42`,                                     // not an object
		`{"event_id":123}`,                       // wrong field type
		ev("badtype", "c1", "u1", "spam_report"), // unknown type
		ev("upper", "c1", "u1", "OPENED"),        // wrong case: rejected, not normalised
		`{"event_id":"nots","campaign_id":"c1","contact_id":"u1","type":"sent","timestamp":"10-08-2026 09:00"}`,
		`{"event_id":"nocontact","campaign_id":"c1","type":"sent","timestamp":"2026-08-10T09:00:00Z"}`,
		ev("good2", "c1", "u2", "sent"),
		ev("good1", "c1", "u1", "sent"), // duplicate of the first
	)
	code, resp := post(t, h, body)
	if code != 200 {
		t.Fatalf("code = %d, want 200", code)
	}
	if resp.Accepted != 2 || resp.Rejected != 6 || resp.Duplicate != 1 || len(resp.Results) != 9 {
		t.Fatalf("unexpected summary: %+v", resp)
	}
	if s := stats(t, h, "c1"); s.Sent != 2 {
		t.Fatalf("sent = %d, want 2", s.Sent)
	}
	for _, r := range resp.Results {
		if r.Status == "rejected" && r.Reason == "" {
			t.Fatalf("rejected item %d has no reason", r.Index)
		}
	}
}

// Same event_id with different content: first write wins, second is not counted.
func TestConflictFirstWriteWins(t *testing.T) {
	h := newMux(NewStore())
	post(t, h, arr(ev("x", "c1", "u1", "sent")))
	_, resp := post(t, h, arr(ev("x", "c1", "u1", "clicked")))
	if resp.Conflict != 1 {
		t.Fatalf("resp = %+v", resp)
	}
	s := stats(t, h, "c1")
	if s.Sent != 1 || s.ClickedTotal != 0 {
		t.Fatalf("stats = %+v", s)
	}
}

// opened before delivered is normal and must be accepted.
func TestOutOfOrderAccepted(t *testing.T) {
	h := newMux(NewStore())
	_, resp := post(t, h, arr(ev("o1", "c1", "u1", "opened"), ev("d1", "c1", "u1", "delivered")))
	if resp.Accepted != 2 {
		t.Fatalf("resp = %+v", resp)
	}
	if s := stats(t, h, "c1"); s.OpenedTotal != 1 || s.Delivered != 1 {
		t.Fatalf("stats = %+v", s)
	}
}

// total vs unique, and unique is per campaign.
func TestUniqueVersusTotal(t *testing.T) {
	h := newMux(NewStore())
	post(t, h, arr(
		ev("a", "c1", "u1", "opened"), ev("b", "c1", "u1", "opened"), ev("c", "c1", "u1", "opened"),
		ev("d", "c1", "u2", "opened"),
		ev("e", "c2", "u1", "opened"), // same contact, other campaign
	))
	if s := stats(t, h, "c1"); s.OpenedTotal != 4 || s.OpenedUnique != 2 {
		t.Fatalf("c1 = %+v", s)
	}
	if s := stats(t, h, "c2"); s.OpenedTotal != 1 || s.OpenedUnique != 1 {
		t.Fatalf("c2 = %+v", s)
	}
}

func TestBodyMustBeJSONArray(t *testing.T) {
	h := newMux(NewStore())
	for _, body := range []string{``, `not json`, `{}`, `null`, `[] []`, `[{"a":1}] x`} {
		if code, _ := post(t, h, body); code != 400 {
			t.Errorf("body %q: code = %d, want 400", body, code)
		}
	}
	if code, _ := post(t, h, `[]`); code != 200 {
		t.Errorf("empty array: want 200")
	}
}

func TestUnknownCampaignIsZeros(t *testing.T) {
	h := newMux(NewStore())
	s := stats(t, h, "nope")
	if s.Sent != 0 || s.LastEventAt != nil {
		t.Fatalf("stats = %+v", s)
	}
}

// Many goroutines post the SAME events at once. Run with -race.
// Each of the 100 events must be counted exactly once.
//
// Note: this end-to-end test checks the exactly-once outcome, but I found by
// removing the lock on purpose that it does NOT reliably trip the race
// detector (likely because JSON decoding uses sync.Pool, which synchronises
// goroutines, and my machine has 1 CPU). TestStoreConcurrentAdd below calls
// the store directly and is the test that really guards the lock.
func TestConcurrentSameEventsCountedOnce(t *testing.T) {
	store := NewStore()
	h := newMux(store)
	items := make([]string, 100)
	for i := range items {
		items[i] = ev(fmt.Sprintf("e%03d", i), "c1", fmt.Sprintf("u%d", i%10), "opened")
	}
	body := arr(items...)

	const goroutines = 50
	start := make(chan struct{})
	accepted := make([]int, goroutines)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("POST", "/events", strings.NewReader(body)))
			var resp ingestResponse
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			accepted[g] = resp.Accepted
			store.Stats("c1") // concurrent reads too
		}(g)
	}
	close(start)
	wg.Wait()

	total := 0
	for _, a := range accepted {
		total += a
	}
	if total != 100 {
		t.Fatalf("accepted across all requests = %d, want exactly 100", total)
	}
	s := stats(t, h, "c1")
	if s.OpenedTotal != 100 || s.OpenedUnique != 10 {
		t.Fatalf("stats = %+v", s)
	}
}

// Large body should be refused, not crash.
func TestBodyTooLarge(t *testing.T) {
	h := newMux(NewStore())
	big := bytes.Repeat([]byte("a"), maxBodyBytes+10)
	body := `["` + string(big) + `"]`
	if code, _ := post(t, h, body); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d, want 413", code)
	}
}

// The real messy seed file. Expected numbers were computed with an
// independent Python script, not with this code.
func TestSeedFile(t *testing.T) {
	data, err := os.ReadFile("seed/events.json")
	if err != nil {
		t.Fatal(err)
	}
	h := newMux(NewStore())
	code, resp := post(t, h, string(data))
	if code != 200 {
		t.Fatalf("code = %d", code)
	}
	if resp.Accepted != 210 || resp.Rejected != 7 || resp.Duplicate != 15 || resp.Conflict != 3 {
		t.Fatalf("summary = acc %d rej %d dup %d conf %d", resp.Accepted, resp.Rejected, resp.Duplicate, resp.Conflict)
	}
	want := map[string][6]int{ // sent, delivered, openedTotal, openedUnique, clickedTotal, clickedUnique
		"cmp_summer_sale": {40, 35, 27, 19, 12, 12},
		"cmp_welcome":     {20, 19, 20, 12, 3, 3},
		"cmp_winback":     {12, 11, 7, 6, 4, 4},
	}
	for c, w := range want {
		s := stats(t, h, c)
		got := [6]int{s.Sent, s.Delivered, s.OpenedTotal, s.OpenedUnique, s.ClickedTotal, s.ClickedUnique}
		if got != w {
			t.Errorf("%s = %v, want %v", c, got, w)
		}
	}
	// Loading the same file again must change nothing.
	_, again := post(t, h, string(data))
	if again.Accepted != 0 {
		t.Fatalf("second load accepted %d events, want 0", again.Accepted)
	}
}

// Guards the lock in Store.Add: many goroutines add the same events at the
// same moment. With the mutex removed this test fails with DATA RACE under
// `go test -race` (checked by mutation).
func TestStoreConcurrentAdd(t *testing.T) {
	s := NewStore()
	evs := make([]Event, 100)
	for i := range evs {
		evs[i] = Event{
			EventID: fmt.Sprintf("e%03d", i), CampaignID: "c1", ContactID: fmt.Sprintf("u%d", i%10),
			Type: TypeOpened, Timestamp: time.Unix(1, 0).UTC(),
		}
	}
	start := make(chan struct{})
	accepted := make([]int, 50)
	var wg sync.WaitGroup
	for g := range accepted {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			for _, e := range evs {
				if s.Add(e) == Accepted {
					accepted[g]++
				}
			}
			s.Stats("c1")
		}(g)
	}
	close(start)
	wg.Wait()

	total := 0
	for _, a := range accepted {
		total += a
	}
	if total != 100 {
		t.Fatalf("accepted = %d, want exactly 100", total)
	}
	if st := s.Stats("c1"); st.OpenedTotal != 100 || st.OpenedUnique != 10 {
		t.Fatalf("stats = %+v", st)
	}
}
