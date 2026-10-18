# Configuration

The process reads environment variables only; it never loads `.env`. Compose reads
its project `.env` for substitution. Keep that file private with mode 0600; do not
use another project's credentials. Empty optional values use defaults.

| Process variable | Default / behavior |
| --- | --- |
| ENERGYTHING_USERNAME | Required, nonempty, no colon; Basic Auth username |
| ENERGYTHING_PASSWORD | Required, nonempty; no shipped password |
| ENERGYTHING_HTTP_ADDR | `:8080`; IP or localhost and port 1–65535 |
| ENERGYTHING_DATA_DIR | `./energything-data`; image `/var/lib/energything-runtime`; unmounted ephemeral runtime root |
| ENERGYTHING_DUMP_DIR | `./energything-dumps`; image `/data`; persistent private dump directory |
| ENERGYTHING_BACKUP_INTERVAL | `15m`; positive duration, wait after previous publication |
| ENERGYTHING_BACKUP_TIMEOUT | `5m`; positive duration, startup/restore/each publication deadline |
| ENERGYTHING_TIMEZONE | `Europe/London`; valid IANA zone, display/calendar interpretation only |
| ENERGYTHING_HISTORY_START | `2022-01-01`, inclusive ISO date, not future; Octopus UTC / Growatt reported dates |
| ENERGYTHING_SHUTDOWN_TIMEOUT | `10s`; positive Go duration, HTTP drain timeout |
| OCTOPUS_API_KEY | Empty disables source unless partially configured |
| OCTOPUS_ACCOUNT_NUMBER | Required together with API key |
| OCTOPUS_IMPORT_MPAN / OCTOPUS_IMPORT_SERIAL | Empty auto-selects one confirmed import meter; otherwise supply matching pair |
| OCTOPUS_EXPORT_MPAN / OCTOPUS_EXPORT_SERIAL | Empty auto-selects one confirmed export meter; otherwise supply matching pair |
| OCTOPUS_REFRESH_INTERVAL | `1h`; positive Go duration |
| GROWATT_API_TOKEN | Empty disables source |
| GROWATT_PLANT_ID | Empty auto-selects unambiguous discovery; otherwise must match |
| GROWATT_DEVICE_SERIAL | Empty auto-selects unambiguous device; otherwise must match |
| GROWATT_LATEST_INTERVAL | `5m`; positive Go duration |
| GROWATT_HISTORY_INTERVAL | `1h`; positive Go duration |
| GROWATT_AGGREGATE_DAILY_CAP_ENABLED | `false`; boolean, enables optional local ten-per-rolling-day cap only |

Five-minute aggregate spacing and provider-directed backoff always apply. Retained
reservations survive flag changes and restarts. History boundary changes retain
frontiers, checkpoints and readings; restart to apply. See [collection](collection.md).
No ONCE, BACKFILL_START or startup RANGE_REFRESH/REQUEST_ID settings are supported.
Use authenticated recent refresh or the durable enqueue CLI for exceptional repair.
There are no alert settings.

Invalid auth, listen address, timezone, history boundary or shutdown timeout is
fatal. Unwritable data directories, incompatible schema or migration failure are
fatal before serving. Provider credential/interval errors disable only that source;
selection errors block affected streams and appear in authenticated status/audit.
No providers configured is supported. A provider outage does not disable stored
browsing or the other provider lane. Configuration validity does not prove remote
authentication. Errors name settings without echoing secret values.

| Compose-only substitution | Default / behavior |
| --- | --- |
| ENERGYTHING_IMAGE | `energything:local`; deployment should set a released digest |
| ENERGYTHING_HTTP_PORT | `8080`; host published port, restrict with firewall/reverse proxy |
| ENERGYTHING_DATA_PATH | `./energything-dumps`; pre-created persistent dump directory mapped to `/data` |
| ENERGYTHING_STOP_GRACE_PERIOD | `6m`; allow HTTP drain, collector cleanup and final dump before SIGKILL |

Compose fixes HTTP to `:8080`, runtime to unmounted `/var/lib/energything-runtime` and dumps to `/data`. Runtime/dump paths must be disjoint and traverse no symlinks. On Linux the actual storage directories must be owned by the service UID and mode 0700 (UID10001 in the image). Linux refuses any separate mount covering runtime. Never mount it or declare a Docker VOLUME there.
Set stop grace above the configured HTTP drain timeout plus five seconds collector
cleanup plus the backup timeout. Increase it whenever either timeout is increased. Runtime
secrets are visible to Docker administrators; restrict host and Docker access.
`TZ=Europe/London` is set in the image for shell tools; application interpretation
uses ENERGYTHING_TIMEZONE. Neither setting establishes Growatt absolute timestamps.

Build controls: `make build CI_BUILD_ID=<public-id>` (or `go run ./scripts/build.go
-build-id <public-id>`), `GO`, `DOCKER`, `IMAGE` are build tooling settings, never
provider configuration. Build IDs appear in authenticated status and must be public.
