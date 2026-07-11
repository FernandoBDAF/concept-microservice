# Decision records

Lightweight ADRs. Convention: **one file per theme**, individual decisions
numbered inside it (`ADR-006.3` = file 006, decision 3) so a future record can
supersede a single decision without rewriting the theme. Format per decision:
context → decision → consequences, kept to a few lines.

ADR-001..010 were decided in one owner Q&A session on 2026-07-10 (all open
questions from the PRD, asked with context and trade-offs). The
[phase documents](../phases/) embed these decisions where they're implemented;
the ADRs are the greppable "why" trail.

| File | Theme |
|---|---|
| [001](001-methodology-and-sequencing.md) | Methodology & roadmap sequencing |
| [002](002-cluster-platform.md) | Local cluster platform (v2) |
| [003](003-observability.md) | Observability stack (v3) |
| [004](004-experiments-and-workloads.md) | Experiments, chaos, load generation (v4) |
| [005](005-mission-control.md) | Mission Control UI (v6) |
| [006](006-aws.md) | AWS track (v5) |
| [007](007-host-and-guests.md) | Host contract & guest systems (v7) |
| [008](008-messaging.md) | Messaging architecture |
| [009](009-auth-and-security.md) | Auth & security |
| [010](010-process.md) | Process: CI, templates, legacy, tagging |

Adding a decision: append to the matching theme file with the next sub-number,
or start a new numbered file for a new theme. Superseding: state
`Supersedes ADR-XXX.Y` in the new entry and mark the old one
`Superseded by ADR-ZZZ.W`.
