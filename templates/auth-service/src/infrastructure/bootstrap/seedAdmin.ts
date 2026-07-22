import { config } from "../../config/index.js";
import { userRepository } from "../../domain/repositories/UserRepository.js";
import { logger } from "../logging/logger.js";

const log = logger.child({ bootstrap: "seedAdmin" });

/**
 * Env-driven admin bootstrap (ADR-009.7). When SEED_ADMIN_EMAIL and
 * SEED_ADMIN_PASSWORD are both set and the user is absent, create it with role
 * "admin". Idempotent: a no-op when the user already exists or the vars are
 * unset. Never throws — a failed seed must not block startup.
 */
export async function seedAdmin(): Promise<void> {
  const { email, password } = config.seedAdmin;
  if (!email || !password) return;

  try {
    const existing = await userRepository.findByEmail(email);
    if (existing) {
      log.info({ email }, "Admin user already present; skipping seed");
      return;
    }
    const user = await userRepository.create({ email, password, role: "admin" });
    log.info({ email, userId: user.id }, "Seeded admin user");
  } catch (error) {
    // A unique-violation race (two instances seeding at once) is benign; any
    // other error is logged but must not abort startup.
    log.error({ err: error, email }, "Admin seed failed");
  }
}
