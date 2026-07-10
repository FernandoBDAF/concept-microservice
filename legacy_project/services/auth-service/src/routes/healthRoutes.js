import express from "express";
import healthService from "../service/healthService.js";

const router = express.Router();

// GET /health - Comprehensive health check with dependencies
router.get("/health", async (req, res) => {
  try {
    const health = await healthService.checkHealth();
    const statusCode = health.status === "healthy" ? 200 : 503;
    res.status(statusCode).json(health);
  } catch (error) {
    res.status(503).json({
      status: "unhealthy",
      timestamp: new Date().toISOString(),
      error: error.message,
    });
  }
});

// GET /ready - Readiness probe for Kubernetes
router.get("/ready", async (req, res) => {
  try {
    const readiness = await healthService.checkReadiness();
    const statusCode = readiness.status === "ready" ? 200 : 503;
    res.status(statusCode).json(readiness);
  } catch (error) {
    res.status(503).json({
      status: "not ready",
      timestamp: new Date().toISOString(),
      error: error.message,
    });
  }
});

// GET /live - Liveness probe for Kubernetes
router.get("/live", (req, res) => {
  const liveness = healthService.checkLiveness();
  res.status(200).json(liveness);
});

export default router;
