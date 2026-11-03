# NOTES.md

## Part 1: Understanding the Problem

### 1. My understanding of the assignment

Relay sends marketing messages through third-party providers. These providers send events to our backend whenever something happens to a message.

The supported events are:

- `sent`
- `delivered`
- `opened`
- `clicked`

My backend has two main responsibilities:

1. Accept and process events from providers.
2. Provide campaign statistics through an API.

The main challenge is handling unreliable event data. Providers may send the same event multiple times, events may arrive late or in the wrong order, and some requests may contain invalid data.

The system should handle these situations without counting events incorrectly or failing an entire batch because of one bad event.

I treat `event_id` as the unique identifier of an event. The event timestamp represents when the event happened at the provider, while arrival time represents when my backend received it. For reporting, I use the event timestamp.

### 2. What I found in the seed file

I inspected `seed/events.json` before deciding how to process the data.

The file contains 235 rows.

My observations:

- **Duplicate event IDs:** 16 event IDs appear more than once.
- **Exact retries:** 13 IDs have identical repeated event data.
- **Conflicts:** 3 IDs appear with different event types: `evt_00029`, `evt_00113`, and `evt_00164`.
- **Invalid events:** 7 rows contain problems such as missing fields, empty IDs, unsupported event types, or invalid timestamps.
- **Unordered events:** Events are not arranged by timestamp. For example, an open event may arrive before a delivered event.
- **Campaigns:** The data contains three campaigns: `cmp_summer_sale`, `cmp_welcome`, and `cmp_winback`.

There are 18 repeated rows in total: 15 identical retries and 3 conflicting events.

Therefore, my expected accepted event count is:

`235 - 7 invalid - 18 repeated = 210 accepted events`

### 3. Decisions and assumptions

#### Event identification and duplicate handling

I use `event_id` as the idempotency key.

If the same event is received again, it should not increase the campaign statistics.

One assumption is that event IDs are unique across providers. If multiple providers can generate the same ID, I would need to include a provider identifier in the deduplication key.

#### Handling conflicting event IDs

If an event ID already exists but the new event contains different data, I follow a **first-write-wins** approach.

I keep the original event, do not count the conflicting event, and report the conflict.

I chose this because I cannot know which version is correct. Replacing an accepted event with a later retry could unexpectedly change campaign statistics.

#### Handling invalid events

I validate each event separately.

The required fields are:

- `event_id`
- `campaign_id`
- `contact_id`
- `type`
- `timestamp`

All required fields must be present and non-empty.

The event type must exactly match one of the four supported values. For example, `OPENED` is rejected rather than automatically converted to `opened`.

The timestamp must follow RFC3339 format and is normalised to UTC.

If one event is invalid, I reject that event but continue processing the other events in the batch.

#### API response handling

If the request body is not a valid JSON array or cannot be read, the API returns HTTP 400 without processing events.

If the array is valid, the API returns HTTP 200 with individual results and a summary containing:

- Accepted events
- Duplicate events
- Conflicting events
- Rejected events

I use this approach because returning an error for an entire batch due to one invalid event could cause providers to retry all events, including those already processed successfully.

#### Understanding opened and clicked statistics

I maintain both total and unique counts.

- `opened_total`: Total number of accepted open events.
- `opened_unique`: Number of distinct contacts who opened a campaign.
- The same distinction applies to clicked events.

This helps because one contact may open or click multiple times. Marketers may want to know both the total activity and the number of people involved.

Sent and delivered are counted as distinct accepted events.

#### Handling events arriving out of order

I accept an event even if its previous lifecycle event has not arrived.

For example, an opened event can be accepted even if the corresponding delivered event is missing.

I do not create missing events or reject events based on their order because providers may send events late.

#### Storage

For this assessment, I use in-memory storage protected by `sync.RWMutex`.

I maintain:

- A map of event IDs to events for duplicate checking.
- Campaign counters that are updated when events are accepted.
- Sets of contacts for unique open and click counts.

Updating counters during ingestion makes campaign statistics quick to retrieve without scanning every event.

The limitation is that all data is lost when the application restarts. For a production system, I would use SQLite or PostgreSQL with a unique constraint on `event_id`.

#### Concurrency

Duplicate checking, event insertion, and counter updates must happen as one atomic operation.

Otherwise, two concurrent requests containing the same event might both check the map before either inserts the event. Both could then count it.

I use a write lock to protect ingestion and a read lock for statistics retrieval.

I also use Go's race detector to check for concurrent access problems.

#### Timestamp handling

I normalise timestamps to UTC.

For this implementation, campaign statistics are all-time statistics rather than date-range statistics.

### 4. Ambiguities in the assignment

These are the questions I would clarify before building a production version:

1. Are event IDs unique across all providers?
2. What should happen if the same event ID arrives with different data?
3. Should opened and clicked show total events, unique contacts, or both?
4. Should statistics be lifetime totals or based on a date range?
5. Should a batch be processed entirely or should valid events succeed independently?
6. Can providers correct or retract previously submitted events?
7. Can opened or clicked events arrive without a delivered event?
8. How long should raw events be stored?
9. Should the data survive application restarts?
10. What is the maximum request or batch size?
11. How quickly should campaign statistics be updated?
12. Does sent represent messages handed to the provider or sent events received from the provider?

### 5. Questions I would ask the PM

1. Can we get a provider ID to make event IDs unique across providers?
2. Do marketers need total opens, unique opens, or both?
3. What is the expected behaviour when an event ID has conflicting data?
4. Do providers support corrections or retractions?
5. Are date filters and campaign-specific time zones required?
6. What are the expected traffic volume and maximum batch size?
7. Is in-memory storage acceptable, or is durable storage required?

### 6. My implementation priorities

I prioritised the work in this order:

1. Understand the starter code and debugging requirements.
2. Inspect the seed data and document assumptions.
3. Fix the four known debugging issues and verify the results.
4. Implement `POST /events` with validation, duplicate detection, and conflict handling.
5. Implement `GET /campaigns/{id}/stats`.
6. Test duplicates, invalid events, conflicts, out-of-order events, and concurrent requests.
7. Compare the seed file results with the expected counts.
8. Complete the scale analysis, marketer explanation, README, and AI usage documentation.

I treated the optional paginated events endpoint, frontend, Docker, and deployment as lower priorities because they are not required for the core task.

---

## Part 4: Scaling to 100 Million Events per Day

### 1. Understanding the scale

100 million events per day means approximately:

`100,000,000 / 86,400 = 1,157 events per second`

This is only the average. Actual traffic can have peaks several times higher.

My current in-memory implementation is designed for a much smaller workload, so it would need significant changes.

### 2. What would break first?

**1. Memory usage**

I store events and contact information in memory. At 100 million events per day, memory usage would grow very quickly.

The application could run out of memory, and all stored data would be lost after a restart.

**2. Global write lock**

Every ingestion operation uses a shared write lock.

At high traffic volumes, requests would wait for one another, increasing latency and reducing throughput.

**3. Lack of durable storage**

The current design stores data inside one process. It cannot safely support multiple application instances because each instance would have separate deduplication state.

### 3. How I would improve the system

I would make the following changes:

1. **Load testing:** First, test with realistic traffic, including duplicates and late events, to identify the actual bottlenecks.
2. **Durable database:** Move event storage to PostgreSQL or another durable database. Use a unique constraint on `event_id` to prevent duplicate insertion.
3. **Reduce lock contention:** Use database transactions, batching, and partitioning or sharding where necessary.
4. **Pre-aggregated counters:** Maintain campaign statistics during event insertion so the dashboard does not need to scan all raw events.
5. **Data retention:** Archive or remove old raw events according to retention requirements while preserving the required aggregates.

### 4. Would the API contract change?

The main API endpoints and event payload can remain mostly the same.

However, I would consider adding a provider identifier to avoid event ID collisions.

I would also clearly define whether a successful response means that an event was received or that it has already been reflected in campaign statistics.

The storage implementation would change from in-memory maps to durable, indexed storage.

### 5. Would I introduce a queue?

Yes. At this scale, I would consider using a message queue.

The webhook would validate the request, store or enqueue the event durably, and return a successful response quickly.

A separate consumer would process events and update the database and campaign counters.

This reduces the amount of work performed directly by the webhook.

However, it introduces new challenges:

- Statistics may become eventually consistent.
- Consumers may fall behind.
- Invalid messages need a dead-letter mechanism.
- Event ordering needs attention.
- The queue adds another system to monitor and maintain.

### 6. Where can duplicates occur?

With a queue-based architecture, duplicates can happen in several places:

- A queue may deliver the same message more than once.
- A consumer may crash after writing to the database but before acknowledging the message.
- Two consumers may process the same event concurrently.
- A provider may retry after a webhook timeout.
- Events may be replayed during recovery or backfills.

Therefore, deduplication must also happen at the database level.

The counter should be updated only when a new event is successfully inserted.

### 7. How would I keep statistics accurate?

- Update counters only when a new event is successfully inserted.
- Use a database transaction to keep event insertion and counter updates consistent.
- Periodically compare stored aggregates with counts calculated from raw events.
- Keep the ability to rebuild aggregates.
- Clearly communicate any delay between event acceptance and dashboard updates.

### 8. What would I monitor?

I would monitor:

- Events received per second.
- Request latency and error rates.
- Accepted, duplicate, conflicting, and rejected event counts.
- Queue size and consumer processing delays.
- Database write latency and storage growth.
- Dashboard response time.
- Differences between raw event counts and stored aggregates.

### 9. What would I not solve immediately?

I would not immediately introduce multi-region processing, complex stream processing, or database sharding across many nodes.

I would first build reliable durable storage, deduplication, monitoring, and load testing.

I would use actual performance measurements to decide which additional improvements are necessary.

---

## Part 5: Explaining the Marketer's Complaint

### 1. Reported statistics

| Metric | 10:00 | 10:30 | Change |
|---|---:|---:|---:|
| Sent | 1,000,000 | 1,000,000 | 0 |
| Delivered | 970,000 | 975,000 | +5,000 |
| Opened | 250,000 | 248,000 | -2,000 |
| Clicked | 20,000 | 20,000 | 0 |

The marketer is concerned because delivered increased while opened decreased.

I would not immediately assume that the backend is broken. I would first check how the statistics are defined and how events are processed.

### 2. Possible explanations

#### Reasons that may be expected behaviour

1. **Late delivered events:** Providers may send delivered events after a delay. This explains why delivered can increase while sent remains unchanged.

2. **Unique counts:** If opened represents unique contacts, its behaviour may differ from total open events.

3. **Rolling time window:** If the dashboard displays the last few hours or today's statistics, older opens may disappear from the reporting window.

4. **Provider corrections:** Some providers may remove or reclassify bot-generated opens.

5. **Contact merging:** If multiple contacts are merged, unique open counts may decrease.

6. **Different filters:** The two snapshots may use different date ranges, time zones, campaign filters, or metric definitions.

7. **Data retention:** Old events may have been archived or removed, followed by recalculation of statistics.

8. **Cache differences:** The two snapshots may come from caches or database replicas with different update times.

#### Reasons that may indicate a bug

9. **Counter race condition:** Concurrent updates may cause lost increments.

10. **Deduplication issue:** A real event may be incorrectly treated as a duplicate, or conflicting data may overwrite an accepted event.

11. **Processing failures:** Open events may be rejected or lost because of validation or consumer errors.

12. **Incorrect aggregation:** A backfill or rebuild may calculate statistics from incomplete data.

13. **Timezone issue:** Incorrect timezone conversion may move events between reporting dates.

### 3. Is the increase in delivered suspicious?

Not by itself.

Delivered increased by 5,000, which is only 0.5% compared with the original total. Delivered is still below sent.

Late-arriving delivered events are normal in event-based systems.

I would investigate further only if delivered exceeds sent or if the increase cannot be explained by newly received events.

### 4. What I would investigate first

1. **Check metric definitions:** Confirm whether opened represents total events or unique contacts, and whether both snapshots use the same filters and time range.

2. **Check raw events:** Recalculate the statistics for both 10:00 and 10:30 using the events available at those times.

3. **Check provider corrections:** Look for retractions, event reclassification, or provider-side changes.

4. **Check the processing pipeline:** Review rejected events, duplicate rates, conflicts, consumer errors, and processing delays.

5. **Compare aggregates with raw data:** If the calculated statistics differ from stored counters, investigate the aggregation logic.

### 5. How I would decide whether it is a bug

If opened is a lifetime total of accepted events and events cannot be removed, the count should not decrease.

In that situation, a decrease indicates a bug or an undocumented correction.

However, if opened is a unique-contact metric, a rolling-window metric, or supports provider corrections, a decrease may be valid.

The important thing is to identify exactly which events caused the change and make the metric definition clear in the dashboard.

---

## What I Completed and What I Intentionally Skipped

Update this checklist based on the work actually completed before submission.

- [x] Part 1: Problem understanding, assumptions, seed-file analysis, and PM questions.
- [x] Part 3: Four debugging fixes and `BUGS.md`.
- [x] Part 3: Compared actual output with expected output; no differences.
- [x] Part 3: Ran the debugging program with Go's race detector without a reported race.
- [x] Part 4: Scaling analysis for 100 million events per day.
- [x] Part 5: Marketer statistics investigation.
- [ ] Part 2: `POST /events` implementation and validation.
- [ ] Part 2: `GET /campaigns/{id}/stats` implementation.
- [ ] Tests for duplicates, invalid events, conflicts, out-of-order events, and concurrent requests.
- [x] Seed-file ingestion and verification of the expected accepted event count.
- [ ] README with exact run instructions.
- [ ] Final review of `AI_USAGE.md`.

**Intentionally skipped or deferred:**

- Frontend development.
- Docker and deployment.
- Authentication.
- Optional paginated events endpoint.
- Durable storage using SQLite or PostgreSQL.

These were not prioritised over the required functionality within the assessment time.

---

## If I Had Another Day

If I had more time, I would:

1. Add SQLite or PostgreSQL so data survives application restarts.
2. Add date-range statistics and campaign-specific time zones.
3. Implement the optional paginated events endpoint.
4. Clarify provider corrections and conflicting event handling with the PM.
5. Perform load testing to identify actual performance limits.
6. Add a reconciliation job to compare campaign counters with raw event data.
7. Add monitoring for accepted, duplicate, conflicting, and rejected events.