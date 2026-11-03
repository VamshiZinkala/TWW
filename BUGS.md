# BUGS.md

## Part 3: Debugging

Four bugs were identified and fixed in `debugging/main.go`. All fixes were small and local; I did not restructure the program.

### How I found the bugs

I first ran the program and compared its output with `expected_output.txt`. The results did not match. Then I ran the race detector, which reported a data race in `apply()`. Finally, I inspected the code against the four requirements described in `debugging/README.md`.

---

## Bug 1: Duplicate events were removed only within a batch

**Problem:**

The `sent` count for `cmp_A` was 2025, while the expected value was 1987. Duplicate events were being counted.

**Root cause:**

The `processBatch` function declared `seen := make(map[string]bool)` locally. Since the function processes batches of 200 lines, a new map was created for every batch.

Consequently, an `event_id` was recognised as a duplicate only when its retry appeared in the same batch. Duplicates appearing in different batches were counted again.

Requirement 1 specifies that each event must be processed at most once across the entire file.

**Fix:**

Moved `seen` to package-level scope, next to `stats` and `openedBy`, and removed the local declaration from `processBatch`.

The map is accessed only from the main goroutine, so no additional lock is required.

**Verification:**

After the fix, the `sent`, `delivered`, `opened`, and `clicked` counts matched the expected output.

---

## Bug 2: Unique opens were tracked per contact instead of per campaign and contact

**Problem:**

The `unique_opens` count for `cmp_A` was 549, while the expected value was 942.

**Root cause:**

The `openedBy` map used only `ev.ContactID` as its key.

If a contact opened an event in campaign A, the same contact was treated as already counted when opening an event in campaign B. This caused unique opens to be counted globally rather than separately for each campaign.

Requirement 2 specifies that the same contact opening two different campaigns must count once in each campaign.

**Fix:**

Changed the map key to include both campaign ID and contact ID:

```go
key := ev.CampaignID + "|" + ev.ContactID
```

The separator ensures that different campaign/contact combinations are distinguishable.

**Verification:**

After the fix, `unique_opens` matched the expected values for all five campaigns.

---

## Bug 3: Daily statistics used the machine's local time zone

**Problem:**

Daily delivered counts could differ depending on the machine's configured time zone.

**Root cause:**

The `track()` function used:

```go
ev.Timestamp.Local().Format("2006-01-02")
```

The `Local()` method converts the timestamp to the machine's local time zone before extracting the date.

For example, an event at 22:00 UTC can fall on the next calendar day in India. Therefore, daily buckets could differ across machines.

Requirement 3 specifies that daily statistics must use the event's UTC date. This also supports reproducible output.

**Fix:**

Replaced:

```go
ev.Timestamp.Local().Format("2006-01-02")
```

with:

```go
ev.Timestamp.UTC().Format("2006-01-02")
```

**Verification:**

The daily delivered counts matched the expected output after the fix.

---

## Bug 4: Concurrent updates caused a data race

**Problem:**

Multiple worker goroutines updated campaign counters concurrently. This could cause lost increments and inconsistent statistics.

**Root cause:**

The `processBatch` function starts eight worker goroutines that call `apply()`.

Each worker updates shared fields such as:

```go
cs.Sent++
cs.Delivered++
cs.Opened++
cs.Clicked++
```

These operations are not atomic. Multiple goroutines can read and update the same counter simultaneously.

Running the program with the race detector identified a data race in `apply()`.

**Fix:**

Introduced a package-level `sync.Mutex` and protected the counter updates in `apply()`:

```go
mu.Lock()
defer mu.Unlock()
```

This ensures that only one goroutine updates the shared campaign statistics at a time.

A mutex was chosen because it is a small change and the critical section is short.

**Verification:**

Ran:

```powershell
go run -race .\debugging\main.go .\debugging\events.jsonl
```

The program completed successfully without reporting a data race.

---

## Final verification

The following checks were performed:

1. Ran the debugging program against the provided JSONL dataset.
2. Compared `actual_output.txt` with `expected_output.txt` using PowerShell `Compare-Object`.
3. Confirmed that the comparison returned no differences.
4. Ran the debugging program with the Go race detector:

```powershell
go run -race .\debugging\main.go .\debugging\events.jsonl
```

The program completed and printed the expected campaign statistics without reporting a data race.

---

## What I did not change

- Kept the changes limited to the four identified bugs.
- Did not restructure the existing program.
- Did not add unnecessary locking to `seen`, `openedBy`, or `DailyDelivered`, because they are accessed by the main goroutine before the worker goroutines start.
- Left unrelated formatting and code unchanged to keep the diff minimal.