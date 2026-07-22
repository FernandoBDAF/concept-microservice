import express from "express";
import { UserController } from "../controllers/UserController.js";
import { asyncHandler } from "../middleware/asyncHandler.js";
import { requiresAuth } from "../middleware/auth.middleware.js";
import { validate } from "../middleware/validation.middleware.js";
import {
  changeRoleSchema,
  createUserSchema,
  getUserByEmailSchema,
  getUserSchema,
  listUsersSchema,
  updateUserSchema,
} from "../../schemas/user.schema.js";

const router = express.Router();

router.get(
  "/me",
  requiresAuth(),
  asyncHandler(async (req, res) => {
    await UserController.getProfile(req, res);
  })
);

router.post(
  "/",
  validate(createUserSchema),
  asyncHandler(async (req, res) => {
    await UserController.createUser(req, res);
  })
);
router.get(
  "/",
  requiresAuth(["admin"]),
  validate(listUsersSchema),
  asyncHandler(async (req, res) => {
    await UserController.listUsers(req, res);
  })
);
router.get(
  "/:id",
  requiresAuth(["admin"]),
  validate(getUserSchema),
  asyncHandler(async (req, res) => {
    await UserController.getUser(req, res);
  })
);
router.get(
  "/email/:email",
  requiresAuth(["admin"]),
  validate(getUserByEmailSchema),
  asyncHandler(async (req, res) => {
    await UserController.getUserByEmail(req, res);
  })
);
router.put(
  "/:id",
  requiresAuth(["admin"]),
  validate(updateUserSchema),
  asyncHandler(async (req, res) => {
    await UserController.updateUser(req, res);
  })
);
router.delete(
  "/:id",
  requiresAuth(["admin"]),
  asyncHandler(async (req, res) => {
    await UserController.deleteUser(req, res);
  })
);

router.patch(
  "/:id/deactivate",
  requiresAuth(["admin"]),
  asyncHandler(async (req, res) => {
    await UserController.deactivateUser(req, res);
  })
);
router.patch(
  "/:id/activate",
  requiresAuth(["admin"]),
  asyncHandler(async (req, res) => {
    await UserController.activateUser(req, res);
  })
);
router.patch(
  "/:id/role",
  requiresAuth(["admin"]),
  validate(changeRoleSchema),
  asyncHandler(async (req, res) => {
    await UserController.changeUserRole(req, res);
  })
);

export default router;

