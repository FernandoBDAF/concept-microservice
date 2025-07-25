import express from "express";
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

// GET /v1/users/me - Get current user profile (auth-service-old compatible)
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
          firstName: user.firstName,
          lastName: user.lastName,
          role: user.role,
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

// GET /v1/users/{id} - Get user by ID (requires admin role)
router.get("/:id", requiresAuth(["admin"]), async (req, res) => {
  try {
    const { id } = req.params;

    // This would require extending storage-service client
    // For now, return current user if requesting own profile
    if (id === req.user.id) {
      return res.json({
        status: "success",
        message: "User profile retrieved",
        data: {
          user: req.user,
        },
      });
    }

    res.status(403).json({
      status: "error",
      message: "Access denied",
    });
  } catch (error) {
    res.status(500).json({
      status: "error",
      message: "Failed to retrieve user profile",
    });
  }
});

export default router;
