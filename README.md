# Aukera

[![Go Tests](https://github.com/google/aukera/workflows/Go%20Tests/badge.svg)](https://github.com/google/aukera/actions?query=workflow%3A%22Go+Tests%22)
[![release](https://github.com/google/aukera/actions/workflows/release.yml/badge.svg)](https://github.com/google/aukera/actions/workflows/release.yml)

Aukera is a tool developed at Google for scheduling maintenance windows
discoverable via a local API.

## Why Aukera?

Aukera was written with the following goals in mind:

### Code-Based Configuration

Maintenance windows are configured and consumed as JSON. This allows engineers
to leverage source control systems to maintain window definition. By keeping
maintenance window configs in source control, we gain peer review, change
history, rollback/forward, and all the other benefits normally reserved for
writing code.

### Flexibility

Aukera is capable of consuming multiple maintenance window configurations. This
allows engineers to define windows pertinent to their service without
conflicting with platform-specific maintenance.

### Stateless Schedule Calculation

Aukera provides a local API for querying for schedules individually or
holistically. Schedule calculation happens when requested, making it possible
for configuration changes to be reflected in the JSON response immediately
afterward.

## Maintenance Windows & Schedules

Aukera consumes window configurations from JSON files stored in its
configuration directory (`%ProgramData%\Aukera\conf` on Windows,
`/etc/aukera` on Linux):

```json
{
  "Windows": [
    {
      "Name": "monthly_patching",
      "Format": 1,
      "Schedule": "CRON_TZ=America/New_York 0 17 * * Tue#2",
      "Duration": "4h",
      "Labels": ["patching"]
    }
  ]
}
```

Format `1` schedules support standard 6-field CRON expressions (with seconds
first), as well as Quartz-style `#` notation (`<weekday>#<occurrence>` or
`<weekday>#L`) in the day-of-week field for recurring monthly schedules (e.g.
Patch Tuesday). When using `#` expressions, standard 5-field crontab syntax is
also accepted (seconds default to `0`). Expressions may optionally include a
`TZ=` or `CRON_TZ=` timezone prefix.

Examples:

*   `0 17 * * Tue#2` — 2nd Tuesday of the month at 17:00 (Patch Tuesday)
*   `0 0 18 * * Fri#L` — Last Friday of the month at 18:00:00
*   `CRON_TZ=America/New_York 0 17 * * Tue#2` — 2nd Tuesday at 17:00
    America/New_York time
*   `0 30 14 * * 6` — Every Saturday at 14:30:00 (standard 6-field CRON)

Schedules can be queried via the local HTTP API (`GET /schedule` or
`GET /schedule/{label}`).

## Getting Started

Pre-compiled binaries are available as
[release assets](https://github.com/google/aukera/releases).

Building Aukera manually:

1.  Clone the repository
1.  Install any missing imports with `go get -u`
1.  Run `go build C:\Path\to\aukera\src`

## Disclaimer

Aukera is maintained by a small team at Google. Support for this repo is treated
as best effort, and issues will be responded to as engineering time permits.

This is not an official Google product.
