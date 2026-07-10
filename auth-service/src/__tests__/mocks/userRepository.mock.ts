import { vi, type Mocked } from "vitest";
import type { IUserRepository } from "../../domain/repositories/IUserRepository.js";

export const createMockUserRepository = (): Mocked<IUserRepository> => ({
  create: vi.fn(),
  findById: vi.fn(),
  findByEmail: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  list: vi.fn(),
  recordLoginAttempt: vi.fn(),
  validatePassword: vi.fn(),
});

