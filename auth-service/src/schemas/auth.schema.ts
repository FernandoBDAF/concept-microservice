import { z } from "zod";

export const loginSchema = z.object({
  body: z
    .object({
      email: z.string().email("Invalid email format").optional(),
      password: z.string().min(1, "Password is required"),
      user_id: z.string().email("Invalid email format").optional(),
    })
    .transform((data) => ({
      email: data.user_id ?? data.email ?? "",
      password: data.password,
    })),
});

export const refreshTokenSchema = z.object({
  body: z.object({
    refresh_token: z.string().min(1, "Refresh token is required"),
  }),
});

export const validateTokenSchema = z
  .object({
    body: z.object({
      token: z.string().optional(),
    }),
    headers: z.object({
      authorization: z.string().optional(),
    }),
  })
  .transform((data) => ({
    token: data.body.token ?? data.headers.authorization?.replace("Bearer ", ""),
  }))
  .refine((data) => data.token && data.token.length > 0, {
    message: "Token is required",
    path: ["token"],
  });

export type LoginInput = z.infer<typeof loginSchema>["body"];
export type RefreshTokenInput = z.infer<typeof refreshTokenSchema>["body"];

