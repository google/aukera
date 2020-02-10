# Aukera

Aukera is a tool developed at Google for scheduling maintenance windows
discoverable via a local API.

## Why Aukera?

Aukera was written with the following goals in mind:

### Code-Based Configuration

Maintenance windows are configured and consumed as JSON. This allows engineers
to leverage source control systems to maintain window definition. By keeping
maintanence window configs in source control, we gain peer review, change history,
rollback/forward, and all the other benefits normally reserved for writing code.

### Flexibility

Aukera is capable of consuming multiple maintenance window configurations. This
allows engineers to define windows pertinent to their service without
conflicting with platform-specific maintenance.

### Stateless Schedule Calculation

Aukera provides a local API for querying for schedules individually or
holistically. Schedule calculation happens when requested, making it possible
for configuration changes to be reflected in the JSON response immediately
afterward.

## Getting Started

Build It!

## Disclaimer

Aukera is maintained by a small team at Google. Support for this repo is
treated as best effort, and issues will be responded to as engineering time
permits.

This is not an official Google product.
