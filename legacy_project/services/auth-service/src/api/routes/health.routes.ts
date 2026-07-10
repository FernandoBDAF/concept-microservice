import express from "express";
import { HealthController } from "../controllers/HealthController.js";

const router = express.Router();

router.get("/health", HealthController.health);
router.get("/ready", HealthController.ready);
router.get("/live", HealthController.live);

export default router;

