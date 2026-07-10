import { z } from "zod";

const userRoleSchema = z.enum(["user", "admin"]);

export const createUserSchema = z.object({
  body: z.object({
    email: z.string().email("Invalid email format"),
    password: z.string().min(8, "Password must be at least 8 characters"),
    role: userRoleSchema.optional().default("user"),
  }),
});

export const updateUserSchema = z.object({
  params: z.object({
    id: z.string().uuid("Invalid user ID"),
  }),
  body: z.object({
    email: z.string().email("Invalid email format").optional(),
    password: z
      .string()
      .min(8, "Password must be at least 8 characters")
      .optional(),
    role: userRoleSchema.optional(),
    isActive: z.boolean().optional(),
  }),
});

export const getUserSchema = z.object({
  params: z.object({
    id: z.string().uuid("Invalid user ID"),
  }),
});

export const getUserByEmailSchema = z.object({
  params: z.object({
    email: z.string().email("Invalid email format"),
  }),
});

export const listUsersSchema = z.object({
  query: z.object({
    page: z.coerce.number().int().positive().optional().default(1),
    pageSize: z.coerce.number().int().min(1).max(100).optional().default(10),
  }),
});

export const changeRoleSchema = z.object({
  params: z.object({
    id: z.string().uuid("Invalid user ID"),
  }),
  body: z.object({
    role: userRoleSchema,
  }),
});

export type CreateUserInput = z.infer<typeof createUserSchema>["body"];
export type UpdateUserInput = z.infer<typeof updateUserSchema>["body"];
export type ListUsersInput = z.infer<typeof listUsersSchema>["query"];

