import type {
  AuditLogEntry,
  IAuditLogRepository,
} from "./IAuditLogRepository.js";
import { db } from "../../infrastructure/database/connection.js";
import { logger } from "../../infrastructure/logging/logger.js";

export class AuditLogRepository implements IAuditLogRepository {
  private readonly log = logger.child({ repository: "AuditLogRepository" });

  async record(entry: AuditLogEntry): Promise<void> {
    try {
      await db.query(
        `INSERT INTO auth_audit_logs (user_id, action, ip_address, user_agent, success, details)
         VALUES ($1, $2, $3, $4, $5, $6)`,
        [
          entry.userId ?? null,
          entry.action,
          entry.ipAddress ?? null,
          entry.userAgent ?? null,
          entry.success,
          entry.details ? JSON.stringify(entry.details) : null,
        ]
      );
    } catch (error) {
      // Auditing must never break the request path it observes.
      this.log.error(
        { err: error, action: entry.action },
        "Failed to write audit log"
      );
    }
  }
}

export const auditLogRepository = new AuditLogRepository();
