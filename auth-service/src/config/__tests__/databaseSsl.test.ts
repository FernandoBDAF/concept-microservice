import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

// Resolution of the pg `ssl` option from DATABASE_SSL / DATABASE_SSL_CA
// (WP-external, RDS force_ssl). The logic lives in config/index.ts
// (resolveDatabaseSsl); we assert config.database.ssl across the three branches.
describe("database ssl resolution", () => {
  const saved: Record<string, string | undefined> = {};
  let caPath: string;
  const CA_CONTENTS = "-----BEGIN CERTIFICATE-----\nTESTCA\n-----END CERTIFICATE-----\n";

  beforeAll(() => {
    const dir = mkdtempSync(join(tmpdir(), "auth-ca-"));
    caPath = join(dir, "test-ca.pem");
    writeFileSync(caPath, CA_CONTENTS);
  });

  afterEach(() => {
    for (const key of ["DATABASE_SSL", "DATABASE_SSL_CA"]) {
      if (saved[key] === undefined) delete process.env[key];
      else process.env[key] = saved[key];
    }
  });

  const stash = () => {
    saved.DATABASE_SSL = process.env.DATABASE_SSL;
    saved.DATABASE_SSL_CA = process.env.DATABASE_SSL_CA;
  };

  it("disables TLS when DATABASE_SSL is false", async () => {
    stash();
    vi.resetModules();
    process.env.DATABASE_SSL = "false";
    delete process.env.DATABASE_SSL_CA;
    const { config } = await import("../index.js");
    expect(config.database.ssl).toBe(false);
  });

  it("verifies against the CA when DATABASE_SSL=true and DATABASE_SSL_CA is set", async () => {
    stash();
    vi.resetModules();
    process.env.DATABASE_SSL = "true";
    process.env.DATABASE_SSL_CA = caPath;
    const { config } = await import("../index.js");
    expect(config.database.ssl).toEqual({
      ca: CA_CONTENTS,
      rejectUnauthorized: true,
    });
  });

  it("enables unverified TLS and warns once when DATABASE_SSL=true without a CA", async () => {
    stash();
    vi.resetModules();
    process.env.DATABASE_SSL = "true";
    delete process.env.DATABASE_SSL_CA;
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { config } = await import("../index.js");
    expect(config.database.ssl).toEqual({ rejectUnauthorized: false });
    expect(warnSpy).toHaveBeenCalledTimes(1);
    expect(warnSpy.mock.calls[0][0]).toMatch(/NOT verifying/i);
    warnSpy.mockRestore();
  });

  it("fails fast when DATABASE_SSL_CA points at an unreadable file", async () => {
    stash();
    vi.resetModules();
    process.env.DATABASE_SSL = "true";
    process.env.DATABASE_SSL_CA = "/nonexistent/path/to/rds-ca.pem";
    await expect(import("../index.js")).rejects.toThrow(/CA file could not be read/);
  });
});
