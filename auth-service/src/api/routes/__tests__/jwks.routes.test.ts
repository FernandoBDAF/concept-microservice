import { describe, expect, it, beforeAll, afterAll, vi } from "vitest";
import { generateKeyPairSync } from "node:crypto";
import request from "supertest";
import type { Express } from "express";
import { createMockUser } from "../../../__tests__/factories/user.factory.js";

// Importing the app pulls the repositories, which construct a DB pool. Mock the
// connection so no real DB is needed. The mock survives vi.resetModules().
vi.mock("../../../infrastructure/database/connection.js", () => ({
  db: {
    query: vi.fn(),
    healthCheck: vi.fn().mockResolvedValue(true),
    transaction: vi.fn(),
  },
}));

interface Jwk {
  kty: string;
  use: string;
  alg: string;
  kid: string;
  n: string;
  e: string;
}

// Reload config + services under an RS256 keypair supplied as base64(PEM),
// exactly as compose/k8s inject JWT_PRIVATE_KEY / JWT_PUBLIC_KEY.
describe("JWKS route + RS256 signing", () => {
  let app: Express;
  let tokenService: import("../../../domain/services/TokenService.js").TokenService;
  let jwksService: import("../../../domain/services/JwksService.js").JwksService;

  const saved = {
    priv: process.env.JWT_PRIVATE_KEY,
    pub: process.env.JWT_PUBLIC_KEY,
    alg: process.env.JWT_ALGORITHM,
  };

  beforeAll(async () => {
    const { privateKey, publicKey } = generateKeyPairSync("rsa", {
      modulusLength: 2048,
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });

    process.env.JWT_PRIVATE_KEY = Buffer.from(privateKey).toString("base64");
    process.env.JWT_PUBLIC_KEY = Buffer.from(publicKey).toString("base64");
    process.env.JWT_ALGORITHM = "RS256";

    vi.resetModules();
    const appMod = await import("../../../app.js");
    const tokenMod = await import("../../../domain/services/TokenService.js");
    const jwksMod = await import("../../../domain/services/JwksService.js");
    app = appMod.createApp();
    tokenService = tokenMod.tokenService;
    jwksService = jwksMod.jwksService;
  });

  afterAll(() => {
    const restore = (k: string, v: string | undefined) => {
      if (v === undefined) delete process.env[k];
      else process.env[k] = v;
    };
    restore("JWT_PRIVATE_KEY", saved.priv);
    restore("JWT_PUBLIC_KEY", saved.pub);
    restore("JWT_ALGORITHM", saved.alg);
    vi.resetModules();
  });

  it("serves /.well-known/jwks.json with one RSA signing key (no auth)", async () => {
    const res = await request(app).get("/.well-known/jwks.json");
    expect(res.status).toBe(200);

    const body = res.body as { keys: Jwk[] };
    expect(Array.isArray(body.keys)).toBe(true);
    expect(body.keys).toHaveLength(1);

    const [key] = body.keys;
    expect(key).toMatchObject({ kty: "RSA", use: "sig", alg: "RS256" });
    expect(key?.kid).toBeTruthy();
    expect(key?.n).toBeTruthy();
    expect(key?.e).toBeTruthy();
  });

  it("signs RS256 with a kid matching the JWKS and verifies the round-trip", () => {
    const user = createMockUser();
    const tokens = tokenService.generateTokens(user);

    const headerJson = Buffer.from(
      tokens.accessToken.split(".")[0] ?? "",
      "base64url"
    ).toString("utf8");
    const header = JSON.parse(headerJson) as { alg: string; kid?: string };
    expect(header.alg).toBe("RS256");

    const jwksKid = jwksService.jwks().keys[0]?.kid;
    expect(header.kid).toBe(jwksKid);

    const decoded = tokenService.verifyAccessToken(tokens.accessToken);
    expect(decoded.userId).toBe(user.id);
    expect(decoded.tokenType).toBe("ACCESS_TOKEN");
  });
});
