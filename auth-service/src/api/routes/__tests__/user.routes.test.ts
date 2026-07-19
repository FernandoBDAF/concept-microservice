import { describe, expect, it, beforeAll, beforeEach, vi } from "vitest";
import request from "supertest";
import type { Express } from "express";
import { createApp } from "../../../app.js";
import { tokenService } from "../../../domain/services/TokenService.js";
import { db } from "../../../infrastructure/database/connection.js";
import { createMockUser } from "../../../__tests__/factories/user.factory.js";

vi.mock("../../../infrastructure/database/connection.js", () => ({
  db: {
    query: vi.fn(),
    healthCheck: vi.fn().mockResolvedValue(true),
    transaction: vi.fn(),
  },
}));

// requiresAuth loads the caller from the DB to read its role; return a row for
// whichever user we mint a token for (ADR-009.7 role enforcement).
function mockCurrentUser(user: ReturnType<typeof createMockUser>): void {
  const row = {
    id: user.id,
    email: user.email,
    hashed_password: "hashed",
    role: user.role,
    is_active: user.isActive,
    failed_attempts: 0,
    locked_until: null,
    created_at: new Date(),
    updated_at: new Date(),
  };
  vi.mocked(db.query).mockResolvedValue({
    rows: [row],
  } as unknown as Awaited<ReturnType<typeof db.query>>);
}

describe("User routes — admin role enforcement", () => {
  let app: Express;

  beforeAll(() => {
    app = createApp();
  });

  beforeEach(() => {
    vi.mocked(db.query).mockReset();
  });

  it("401s when no token is presented on an admin route", async () => {
    const res = await request(app).get("/v1/users");
    expect(res.status).toBe(401);
  });

  it("403s a normal user on the admin-only user list", async () => {
    const user = createMockUser({ role: "user" });
    mockCurrentUser(user);
    const { accessToken } = tokenService.generateTokens(user);

    const res = await request(app)
      .get("/v1/users")
      .set("Authorization", `Bearer ${accessToken}`);

    expect(res.status).toBe(403);
    const body = res.body as { status: string };
    expect(body.status).toBe("error");
  });

  it("403s a normal user on the admin-only delete route", async () => {
    const user = createMockUser({ role: "user" });
    mockCurrentUser(user);
    const { accessToken } = tokenService.generateTokens(user);

    const res = await request(app)
      .delete(`/v1/users/${user.id}`)
      .set("Authorization", `Bearer ${accessToken}`);

    expect(res.status).toBe(403);
  });
});
