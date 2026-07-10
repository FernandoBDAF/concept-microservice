import type { AuthenticatedUser } from "./auth.types.ts";

declare global {
  namespace Express {
    interface Request {
      id: string;
      user?: AuthenticatedUser;
      startTime?: number;
    }
  }
}

export {};

