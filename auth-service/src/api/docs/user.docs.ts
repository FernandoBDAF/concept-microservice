import { z } from "zod";
import { registry } from "./openapi.js";
import { createUserSchema, updateUserSchema } from "../../schemas/user.schema.js";

const safeUserSchema = z.object({
  id: z.string().uuid(),
  email: z.string().email(),
  role: z.enum(["user", "admin"]),
  isActive: z.boolean(),
  createdAt: z.string().datetime(),
  updatedAt: z.string().datetime(),
});

registry.registerPath({
  method: "post",
  path: "/v1/users",
  tags: ["Users"],
  summary: "Create user",
  request: {
    body: {
      content: {
        "application/json": {
          schema: createUserSchema.shape.body,
        },
      },
    },
  },
  responses: {
    201: {
      description: "User created",
      content: {
        "application/json": {
          schema: z.object({
            status: z.literal("success"),
            message: z.string(),
            data: z.object({
              user: safeUserSchema,
            }),
          }),
        },
      },
    },
    400: { description: "Validation error" },
  },
});

registry.registerPath({
  method: "get",
  path: "/v1/users/me",
  tags: ["Users"],
  summary: "Get current user profile",
  security: [{ bearerAuth: [] }],
  responses: {
    200: {
      description: "User profile",
      content: {
        "application/json": {
          schema: z.object({
            status: z.literal("success"),
            message: z.string(),
            data: z.object({
              user: safeUserSchema,
            }),
          }),
        },
      },
    },
    401: { description: "Unauthorized" },
  },
});

registry.registerPath({
  method: "put",
  path: "/v1/users/{id}",
  tags: ["Users"],
  summary: "Update user",
  security: [{ bearerAuth: [] }],
  request: {
    params: updateUserSchema.shape.params,
    body: {
      content: {
        "application/json": {
          schema: updateUserSchema.shape.body,
        },
      },
    },
  },
  responses: {
    200: { description: "User updated" },
    404: { description: "User not found" },
  },
});

