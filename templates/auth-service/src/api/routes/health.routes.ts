import express from "express";
import { HealthController } from "../controllers/HealthController.js";
import { asyncHandler } from "../middleware/asyncHandler.js";

const router = express.Router();

router.get(
  "/health",
  asyncHandler(async (req, res) => {
    await HealthController.health(req, res);
  })
);
router.get(
  "/ready",
  asyncHandler(async (req, res) => {
    await HealthController.ready(req, res);
  })
);
router.get(
  "/live",
  asyncHandler((req, res) => {
    HealthController.live(req, res);
  })
);
router.get(
  "/metrics",
  asyncHandler(async (req, res) => {
    await HealthController.metrics(req, res);
  })
);

export default router;

