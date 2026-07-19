/**
 * OpenTelemetry tracing bootstrap.
 *
 * Enabled ONLY when OTEL_EXPORTER_OTLP_ENDPOINT is set (same gate as the Go
 * services); otherwise initTracing() is a no-op returning null. Spans are
 * exported over OTLP HTTP; the exporter reads OTEL_EXPORTER_OTLP_ENDPOINT
 * itself (appending /v1/traces). Service name defaults to "auth-service" and
 * can be overridden with OTEL_SERVICE_NAME. NodeSDK's default propagators
 * include W3C tracecontext (+ baggage).
 *
 * This module deliberately reads process.env directly instead of the zod
 * config: it must be evaluated (via ./register.js) before any other module in
 * the app -- including src/config -- so that the instrumentation hooks are
 * registered before express/pg/http are first imported.
 *
 * ESM caveat: this codebase is ESM ("type": "module"). express and pg are
 * CommonJS packages, so as long as the SDK starts before they are imported
 * (guaranteed by ./register.js being the first static import of server.ts,
 * with a top-level await), require-in-the-middle patches them and the http
 * builtin normally. Full auto-patching of *pure ESM* packages would need the
 * import-in-the-middle --import hook; none of the instrumented targets here
 * need it.
 */
import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { HttpInstrumentation } from "@opentelemetry/instrumentation-http";
import { ExpressInstrumentation } from "@opentelemetry/instrumentation-express";
import { PgInstrumentation } from "@opentelemetry/instrumentation-pg";

let sdk: NodeSDK | null = null;

export async function initTracing(): Promise<NodeSDK | null> {
  if (sdk) {
    return sdk;
  }

  if (!process.env.OTEL_EXPORTER_OTLP_ENDPOINT) {
    return null;
  }

  sdk = new NodeSDK({
    serviceName: process.env.OTEL_SERVICE_NAME ?? "auth-service",
    traceExporter: new OTLPTraceExporter(),
    instrumentations: [
      new HttpInstrumentation(),
      new ExpressInstrumentation(),
      new PgInstrumentation(),
    ],
  });

  sdk.start();
  return sdk;
}

export async function shutdownTracing(): Promise<void> {
  if (!sdk) {
    return;
  }

  const current = sdk;
  sdk = null;
  await current.shutdown();
}
