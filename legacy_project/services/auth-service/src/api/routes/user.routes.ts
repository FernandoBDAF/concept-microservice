import express from "express";
import { UserController } from "../controllers/UserController.js";
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

router.get("/me", requiresAuth(), UserController.getProfile);

router.post("/", validate(createUserSchema), UserController.createUser);
router.get("/", requiresAuth(["admin"]), validate(listUsersSchema), UserController.listUsers);
router.get("/:id", requiresAuth(["admin"]), validate(getUserSchema), UserController.getUser);
router.get(
  "/email/:email",
  requiresAuth(["admin"]),
  validate(getUserByEmailSchema),
  UserController.getUserByEmail
);
router.put(
  "/:id",
  requiresAuth(["admin"]),
  validate(updateUserSchema),
  UserController.updateUser
);
router.delete("/:id", requiresAuth(["admin"]), UserController.deleteUser);

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
  validate(changeRoleSchema),
  UserController.changeUserRole
);

export default router;

