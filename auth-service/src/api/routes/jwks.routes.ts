import express from "express";
import { jwksService } from "../../domain/services/JwksService.js";

const router = express.Router();

// Public JWKS document (ADR-009.1): no auth, no rate limit. api-service fetches
// and caches this to verify RS256 access tokens locally. Empty `keys` in the
// HS256 keyless fallback. Mounted ahead of the rate limiter in app.ts.
router.get("/.well-known/jwks.json", (_req, res) => {
  res.json(jwksService.jwks());
});

export default router;
