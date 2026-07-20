-- Transactional outbox (ADR-008.3): domain writes and their task events
-- commit in one transaction; the in-process relay publishes pending rows
-- with confirms and stamps sent_at. Envelope is stored whole (JSONB) so the
-- relay needs no domain knowledge and duplicates can be deduped downstream
-- on envelope id (ADR-008.2).
CREATE TABLE outbox (
    id          BIGSERIAL PRIMARY KEY,
    routing_key VARCHAR(64)  NOT NULL,
    envelope    JSONB        NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    sent_at     TIMESTAMPTZ,
    attempts    INTEGER      NOT NULL DEFAULT 0
);
CREATE INDEX idx_outbox_pending ON outbox (id) WHERE sent_at IS NULL;
