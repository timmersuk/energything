# Energything: goal and staged delivery

Created 13 October 2026. Planning document; implementation has not started.

## Goal

Build a small self-hosted home energy application that retains electricity and
inverter data locally, explains usage and solar/battery value over time, and can
eventually help choose cheaper battery charging schedules. Deploy to an
OpenMediaVault Docker host with logthing's operational shape: one Go process,
an embedded browser UI, one container and persistent local storage.

The first useful outcome is a working collector with a UI. Bring Octopus and
Growatt into milestone 1 so useful history starts accumulating early. Rich charts
and Octopus presentation parity follow later.

The public repository contains service code, synthetic tests and generic docs.
Keep the existing octopull workspace local for private investigations and data.
Selectively reuse reviewed code and protocol knowledge; do not copy its reports,
credentials, data or household identifiers into this repository.

## Delivery stages

| Milestone | Outcome | Completion boundary |
| --- | --- | --- |
| **1. Running service and durable collection** | A logthing-shaped application collecting Octopus electricity imports/exports and read-only Growatt cloud telemetry, storing them locally and displaying status and records in a basic UI | Both sources collect automatically, retain history across restarts and show independent success/failure/coverage plus durable attempt audit in the deployed UI and REST API. See [the detailed plan](milestone-1-service-and-collection.md). |
| **2. Energy and tariff dashboard** | Day/week/month/year navigation; electricity kWh and calculated cost/revenue; standing charges; retained Agile import/export rates for today and previous days, tomorrow when published; gas usage where supported | Calculations respect effective tariffs, units, gaps and UK calendar time. Charts and tables provide useful Octopus-like breakdowns; estimates remain distinct from bills and payments. |
| **3. Seasonal solar and battery understanding** | Hour-by-month profiles, seasonal trends, solar production, self-consumption, export value and avoided purchases | Reconciled inverter/meter data and visible coverage support meaningful comparisons. Explain when solar contributes through the year; financial payback is optional and needs installation cost and explicit assumptions. |
| **4. Charging suggestions and shadow operation** | Suggested cheap charging windows with forecast-based replay and observed comparisons | Recommendations account for real SOC/headroom, charging limits, efficiency and export opportunity cost. No inverter settings change. Present bill savings directly; any battery-wear sensitivity is separate. |
| **5. Optional inverter control** | Explicitly enabled scheduling after supported settings and recovery behaviour are verified | Bounded writes, readback, observed execution, manual override, stale-data behaviour, audit history and recovery are established. A separate future decision, not implied by telemetry collection. |

Local Growatt access is an optional later investigation, not a gate for milestone 1.
Start with cloud collection; evaluate a local adapter when its benefit justifies
device-specific work. Retain stored history and UI contracts. A proxy, replacement
logger firmware or server emulation is not automatically read-only or control-capable.

## Architecture direction

- Go serves pages through html/template, with plain CSS and modest JavaScript.
  Embed templates and static assets in the binary. Use HTMX only where fragment
  updates simplify interactions. Authentication protects records and UI; the
  initial build needs no Node, pnpm or Vite.
- Collection runs in the application, independently per source, with durable
  progress and retries. Browsing stored history does not require cloud requests.
- Recommend SQLite as the primary store, accessed through a pure-Go driver. One
  container and a host-mounted data directory; no database server. The milestone
  plan records the trade-offs and build/runtime validation gate.
- Keep protocol details inside source modules and SQL/migrations inside storage.
  Introduce replacement adapters when there is a real second implementation.
- Preserve measured data, provenance and quality. Missing data is not zero;
  grid exports are not generation; sampled power is not interval energy.

### UI fallback for later milestones

If later UI features become difficult to implement or maintain with templates
and modest JavaScript, reconsider React/Vite using logthing's frontend model:
build a separate frontend and embed its generated assets in the Go binary.
Triggers include complex interconnected browser state or increasingly tangled
interaction code. Charts alone do not require this switch. Preserve collection,
storage, authentication and reusable data/query interfaces so a UI change does
not require rebuilding the service. Adopt the extra build tooling only when
concrete feature work demonstrates its value.

## Privacy and operations

No real credentials, household identifiers, telemetry, costs, private screenshots
or investigation results belong in public source, fixtures, docs, CI output,
artifacts or image layers. Supply runtime secrets privately and exclude runtime
files from Git and Docker contexts. CI uses synthetic data and mock cloud responses.
The deployed UI and data are private even though the source is public.

History survives image replacement. Keep backup/restore, outage visibility and
bounded resource use practical for a home deployment. Use Europe/London for
calendar views while retaining source timestamps and established absolute instants.

## Planning status

Milestone 1 is the immediate target. Later milestones express direction, not
fixed commitments or dates. Revisit their scope/order once collection runs.
Milestone 1 makes recommended defaults explicit so implementation can begin;
validate those defaults during the first implementation steps.

