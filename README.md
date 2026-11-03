# TWW Engineering Assessment – Campaign Events Service

A Go-based HTTP service for ingesting campaign events, validating event data, detecting duplicates and conflicts, and calculating campaign statistics.

## Features

- Ingest campaign events through a REST API.
- Validate incoming event data.
- Detect duplicate events and conflicting event IDs.
- Process events in batches with per-event results.
- Calculate campaign statistics, including total and unique opens and clicks.
- Track the latest event and ingestion timestamps.
- Store events in memory.
- Include automated tests and a separate debugging exercise.

## Technology

- Go 1.22+
- Go standard library
- In-memory storage
- Go testing and race detector

## Project Structure

```text
.
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

## Getting Started

### Prerequisites

Install Go 1.22 or later.

Check your installation:

```bash
go version
```

### Run the service

From the project root:

```bash
go run .
```

The service runs on port `8080`.

## API Endpoints

### 1. Ingest campaign events

**POST** `/events`

Accepts a JSON array of campaign events.

Example using PowerShell:

```powershell
Invoke-RestMethod `
    -Uri "http://localhost:8080/events" `
    -Method Post `
    -ContentType "application/json" `
    -InFile ".\seed\events.json"
```

The response provides a result for each submitted event, identifying accepted events, duplicates, conflicts, and rejected events.

### 2. Get campaign statistics

**GET** `/campaigns/{campaign_id}/stats`

Example:

```text
http://localhost:8080/campaigns/cmp_welcome/stats
```

The statistics include:

- Sent events
- Delivered events
- Total opens
- Unique opens
- Total clicks
- Unique clicks
- Last event timestamp
- Last ingestion timestamp

Unknown campaign IDs return zero counts and null timestamps.

## Seed Data Verification

Submit the seed file:

```powershell
Invoke-RestMethod `
    -Uri "http://localhost:8080/events" `
    -Method Post `
    -ContentType "application/json" `
    -InFile ".\seed\events.json"
```

Expected first-upload results:

| Status | Count |
|---|---:|
| Accepted | 210 |
| Duplicate | 15 |
| Conflict | 3 |
| Rejected | 7 |
| **Total** | **235** |

Submitting the same seed data again should not create additional accepted events.

## Running Tests

Run the test suite:

```bash
go test ./...
```

Run tests with the race detector:

```bash
go test -race ./...
```

Check code using:

```bash
go vet ./...
```

## Debugging Exercise

The separate debugging exercise is located in the `debugging/` directory.

Run it from that directory:

```bash
cd debugging
go run -race . events.jsonl
```

Compare the generated output with `expected_output.txt`.

## Design Considerations

- Standard-library-only implementation.
- In-memory event storage.
- Event validation before processing.
- Duplicate detection based on campaign, contact, event type, and timestamp instant.
- Event metadata does not affect duplicate identification.
- Thread-safe access to shared state.
- Idempotent ingestion for repeated event submissions.

## Additional Documentation

- `NOTES.md` – Assumptions, design decisions, and scalability considerations.
- `BUGS.md` – Debugging exercise findings and fixes.
- `AI_USAGE.md` – Disclosure of AI assistance used during development.

