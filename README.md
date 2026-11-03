# TWW Engineering Assessment – Campaign Events Service

## Project Overview

This project implements a REST API service in Go for ingesting campaign events, validating event data, identifying duplicates and conflicts, and generating campaign statistics.

The service uses in-memory storage and supports concurrent event processing.

## Technology Stack

- **Language:** Go 1.22+
- **HTTP Server:** Go standard library
- **Storage:** In-memory
- **Testing:** Go testing package and race detector

## Project Structure

```text
TWW Main Task/
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

- Go 1.22 or later

Verify Go installation:

```bash
go version
```

### Run the application

From the project root directory:

```bash
go run .
```

The server runs on port `8080`.

## API Endpoints

### 1. Ingest Events

**Endpoint:** `POST /events`

Accepts a JSON array of campaign events.

Example using PowerShell:

```powershell
Invoke-RestMethod `
    -Uri "http://localhost:8080/events" `
    -Method Post `
    -ContentType "application/json" `
    -InFile ".\seed\events.json"
```

Each event is processed independently and receives a result indicating whether it was accepted, identified as a duplicate, rejected due to a conflict, or rejected due to validation errors.

### 2. Get Campaign Statistics

**Endpoint:** `GET /campaigns/{campaign_id}/stats`

Example:

```text
http://localhost:8080/campaigns/cmp_welcome/stats
```

The response provides campaign statistics, including:

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

Submit the seed file using the `POST /events` endpoint.

Expected first-upload results:

| Result | Expected Count |
|---|---:|
| Accepted | 210 |
| Duplicate | 15 |
| Conflict | 3 |
| Rejected | 7 |
| **Total** | **235** |

Submitting the same seed file again should not create additional accepted events.

## Testing

Run all tests:

```bash
go test ./...
```

Run tests with the race detector:

```bash
go test -race -count=1 ./...
```

Check the code:

```bash
go vet ./...
```

The test suite covers:

- Duplicate event handling
- Mixed batches containing invalid events
- Conflicting event IDs
- Out-of-order event timestamps
- Total and unique event statistics
- Invalid request formats
- Unknown campaign statistics
- Request body size limits
- Seed-file ingestion
- Concurrent event processing

## Debugging Exercise

The debugging exercise is maintained separately in the `debugging/` directory.

Run it from that directory:

```bash
cd debugging
go run -race . events.jsonl
```

Compare its output with `expected_output.txt`.

The debugging exercise includes fixes for:

1. Duplicate tracking across batches.
2. Unique-open tracking across campaigns.
3. UTC-based daily grouping.
4. Thread-safe counter updates.

## Design Considerations

- Standard-library-only implementation.
- In-memory event storage.
- Validation before event processing.
- Duplicate detection based on campaign, contact, event type, and timestamp instant.
- Event metadata is ignored during duplicate comparison.
- Thread-safe shared-state access.
- Idempotent handling of repeated event submissions.

## Additional Documentation

- **NOTES.md:** Assumptions, design decisions, PM questions, and scalability considerations.
- **BUGS.md:** Debugging findings, identified bugs, and fixes.
- **AI_USAGE.md:** Disclosure of AI assistance used during development.

## Future Improvements

If given more time, I would focus on the following improvements to make the service more scalable, reliable, and production-ready.

### 1. Persistent Database
Replace the current in-memory storage with PostgreSQL or another database to ensure that event data persists even after application restarts.

### 2. Scalability
Introduce message queues and asynchronous batch processing to handle high-volume event ingestion, potentially scaling to 100 million events per day.

### 3. Performance Optimization
Implement database indexing, optimized queries, and caching to improve event ingestion and campaign statistics retrieval.

### 4. Monitoring and Logging
Add structured logging, application metrics, health checks, and monitoring to identify errors and performance bottlenecks.

### 5. API Improvements
Enhance the API with pagination, consistent error responses, authentication, and API documentation.

### 6. Additional Testing
Expand test coverage with integration tests, load testing, benchmarks, and additional edge cases.

### 7. Deployment and Automation
Containerize the application using Docker and introduce a CI/CD pipeline to automate testing, building, and deployment.

### Implementation Priority

My initial priorities would be:

1. Persistent database integration.
2. Scalable event ingestion and processing.
3. Monitoring and performance optimization.

These improvements would help evolve the current implementation into a reliable production-ready campaign events service.
