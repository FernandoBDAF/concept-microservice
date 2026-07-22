import { vi, type Mocked } from "vitest";
import type { IAuditLogRepository } from "../../domain/repositories/IAuditLogRepository.js";

export const createMockAuditLogRepository = (): Mocked<IAuditLogRepository> => ({
  record: vi.fn(),
});
