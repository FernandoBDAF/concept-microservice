/**
 * Security audit trail (auth_audit_logs, migration 001). Deliberately minimal:
 * the template records exactly ONE out-of-band event — refresh-token reuse, the
 * theft signal from rotation (ADR-009.2). The lab's broader audit surface (per-
 * request IP/user-agent capture, login/logout trails) was trimmed on extraction;
 * add fields back here and in the migration if your service needs them.
 */
export interface AuditLogEntry {
  userId?: string | null;
  action: string;
  success: boolean;
  details?: Record<string, unknown>;
}

export interface IAuditLogRepository {
  record(entry: AuditLogEntry): Promise<void>;
}
