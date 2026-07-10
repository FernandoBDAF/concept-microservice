import express from "express";
import UserController from "../controllers/UserController.js";
import authenticationService from "../service/authenticationService.js";

const router = express.Router();

// Simple auth middleware
const requiresAuth = (roles = []) => {
  return async (req, res, next) => {
    try {
      const token = req.headers.authorization?.split(" ")[1];

      if (!token) {
        return res.status(401).json({
          status: "error",
          message: "Authorization token required",
        });
      }

      const validation = await authenticationService.validateToken(token);

      if (!validation.valid) {
        return res.status(401).json({
          status: "error",
          message: "Invalid token",
        });
      }

      // Check role if required
      if (roles.length > 0 && !roles.includes(validation.user.role)) {
        return res.status(403).json({
          status: "error",
          message: "Access denied",
        });
      }

      req.user = validation.user;
      next();
    } catch (error) {
      res.status(401).json({
        status: "error",
        message: "Authentication failed",
      });
    }
  };
};

// User profile endpoints (compatible with existing API)
router.get("/me", requiresAuth(), async (req, res) => {
  try {
    const user = req.user; // Set by auth middleware

    res.json({
      status: "success",
      message: "User profile retrieved",
      data: {
        user: {
          id: user.id,
          email: user.email,
          role: user.role,
          isActive: user.isActive,
          createdAt: user.createdAt,
          updatedAt: user.updatedAt,
        },
      },
    });
  } catch (error) {
    res.status(500).json({
      status: "error",
      message: "Failed to retrieve user profile",
    });
  }
});

// Admin-only user management endpoints
router.post("/", 
  // requiresAuth(["admin"]), 
  UserController.createUser);
router.get("/", requiresAuth(["admin"]), UserController.listUsers);
router.get("/:id", requiresAuth(["admin"]), UserController.getUser);
router.get(
  "/email/:email",
  requiresAuth(["admin"]),
  UserController.getUserByEmail
);
router.put("/:id", requiresAuth(["admin"]), UserController.updateUser);
router.delete("/:id", requiresAuth(["admin"]), UserController.deleteUser);

// User status management (admin only)
router.patch(
  "/:id/deactivate",
  requiresAuth(["admin"]),
  UserController.deactivateUser
);
router.patch(
  "/:id/activate",
  requiresAuth(["admin"]),
  UserController.activateUser
);
router.patch(
  "/:id/role",
  requiresAuth(["admin"]),
  UserController.changeUserRole
);

export default router;
