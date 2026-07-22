/**
 * Side-effect module that starts tracing before the rest of the app loads.
 *
 * MUST stay the first static import of src/server.ts: ESM evaluates the
 * import graph depth-first in declaration order, and the top-level await
 * below blocks sibling modules (app.ts -> express, pg, ...) from evaluating
 * until the SDK has started and registered its module-patching hooks.
 */
import { initTracing } from "./otel.js";

await initTracing();
