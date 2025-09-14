Testing strategy and commands

Categories

- Functional tests: default unit and handler tests that run with `go test ./...`.
- Performance tests: benchmarks using `testing.B` with `go test -bench=. ./...`.
- Integration tests: end-to-end-ish tests exercising real HTTP stacks via local mocks, guarded by the `integration` build tag.
- Database tests: live DB tests for Valkey/Redis, guarded by the `db` build tag and environment variables.

How to run

- All functional tests: `go test ./...`
- Benchmarks (performance): `go test -bench=. -run ^$ ./...`
- Integration tests: `go test -tags=integration ./...`
- Database tests (Valkey single): set env and run
  - `VALKEY_ADDR=127.0.0.1:6379 go test -tags=db ./pkg/cache`
- Database tests (Valkey cluster): set env and run
  - `VALKEY_NODES=127.0.0.1:7000,127.0.0.1:7001 go test -tags=db ./pkg/cache`

Notes

- Generated protobuf code and command binaries are not targeted for high unit coverage.
- External services (VictoriaMetrics, gRPC engines) are exercised via mocked HTTP servers and optional db-tagged tests.

