/**
 * JwksService (ADR-009.1).
 *
 * auth-service signs with RS256 (kid header) and serves GET
 * /.well-known/jwks.json so api-service can verify locally with a cached JWKS
 * (introspection stays available behind a strict-mode flag).
 *
 *   - keypair source: PEM via env/secret (Secret auth-service-keys; compose
 *     .env) decoded by the config loader — see config/index.ts
 *   - kid = first 8 bytes of SHA-256 of the DER SPKI, hex
 *   - TokenService signs RS256 when a keypair is configured (HS256 stays the
 *     keyless fallback during migration, controlled by JWT_ALGORITHM)
 *   - JWKS shape: { keys: [{ kty:"RSA", use:"sig", alg:"RS256", kid, n, e }] }
 *   - rotation: publicKeys array, newest signs, all published until expiry
 *
 * No new npm deps are required (node:crypto covers RSA + JWK export). In HS256
 * fallback mode there are no public keys, so jwks() returns { keys: [] }.
 */
import { createPublicKey, createHash, type KeyObject } from "node:crypto";
import { config } from "../../config/index.js";

export interface JsonWebKey {
  kty: string;
  use: string;
  alg: string;
  kid: string;
  n: string;
  e: string;
}

export class JwksService {
  private readonly publicKeys: KeyObject[];

  constructor(publicKeyPems: readonly string[]) {
    this.publicKeys = publicKeyPems.map((pem) => createPublicKey(pem));
  }

  /** Stable key id derived from the SPKI DER (HANDOFF §A6). */
  static kidFor(key: KeyObject): string {
    const der = key.export({ type: "spki", format: "der" });
    return createHash("sha256").update(der).digest("hex").slice(0, 16);
  }

  /** The /.well-known/jwks.json document. Empty in HS256 fallback mode. */
  jwks(): { keys: JsonWebKey[] } {
    return {
      keys: this.publicKeys.map((key) => {
        const jwk = key.export({ format: "jwk" }) as { kty: string; n: string; e: string };
        return {
          kty: jwk.kty,
          use: "sig",
          alg: "RS256",
          kid: JwksService.kidFor(key),
          n: jwk.n,
          e: jwk.e,
        };
      }),
    };
  }
}

/** Singleton over the configured public key(s) (RS256 mode); empty in HS256. */
export const jwksService = new JwksService(config.jwt.publicKeys);
