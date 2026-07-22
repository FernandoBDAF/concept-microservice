import { afterAll, beforeAll, beforeEach, vi } from "vitest";

beforeAll(() => {
  process.env.NODE_ENV = "test";
  process.env.JWT_SECRET = "test-secret-key-at-least-32-characters-long";
  process.env.DATABASE_PASSWORD = "test-password";
  process.env.LOG_LEVEL = "silent";
});

beforeEach(() => {
  vi.clearAllMocks();
});

afterAll(() => {
  vi.restoreAllMocks();
});

