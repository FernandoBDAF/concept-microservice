import { afterEach, describe, expect, it, vi } from "vitest";

describe("env boolean parsing", () => {
  const savedLogPretty = process.env.LOG_PRETTY;

  afterEach(() => {
    if (savedLogPretty === undefined) {
      delete process.env.LOG_PRETTY;
    } else {
      process.env.LOG_PRETTY = savedLogPretty;
    }
  });

  // Regression test: z.coerce.boolean() runs the JS `Boolean()` constructor, so any
  // non-empty string -- including the literal "false" -- coerces to `true`. Since env
  // vars are always strings, `LOG_PRETTY=false` must resolve to the boolean `false`.
  it("parses LOG_PRETTY=false as boolean false", async () => {
    vi.resetModules();
    process.env.LOG_PRETTY = "false";
    const { env } = await import("../env.js");
    expect(env.LOG_PRETTY).toBe(false);
  });

  it("parses LOG_PRETTY=true as boolean true", async () => {
    vi.resetModules();
    process.env.LOG_PRETTY = "true";
    const { env } = await import("../env.js");
    expect(env.LOG_PRETTY).toBe(true);
  });

  it("defaults LOG_PRETTY to false when unset", async () => {
    vi.resetModules();
    delete process.env.LOG_PRETTY;
    const { env } = await import("../env.js");
    expect(env.LOG_PRETTY).toBe(false);
  });
});
