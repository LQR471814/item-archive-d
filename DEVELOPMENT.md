# For Developers

## Architecture

- **Backend**: Go `net/http` server.
- **Database**: SQLite with `sqlc` (`internal/db`).
- **Storage**: CAS filesystem storage (`internal/blob`).

## Files

- **Server**: `main.go` initializes dependencies and routes.
- **AI Tagger**: `internal/ai-tagger` auto-tags images via GenAI.

## Testing

Run all tests:

```bash
go test ./...
```

The data layer is tested with an oracle-based fuzzing system found in [query_test.go](./internal/db/query_test.go).
