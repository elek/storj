# Network stats export chore

Periodically write a small JSON snapshot of network-wide numbers to a file, so
that something outside the satellite (a static site, a status page, a scraper)
can serve them without talking to the satellite database.

```json
{
  "updated": "2026-09-19T20:00:00Z",
  "nodes_online": 1,
  "users": 1,
  "data_stored_bytes": 0,
  "free_space_bytes": 0
}
```

## Decisions

### Four cheap aggregates, one round trip each

Every number is a single aggregate query against the satellite database. None of
them scan the metabase.

| field               | source                                                              |
|---------------------|---------------------------------------------------------------------|
| `nodes_online`      | `count(*)` over `nodes` contacted within the online window           |
| `free_space_bytes`  | `sum(free_disk)` over the same node set                              |
| `users`             | `count(*)` over `users` with `status = 1` (active)                   |
| `data_stored_bytes` | `sum(total_bytes)` over the most recent `bucket_storage_tallies` run |

`nodes_online` and `free_space_bytes` come from one query, since they share a
predicate. So the chore issues three queries per interval.

The node predicate is the one `GetAllParticipatingNodes` uses
(`satellitedb/overlaycache.go:533`): `disqualified IS NULL AND exit_finished_at
IS NULL`, plus `last_contact_success > now - onlineWindow` for online. Reusing it
means the count matches what node selection considers part of the network.
`free_disk` defaults to `-1` for a node that has never checked in; those rows are
excluded from the sum so a fleet of never-contacted nodes cannot drive the total
negative. In practice the online window already excludes them.

### `data_stored_bytes` is user data, from the last tally

`data_stored_bytes` is what customers uploaded, not what is physically on nodes
— the latter is roughly 2.7x larger after erasure coding. The value is therefore
read from `bucket_storage_tallies` rather than from node piece counts.

A live sum would mean `sum(encrypted_size)` over `segments`, a full metabase
scan, which is not something to run on an interval. The tally table is the
existing precomputed answer, so the number is as fresh as the last tally run
(hourly in production) rather than live. That staleness is the accepted cost;
`updated` reflects when the file was written, not when the tally ran.

`SaveTallies` (`satellitedb/projectaccounting.go:49`) writes every bucket of one
tally run with an identical `interval_start`, so "the latest tally" is exactly
`interval_start = (SELECT max(interval_start) FROM bucket_storage_tallies)`, and
`bucket_storage_tallies_interval_start_index` makes both halves indexed lookups.

Rows written before `total_bytes` existed carry `0` there and the real value in
`inline + remote`. Every other reader of this table falls back the same way
(`projectaccounting.go:1258`, `:107`), so this query does too.

### Disabled unless a path is configured

`stats-export.path` defaults to empty, which means the chore does nothing. An
existing deployment that picks up this code writes no files and issues no extra
queries until someone sets the path. `stats-export.interval` defaults to 10
minutes.

### Atomic writes

Readers are expected to be external and uncoordinated, so a reader must never
observe a half-written file. Each run writes a temporary file in the destination
directory and renames it over the target; rename within a directory is atomic.
Writing into the destination directory rather than `TMPDIR` keeps the rename on
one filesystem, where it would otherwise degrade to a copy.

### Wiring

Registered as a mud module and selected in `satellite/run/core.go`, so it runs in
the core peer. The core peer already holds every database the chore needs, and
the queries are cheap enough that a separate process would buy nothing.

## Layout

| file                                  | contents                                |
|---------------------------------------|-----------------------------------------|
| `satellite/statsexport/chore.go`      | `Config`, `Stats`, `DB`, `Chore`        |
| `satellite/statsexport/mud.go`        | mud module                              |
| `satellite/satellitedb/statsexport.go`| the three queries                       |
| `satellite/run/core.go`               | selector entry                          |

`Stats` carries the JSON tags and is what gets marshalled, so the file format is
defined in one place next to the chore that writes it.
