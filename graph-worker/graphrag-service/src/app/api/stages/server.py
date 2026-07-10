"""
Stages API HTTP Server

Simple HTTP server for testing the Stages API.
For production, integrate with existing API infrastructure.

Usage:
    python -m src.app.api.stages.server --port 8080
"""

# IMPORTANT: Load .env BEFORE any other imports that might use environment variables
import os

try:
    from dotenv import load_dotenv
    # Try multiple paths to find .env
    possible_paths = [
        os.path.join(os.path.dirname(__file__), '..', '..', '.env'),  # relative to this file
        os.path.join(os.getcwd(), '.env'),  # current working directory
        '.env',  # direct
    ]
    for env_path in possible_paths:
        if os.path.exists(env_path):
            load_dotenv(env_path, override=True)
            print(f"✓ Loaded environment from: {os.path.abspath(env_path)}")
            break
    else:
        print("⚠ No .env file found")
except ImportError:
    print("⚠ python-dotenv not installed, using shell environment")

# Now import everything else
import argparse
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse

from .api import handle_request

# Use the project's logging library for consistent, Loki-compatible logging
from src.lib.logging import setup_logging, get_logger, LokiFormatter

# Setup logging with JSON format for Loki, writing to a log file
log_dir = os.path.join(os.path.dirname(__file__), '..', '..', 'logs')
os.makedirs(log_dir, exist_ok=True)
log_file = os.path.join(log_dir, 'stages_api.log')

# Initialize logging using the project's logging library
setup_logging(
    level=20,  # INFO
    log_file=log_file,
    json_format=True,  # Use JSON format for Loki
    silence_third_party=True,
    rotate_logs=True,
    max_bytes=10_000_000,  # 10MB
    backup_count=5,
)

logger = get_logger(__name__)


class StagesAPIHandler(BaseHTTPRequestHandler):
    """HTTP request handler for Stages API"""

    def do_GET(self):
        """Handle GET requests"""
        # Handle Prometheus metrics endpoint (before API routing)
        if self.path in ["/metrics", "/api/v1/metrics/prometheus"]:
            try:
                from src.lib.metrics import export_prometheus_text
                metrics_text = export_prometheus_text()
                self._send_text(metrics_text)
            except Exception as e:
                logger.error(f"Error exporting metrics: {e}")
                self._send_text(f"# Error: {e}\n", 500)
            return

        parsed = urlparse(self.path)
        path = parsed.path.replace("/api/v1/", "").replace("/api/", "")

        result, status = handle_request("GET", path)
        self._send_json(result, status)

    def do_POST(self):
        """Handle POST requests"""
        parsed = urlparse(self.path)
        path = parsed.path.replace("/api/v1/", "").replace("/api/", "")

        # Read request body
        content_length = int(self.headers.get("Content-Length", 0))
        body = None
        if content_length > 0:
            body_bytes = self.rfile.read(content_length)
            try:
                body = json.loads(body_bytes.decode("utf-8"))
            except json.JSONDecodeError:
                self._send_json({"error": "Invalid JSON body"}, 400)
                return

        result, status = handle_request("POST", path, body)
        self._send_json(result, status)

    def do_HEAD(self):
        """Handle HEAD requests (same as GET but no body)"""
        parsed = urlparse(self.path)
        path = parsed.path.replace("/api/v1/", "").replace("/api/", "")

        result, status = handle_request("GET", path)
        self._send_headers(status, len(json.dumps(result, indent=2).encode()))

    def do_OPTIONS(self):
        """Handle CORS preflight requests"""
        self.send_response(200)
        self._add_cors_headers()
        self.end_headers()

    def _send_headers(self, status=200, content_length=0):
        """Send response headers only (for HEAD requests)"""
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(content_length))
        self._add_cors_headers()
        self.end_headers()

    def _send_json(self, data, status=200):
        """Send JSON response"""
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self._add_cors_headers()
        self.end_headers()
        self.wfile.write(json.dumps(data, indent=2).encode())

    def _send_text(self, text: str, status=200):
        """Send plain text response (for Prometheus metrics)"""
        self.send_response(status)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self._add_cors_headers()
        self.end_headers()
        self.wfile.write(text.encode())

    def _add_cors_headers(self):
        """Add CORS headers for browser access"""
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, HEAD, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")

    def log_message(self, format, *args):
        """Custom logging"""
        logger.info(f"{self.address_string()} - {format % args}")


def run_server(host: str = "0.0.0.0", port: int = 8080):
    """Run the HTTP server"""
    server = HTTPServer((host, port), StagesAPIHandler)
    logger.info(f"Stages API server running on http://{host}:{port}")
    logger.info("Endpoints:")
    logger.info("  GET  /api/v1/stages")
    logger.info("  GET  /api/v1/stages/{pipeline}")
    logger.info("  GET  /api/v1/stages/{stage_name}/config")
    logger.info("  GET  /api/v1/stages/{stage_name}/defaults")
    logger.info("  POST /api/v1/stages/{stage_name}/validate")
    logger.info("  POST /api/v1/pipelines/validate")
    logger.info("  POST /api/v1/pipelines/execute")
    logger.info("  GET  /api/v1/pipelines/{pipeline_id}/status")
    logger.info("  POST /api/v1/pipelines/{pipeline_id}/cancel")
    logger.info("  GET  /api/v1/pipelines/active")
    logger.info("  GET  /api/v1/pipelines/history")
    logger.info("  GET  /metrics (Prometheus format)")
    logger.info("  --- Management ---")
    logger.info("  GET  /api/v1/management/inspect-databases")
    logger.info("  GET  /api/v1/management/operations/{id}")
    logger.info("  POST /api/v1/management/copy-collection")
    logger.info("  POST /api/v1/management/clean-graphrag")
    logger.info("  POST /api/v1/management/clean-stage-status")
    logger.info("  POST /api/v1/management/setup-test-db")
    logger.info("  POST /api/v1/management/rebuild-indexes")
    logger.info("  --- Viewer ---")
    logger.info("  GET  /api/v1/viewer/databases")
    logger.info("  GET  /api/v1/viewer/collections/{db}")
    logger.info("  GET  /api/v1/viewer/document/{db}/{collection}/{id}")
    logger.info("  POST /api/v1/viewer/query")
    logger.info("  GET  /api/v1/viewer/schema/{db}/{collection}")
    logger.info("  --- Iteration ---")
    logger.info("  GET  /api/v1/viewer/compare/{db}/{coll}/{id1}/{id2}")
    logger.info("  GET  /api/v1/viewer/timeline/{db}/{coll}/{source_id}")
    logger.info("  POST /api/v1/viewer/suggest-rerun")
    logger.info("  GET  /api/v1/viewer/run-history/{db}/{coll}/{doc_id}")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logger.info("Server stopped")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Stages API Server")
    parser.add_argument("--host", default="0.0.0.0", help="Host to bind to")
    parser.add_argument("--port", type=int, default=8080, help="Port to listen on")
    args = parser.parse_args()

    run_server(args.host, args.port)
