# AI Usage Disclosure

## AI Tools Used

ChatGPT was used as an AI assistance tool during the development of this assessment.

## Areas Where AI Assistance Was Used

AI assistance was used for the following activities:

1. **Requirement Understanding**
   - Understanding the assessment requirements.
   - Clarifying API behavior, validation rules, duplicate detection, and conflict handling.
   - Reviewing assumptions and identifying questions that could be discussed with the product manager.

2. **Implementation Assistance**
   - Discussing Go implementation approaches.
   - Understanding HTTP handlers, JSON processing, event storage, and campaign statistics.
   - Reviewing concurrency-related implementation considerations.

3. **Debugging Assistance**
   - Understanding the debugging exercise and investigating the identified issues.
   - Discussing duplicate tracking, unique-open calculations, UTC timestamp grouping, and concurrent counter updates.

4. **Testing Assistance**
   - Identifying relevant test scenarios.
   - Understanding how to run Go tests and use the race detector.
   - Reviewing test output and expected seed-file results.

5. **Documentation Assistance**
   - Improving the structure and clarity of README.md, NOTES.md, BUGS.md, and AI_USAGE.md.

## Validation and Verification

The implementation was reviewed and tested locally.

The following verification commands were executed:

```bash
go test -v -race -count=1 ./...
go vet ./...
```

The test suite completed successfully with no reported failures or race conditions.

The debugging exercise was also executed with Go's race detector, and its actual output was compared with the expected output.

The seed-file ingestion was verified against the expected results:

- Accepted: 210
- Duplicate: 15
- Conflict: 3
- Rejected: 7

## Responsibility

AI assistance was used to support understanding, implementation, debugging, testing, and documentation.

The project was run and verified in the local development environment. This disclosure is intended to transparently describe the use of AI assistance during the assessment.
