-- Transactional outbox: domain writes and their task events commit in one
-- transaction; the in-process relay publishes pending rows with broker
-- confirms and stamps sent_at. The envelope is stored whole (JSONB) so the
-- relay needs no domain knowledge and duplicates can be deduped downstream on
-- the envelope id (consumer-side idempotency — see the worker-go template).
--
-- Provenance: extracted from api-service migration 000003_create_outbox
-- (post-v4). Renumbered to 000001 as this module's first migration.
CREATE TABLE outbox (
    id          BIGSERIAL PRIMARY KEY,
    routing_key VARCHAR(64)  NOT NULL,
    envelope    JSONB        NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    sent_at     TIMESTAMPTZ,
    attempts    INTEGER      NOT NULL DEFAULT 0
);
-- Partial index makes the relay's "pending rows" scan and the
-- api_outbox_pending gauge cheap regardless of table size.
CREATE INDEX idx_outbox_pending ON outbox (id) WHERE sent_at IS NULL;
