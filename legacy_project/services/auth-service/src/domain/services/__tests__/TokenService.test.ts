import { describe, expect, it } from "vitest";
import { TokenService } from "../TokenService.js";
import { createMockUser } from "../../../__tests__/factories/user.factory.js";

describe("TokenService", () => {
  it("generates access and refresh tokens", () => {
    const service = new TokenService();
    const user = createMockUser();

    const tokens = service.generateTokens(user);

    expect(tokens.accessToken).toBeDefined();
    expect(tokens.refreshToken).toBeDefined();
    expect(tokens.tokenType).toBe("bearer");
  });

  it("verifies access token", () => {
    const service = new TokenService();
    const user = createMockUser();

    const tokens = service.generateTokens(user);
    const decoded = service.verifyAccessToken(tokens.accessToken);

    expect(decoded.userId).toBe(user.id);
    expect(decoded.tokenType).toBe("ACCESS_TOKEN");
  });
});

