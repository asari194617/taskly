# Public iCal feed

`GET /ical/{token}.ics` returns all tasks created by the token owner, including
past and future deadlines. There is no date window or task-count limit. Events
are ordered by deadline and task ID. A task without a deadline produces no event.

The request reads the token, the owner's notification language, and the tasks
with their subjects in three database queries, regardless of task count. The
language selects the English subject name when available; otherwise the
Japanese name is used. The response is generated on every request so task and
language changes are visible on the next fetch. No short-term cache or
conditional response is used because there is no measured subscription traffic
that justifies the added invalidation behavior.

An unknown, expired, or deleted user's token returns 404. A database failure
returns 500 and does not serve a partial calendar as a successful response.

## Verification (2026-09-22)

SQLite route tests with 127 and 501 tasks verified all events, the final event,
the translated subject name, and three queries per request. A dropped task table
verified that feed generation returns 500. The local benchmark below measures
generation only (language and task queries plus ICS construction) on Apple M2
Pro with SQLite; these are not production MySQL latency or memory figures.

| Tasks | Time/op | Allocated bytes/op | Allocations/op |
| ---: | ---: | ---: | ---: |
| 127 | 1.23 ms | 368,003 | 7,263 |
| 501 | 9.41 ms | 1,362,158 | 28,714 |

Run `go test . -run '^$' -bench '^BenchmarkBuildICSForUser$' -benchmem -benchtime=20x -count=1`
from `apps/server` to repeat the benchmark.
