/**
 * Security audit trail (auth_audit_logs, migration 001). Used for events worth
 * recording out of band — notably refresh-token reuse (ADR-009.2).
 */
export interface AuditLogEntry {
  userId?: string | null;
  action: string;
  success: boolean;
  details?: Record<string, unknown>;
  ipAddress?: string | null;
  userAgent?: string | null;
}

export interface IAuditLogRepository {
  record(entry: AuditLogEntry): Promise<void>;
}
