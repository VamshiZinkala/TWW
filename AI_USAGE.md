# AI_USAGE.md

## 1. Tools I Used

- ChatGPT

*Add Claude only if I personally used it during this assessment.*

## 2. How I Used AI

I used ChatGPT to understand the assignment and identify what I needed to complete.

I also used it to:
- Understand the Go backend structure and how the API should work.
- Discuss event validation, duplicate event handling, and campaign statistics.
- Understand concurrency issues and how to fix them.
- Review the four debugging problems and possible solutions.
- Improve my documentation, including `NOTES.md` and `BUGS.md`.
- Check my testing approach and final submission requirements.

I did not blindly accept the suggestions. I reviewed the code, ran the program, compared the output with the expected results, and checked concurrency using Go's race detector.

I am responsible for the final implementation and its behaviour.

## 3. One AI Suggestion I Rejected or Changed

I decided to use a **first-write-wins** approach for duplicate event IDs.

If an event ID has already been processed and another event arrives with the same ID but different data, I keep the original event instead of replacing it.

I chose this because a delayed retry or conflicting provider payload should not silently change statistics that were already recorded.

## 4. One Thing AI Helped Me Understand

I understood why duplicate checking and updating statistics must be handled safely when multiple requests arrive at the same time.

For example, two requests might arrive with the same event ID. If both check the ID before either inserts it, both could think it is a new event and count it twice.

Using a mutex to protect the relevant operations ensures that only one goroutine can perform the protected update at a time within a single service process.

## 5. Anything AI Generated That I Had to Debug or Correct

While checking the debugging program, I noticed that some output format strings contained incorrectly escaped newline characters. Instead of printing each campaign on a separate line, the program printed literal `\n` characters.

I corrected the newline escaping in the output formatting and ran the program again.

I then compared `actual_output.txt` with `expected_output.txt` using PowerShell's `Compare-Object`. The comparison returned no differences.

I also ran the debugging program with Go's race detector:

```powershell
go run -race .\debugging\main.go .\debugging\events.jsonl
```

The program completed without reporting a data race.