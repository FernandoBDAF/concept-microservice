import logging
import threading

from flask import Flask, jsonify

logger = logging.getLogger(__name__)

app = Flask(__name__)

_ready = False
_healthy = True


def set_ready(ready: bool) -> None:
    global _ready
    _ready = ready


def set_healthy(healthy: bool) -> None:
    global _healthy
    _healthy = healthy


@app.route("/health")
def health():
    if _healthy:
        return jsonify({"status": "ok"}), 200
    return jsonify({"status": "unhealthy"}), 503


@app.route("/ready")
def ready():
    if _ready:
        return jsonify({"status": "ready"}), 200
    return jsonify({"status": "not ready"}), 503


@app.route("/live")
def live():
    return health()


def start_health_server(port: int = 8080, metrics_port: int = 8081) -> None:
    def run() -> None:
        import logging as flask_logging

        flask_logging.getLogger("werkzeug").setLevel(flask_logging.ERROR)
        app.run(host="0.0.0.0", port=port, debug=False, use_reloader=False)

    thread = threading.Thread(target=run, daemon=True)
    thread.start()
    logger.info("Health server started", extra={"port": port})

    from prometheus_client import start_http_server

    start_http_server(metrics_port)
    logger.info("Metrics server started", extra={"port": metrics_port})
