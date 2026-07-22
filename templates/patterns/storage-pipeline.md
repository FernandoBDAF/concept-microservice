# Pattern: object-storage document pipeline with status lifecycle

Upload a file to an object store, record a metadata row, kick off async
processing, and write the result back onto the row — the way `api-service`
handles documents. The interesting parts are the *consistency* seams between
three stores (object store, Postgres, broker) and the status lifecycle that
lets a client poll a document to completion.

Every concrete value below was read from the POST-v4 tree
(`/…/api-service`). Where a section rests on a not-yet-run experiment or a
future phase, it says so inline — do not read those as measured facts.

> **Scope note / correction to the outline.** The outline framed this as a
> *presigned-PUT upload* pipeline. The lab code does not do that: uploads are
> a **server-side proxy** — the client POSTs multipart to `api-service`, which
> streams bytes to MinIO via `PutObject`. Presigned URLs are used **only for
> downloads (GET)**. This doc documents what the code does and flags the
> presigned-PUT variant as an adaptation option.

## Context

Use this shape when a user uploads a blob that needs durable object storage, a
queryable metadata record, and asynchronous post-processing (thumbnailing,
extraction, indexing), and the client needs to see when processing finishes.
Three stores are involved and none of them is transactional with the others —
so the pattern is really about *what happens when a crash lands between two
writes*.

## The pattern as the lab implements it

### Upload (`api-service/internal/domain/document/service.go`, `Service.Upload`)

1. Validate type/mime/size.
2. **Upload the object bytes to MinIO first** (`s.minio.Upload → PutObject`).
   Object key: `{profileID}/{YYYY}/{MM}/{DD}/{documentUUID}{ext}`; bucket
   `documents-raw` (default, env `MINIO_BUCKET_NAME`).
3. Build the `document.process` task envelope.
4. **Insert the metadata row and the outbox row in one Postgres transaction**
   (`repo.CreateWithTask`, `internal/infrastructure/postgres/document_repository.go`):
   `INSERT INTO documents` (status `pending`) → `outbox.Add(tx, …)` →
   `UPDATE documents SET status = processing` → `tx.Commit()`.

So the object lands first, then the row + outbox + `pending→processing` commit
atomically. There is no partial "row without outbox" state, because they share
the transaction.

### Compensating delete on failure

If a step *after* the object upload fails — envelope build or the
`CreateWithTask` transaction — the service issues a **compensating delete** of
the just-uploaded object (`_ = s.minio.Delete(ctx, storagePath)` in
`service.go`). Direction matters: because the object is written before the row,
the only orphan the happy path can create is *object-without-row*, and that is
exactly what the compensating delete cleans up. Note it is **best-effort — the
delete error is swallowed** — so a failed compensating delete permanently
orphans the object with nothing to reconcile it later (see the reconciliation
gap below). The standalone `Service.Delete` path has the same property: if the
MinIO delete fails it logs and still deletes the row.

### Publish-after-commit via the transactional outbox (EXP-42 target)

The lab implements a full transactional outbox (ADR-008.3) so a message is
never published for an uncommitted row and never lost after a committed one:

- **Table** (`api-service/migrations/000003_create_outbox.up.sql`):
  `outbox(id BIGSERIAL, routing_key, envelope JSONB, created_at, sent_at,
  attempts)` with a partial index `WHERE sent_at IS NULL` for cheap
  pending scans.
- **Same-tx insert**: `outbox.Add(ctx, tx, routingKey, envelope)` runs inside
  the upload transaction (`outbox/outbox.go`), so the event commits with the
  row or not at all.
- **Relay loop** (`outbox.Relay`, started as a goroutine in
  `cmd/server/main.go`): every 250ms it selects unsent rows
  `FOR UPDATE SKIP LOCKED`, publishes each to RabbitMQ in **confirm mode**
  (`PublishRaw`), and stamps `sent_at`; failures bump `attempts` and are
  retried. Duplicate publishes are tolerated by design — the downstream worker
  dedupes on the envelope id (that is why the whole envelope is stored).

This is the crash-consistency guarantee: a `kill -9` in the commit→publish
window leaves the event durably in `outbox`, and the relay republishes it on
restart.

> **Evidence status — EXP-42 is authored but NOT yet live-run.** The
> crash-consistency drill ("kill api-service in the commit→publish window
> under upload load; relay recovers; document reaches `completed`") is
> specified in `EXPERIMENTS.md` §EXP-42 and listed as deferred in
> `documentation/phases/v4-DEFERRED.md` and `documentation/phases/v4-HANDOFF.md`
> §A5. The code above is present and reviewed; the *runtime proof* is pending.
> Treat the crash-recovery claim as designed-and-code-complete, not yet
> empirically demonstrated.

### Status lifecycle via `task-results`

Statuses (`internal/domain/document/model.go`): `pending`, `processing`,
`completed`, `failed`.

- `pending` is only the pre-commit in-memory value; the row is `processing`
  the moment the upload transaction commits.
- Workers publish a terminal result to exchange **`task-results`**, routing key
  `task.result`, with status `completed` or `failed` and the `document_id`.
- `api-service` runs a **results consumer** (`ResultsConsumer.Start` in
  `internal/infrastructure/rabbitmq/results_consumer.go`, wired in `main.go`):
  it passively verifies the queue, manually acks, and on handler error
  `Nack(requeue=true)`. The handler (`internal/domain/task/results.go`) decodes
  the envelope and calls `document.Service.ApplyTaskResult`, which writes back
  `UpdateProcessingCompleted` or `UpdateStatus(..., failed, errorMsg)`.
- The write-back is **idempotent**: unknown document → dropped; already-terminal
  document → skipped. So a requeued/duplicate result is safe.

Note there is no explicit worker-sent "processing" ping — a document sits in
`processing` (set at commit) until a terminal result arrives. This closes the
EXP-11 "document status is write-only" gap
(`documentation/review/CONCEPTUAL_REVIEW.md` §12).

### Presigned download URLs (15m)

Download is offloaded from the API to the object store via a presigned GET:
`GetPresignedURL → minio-go PresignedGetObject` with expiry **`15*time.Minute`**
(`internal/domain/document/service.go`), and the handler advertises
`"expires_in": "15 minutes"` to the client
(`internal/api/handlers/document.go`). 15m is the deliberate choice: long
enough for a client to follow the redirect and download, short enough that a
leaked URL expires quickly.

> **S3-vs-MinIO presign behavior — expected from v5, not yet verified.** The
> code speaks the S3 API, and v5 swaps MinIO for real S3
> (`documentation/decisions/006-aws.md`). The v5 plan explicitly calls out
> "verify presigned URL behavior on real S3"
> (`documentation/phases/v5-aws.md`, the aws-overlay task) as work to do; the
> exit runs EXP-50 (`aws-up` smoke/burst) and EXP-51 (catalog on EKS) are where
> that would be exercised. **As of this tree that verification does not exist
> yet** — expect real-S3 quirks (SigV4 clock-skew sensitivity, virtual-host vs
> path-style addressing, region-scoped signatures, and differing max-expiry
> caps) to surface there. Do not assume MinIO presign behavior transfers
> unchanged.
>
> (Minor note: the outline attributed the S3-vs-MinIO evidence to "EXP-50";
> in the source, EXP-50 is the *session-lifecycle* run — the presign-on-real-S3
> check lives in the aws-overlay task and would be observed during EXP-50/51.)

## Failure modes and gaps

- **Orphan objects — no reconciliation sweep (ABSENT).** Confirmed absent in
  the tree (grep for `orphan|reconcil|sweep` finds no job). Because the
  compensating delete is best-effort and swallows its error, and because the
  `Delete` path continues past a failed object delete, the system can
  accumulate objects-without-rows over time with nothing to clean them up.
  This is the single biggest hardening gap in the pipeline. See the design
  below.
- **Storage consistency across three stores** was the original motivation for
  the outbox (`documentation/review/CONCEPTUAL_REVIEW.md` §9). The outbox
  closes the DB↔broker seam; the object↔DB seam is only covered on the happy
  compensating path, not against a lost compensating delete.
- **Duplicate results** are safe (idempotent write-back), and **duplicate
  publishes** are safe (downstream envelope-id dedupe) — these seams are
  handled.

### Orphan reconciliation sweep — design (not yet built)

A periodic reconciliation job is the standard answer to the best-effort-delete
gap. Two directions:

- **Objects without rows**: list the bucket, and for each object whose
  `documentUUID` (parse it from the key) has no `documents` row past a grace
  period, delete the object. These are failed compensating deletes.
- **Rows without objects**: for each `documents` row, `StatObject` the
  `storage_path`; a row pointing at a missing object past a grace period is
  marked `failed` (or repaired). These arise from external object deletion.

Run it as a low-frequency `CronJob` with a grace window (never act on
in-flight uploads), operate in dry-run/report mode first, and emit a metric so
a nonzero orphan count is visible. This is offered as a design; it is not in
the lab code today.

## Adaptation checklist

- [ ] Decide upload style: server-side proxy (lab default, simple, streams
      through your API) vs presigned PUT (offloads bytes from the API but needs
      a client that can PUT and a callback/verify step to learn the object
      landed). The lab does proxy uploads and presigned *downloads* only.
- [ ] Keep the ordering: object first, then commit (row + outbox) atomically,
      then compensate the object if the commit fails.
- [ ] Make the compensating delete *not* best-effort if you cannot afford
      orphans — or accept orphans and build the reconciliation sweep.
- [ ] Use a transactional outbox for the DB→broker publish; relay in confirm
      mode; store the whole envelope so downstream can dedupe on its id.
- [ ] Model status as `pending→processing→completed|failed`, write results
      back through an idempotent, requeue-safe consumer, and skip terminal rows.
- [ ] Pick a presign expiry deliberately (lab: 15m) and re-verify it against
      your real object store — presign semantics differ between MinIO and S3.
- [ ] Build the orphan reconciliation sweep before you run at any real volume.
- [ ] Run the crash-in-commit→publish drill (the EXP-42 scenario) yourself —
      the lab has it specified but not yet executed.
