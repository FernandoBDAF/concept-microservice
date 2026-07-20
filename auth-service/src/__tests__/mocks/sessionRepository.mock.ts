import { vi, type Mocked } from "vitest";
import type {
  ISessionRepository,
  SessionRecord,
} from "../../domain/repositories/ISessionRepository.js";

export const createMockSessionRepository = (): Mocked<ISessionRepository> => ({
  create: vi.fn(),
  findByRefreshTokenId: vi.fn(),
  findByPreviousTokenId: vi.fn(),
  rotate: vi.fn(),
  revoke: vi.fn(),
});

export const createMockSession = (
  overrides: Partial<SessionRecord> = {}
): SessionRecord => ({
  id: "00000000-0000-0000-0000-0000000000aa",
  userId: "00000000-0000-0000-0000-0000000000bb",
  refreshTokenId: "00000000-0000-0000-0000-0000000000cc",
  previousTokenId: null,
  expiresAt: new Date(Date.now() + 7 * 24 * 3600 * 1000),
  createdAt: new Date(),
  revokedAt: null,
  ...overrides,
});
