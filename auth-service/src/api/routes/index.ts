import type { Express } from "express";
import authRoutes from "./auth.routes.js";
import healthRoutes from "./health.routes.js";
import userRoutes from "./user.routes.js";

export const registerRoutes = (app: Express): void => {
  app.use(healthRoutes);
  app.use("/v1/auth", authRoutes);
  app.use("/v1/users", userRoutes);
};

