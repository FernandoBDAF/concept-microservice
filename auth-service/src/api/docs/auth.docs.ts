import { z } from "zod";
import { registry } from "./openapi.js";
import { loginSchema, refreshTokenSchema } from "../../schemas/auth.schema.js";

registry.registerPath({
  method: "post",
  path: "/v1/auth/login",
  tags: ["Authentication"],
  summary: "User login",
  description: "Authenticate user with email and password",
  request: {
    body: {
      content: {
        "application/json": {
          schema: loginSchema.shape.body,
        },
      },
    },
  },
  responses: {
    200: {
      description: "Login successful",
      content: {
        "application/json": {
          schema: z.object({
            status: z.literal("success"),
            message: z.string(),
            data: z.object({
              access_token: z.string(),
              refresh_token: z.string(),
              token_type: z.literal("bearer"),
              expires_in: z.number(),
              user: z.object({
                id: z.string().uuid(),
                email: z.string().email(),
                role: z.enum(["user", "admin"]),
                isActive: z.boolean(),
                createdAt: z.string().datetime(),
                updatedAt: z.string().datetime(),
              }),
            }),
          }),
        },
      },
    },
    401: { description: "Invalid credentials" },
    423: { description: "Account locked" },
  },
});

registry.registerPath({
  method: "post",
  path: "/v1/auth/token/refresh",
  tags: ["Authentication"],
  summary: "Refresh access token",
  request: {
    body: {
      content: {
        "application/json": {
          schema: refreshTokenSchema.shape.body,
        },
      },
    },
  },
  responses: {
    200: { description: "Token refreshed successfully" },
    401: { description: "Invalid refresh token" },
  },
});

registry.registerPath({
  method: "post",
  path: "/v1/auth/token/validate",
  tags: ["Authentication"],
  summary: "Validate access token",
  security: [{ bearerAuth: [] }],
  responses: {
    200: { description: "Token is valid" },
    401: { description: "Token is invalid or expired" },
  },
});

