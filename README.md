# Campaign Events Service

A Go HTTP service that ingests campaign lifecycle events from delivery providers and provides campaign statistics.

## Requirements

- Go 1.22 or later
- Go standard library only
- In-memory storage (data is lost when the process stops)

## Project Structure

The repository should follow this structure:

```text
TWW-Engineering-Assessment/
├── main.go
├── handlers.go
├── store.go
├── types.go
├── validate.go
├── service_test.go
├── go.mod
├── seed/
│   └── events.json
├── debugging/
│   ├── go.mod
│   ├── main.go
│   ├── README.md
│   ├── events.jsonl
│   └── expected_output.txt
├── README.md
├── NOTES.md
├── BUGS.md
└── AI_USAGE.md
```

Include `seed/events.json` and all four debugging support files in the repository. Do not submit only `debugging/main.go`.

## Run the Service

From the repository root:

```bash
go run .
```

The server listens on `:8080` by default.

To use a different address:

**PowerShell**

```powershell
$env:ADDR = ":9000"
go run .
```

**Linux/macOS**

```bash
ADDR=:9000 go run .
```

## Try the API

### 1. Load the supplied provider traffic

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  --data-binary @seed/events.json
```

### 2. Submit a single event

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '[{"event_id":"evt_1","campaign_id":"cmp_summer_sale","contact_id":"ct_001","type":"delivered","timestamp":"2026-08-10T06:15:00Z"}]'
```

### 3. Read campaign statistics

```bash
curl http://localhost:8080/campaigns/cmp_summer_sale/stats
```

### Expected Seed Results

The supplied seed file should produce the following results:

| Result | Count |
|---|---:|
| Accepted | 210 |
| Duplicate | 15 |
| Conflict | 3 |
| Rejected | 7 |

Loading the same seed file again should accept zero new events because previously accepted event IDs are already stored.

## API

### `POST /events`

Accepts a JSON array of campaign events.

The request body is limited to **5 MB**.

Each event is decoded and validated independently. If the top-level request body is a valid JSON array, the service returns HTTP `200` with per-item results and aggregate totals.

#### Event fields

| Field | Required | Description |
|---|---|---|
| `event_id` | Yes | Unique event identifier and idempotency key |
| `campaign_id` | Yes | Campaign identifier |
| `contact_id` | Yes | Contact associated with the event |
| `type` | Yes | Event type |
| `timestamp` | Yes | Provider event timestamp in RFC3339 format |
| `metadata` | No | Optional additional event information |

Supported event types are exactly:

- `sent`
- `delivered`
- `opened`
- `clicked`

#### Item processing statuses

| Status | Meaning | Changes statistics? |
|---|---|---|
| `accepted` | First valid occurrence of an event ID | Yes |
| `duplicate` | Same event ID and same event content | No |
| `conflict` | Same event ID but different event content; first write wins | No |
| `rejected` | Invalid event; the response includes the reason | No |

For duplicate comparison, the service compares:

- Campaign ID
- Contact ID
- Event type
- Timestamp instant

Metadata is intentionally ignored when determining whether an event is a duplicate.

#### HTTP responses

| Status code | Meaning |
|---|---|
| `200 OK` | Valid top-level JSON array processed, including batches with rejected items |
| `400 Bad Request` | Malformed JSON or a body that is not a JSON array |
| `413 Request Entity Too Large` | Request body exceeds 5 MB |

### `GET /campaigns/{campaign_id}/stats`

Returns all-time statistics for the requested campaign.

The response includes:

| Field | Description |
|---|---|
| `sent` | Total accepted sent events |
| `delivered` | Total accepted delivered events |
| `opened_total` | Total accepted opened events |
| `clicked_total` | Total accepted clicked events |
| `opened_unique` | Number of distinct contacts with an accepted opened event in the campaign |
| `clicked_unique` | Number of distinct contacts with an accepted clicked event in the campaign |
| `last_event_at` | Latest provider event timestamp |
| `last_ingested_at` | Time the service most recently accepted an event |

An unknown campaign returns zero counters and `null` timestamps.

## Tests

Run the service tests from the repository root:

```bash
go test -race ./...
```

Run the debugging exercise separately:

```bash
cd debugging
go run -race . events.jsonl
go run . events.jsonl
```

On PowerShell, compare the debugging output against the expected output:

```powershell
go run . events.jsonl | Set-Content actual_output.txt
Compare-Object (Get-Content actual_output.txt) (Get-Content expected_output.txt)
```

No output from `Compare-Object` means the two files match.

## Design Choices and Limitations

- **Idempotency:** `event_id` is used as the idempotency key.
- **Conflict handling:** Conflicting reuse of an event ID follows a first-write-wins policy.
- **Batch processing:** Valid events in a mixed batch are processed even when other events are rejected.
- **Out-of-order events:** Events are accepted even when they arrive out of chronological order. The service does not invent missing lifecycle events.
- **Concurrency safety:** A `sync.RWMutex` protects the event map and campaign counters.
- **Efficient statistics:** Statistics are maintained during ingestion, so a stats request does not need to scan all stored events.
- **Storage:** Data is stored in memory. All events and counters are lost when the process stops. This is a deliberate time-boxed trade-off, not durable production storage.

## Out of Scope

The following features are not implemented:

- Optional paginated campaign-events endpoint
- Date-range filters
- Frontend
- Authentication
- Deployment
- Persistent database storage

See `NOTES.md` for assumptions, design decisions, scalability considerations, and product questions.

See `BUGS.md` for the debugging exercise and fixes.

See `AI_USAGE.md` for disclosure of AI assistance.