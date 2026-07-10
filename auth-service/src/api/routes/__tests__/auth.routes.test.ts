import { describe, expect, it, beforeAll, vi } from "vitest";
import request from "supertest";
import { createApp } from "../../../app.js";
import type { Express } from "express";

vi.mock("../../../infrastructure/database/connection.js", () => ({
  db: {
    query: vi.fn(),
    healthCheck: vi.fn().mockResolvedValue(true),
    transaction: vi.fn(),
  },
}));

describe("Auth Routes", () => {
  let app: Express;

  beforeAll(() => {
    app = createApp();
  });

  it("returns 400 for missing credentials", async () => {
    const response = await request(app).post("/v1/auth/login").send({});
    const { status, body } = response as {
      status: number;
      body: { status: string };
    };
    expect(status).toBe(400);
    expect(body.status).toBe("error");
  });

  it("returns 400 for invalid email format", async () => {
    const response = await request(app)
      .post("/v1/auth/login")
      .send({ email: "invalid-email", password: "password123" });
    expect(response.status).toBe(400);
  });

  it("returns 400 for missing token validation", async () => {
    const response = await request(app).post("/v1/auth/token/validate").send({});
    const { status } = response as { status: number };
    expect(status).toBe(400);
  });

  it("returns health status", async () => {
    const response = await request(app).get("/health");
    const { status, body } = response as {
      status: number;
      body: { status: string };
    };
    expect(status).toBe(200);
    expect(body.status).toBe("healthy");
  });
});

