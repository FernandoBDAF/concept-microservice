#!/usr/bin/env bash
# Worker-outage drill: stop a worker, build a backlog, restart, watch it drain.
# Usage: scripts/simulate/worker-outage.sh [email|image|profile] [message-count]
# Watch on Grafana (localhost:3001): queue depth rises, then drains; worker
# throughput spikes on recovery.
set -euo pipefail
cd "$(dirname "$0")/../.."

WORKER="${1:-email}"
COUNT="${2:-100}"
case "$WORKER" in
  email)   RK="email.send";     QUEUE="email-processing" ;;
  image)   RK="image.process";  QUEUE="image-processing" ;;
  profile) RK="profile.task";   QUEUE="profile-processing" ;;
  *) echo "unknown worker: $WORKER (use email|image|profile)"; exit 1 ;;
esac

queue_depth() {
  docker compose exec -T rabbitmq rabbitmqctl list_queues name messages 2>/dev/null \
    | awk -v q="$QUEUE" '$1==q {print $2}'
}

echo "== 1/4 stopping ${WORKER}-worker"
docker compose stop "${WORKER}-worker"

echo "== 2/4 flooding ${COUNT} ${RK} messages"
python3 scripts/simulate/publish.py flood --routing-key "$RK" --count "$COUNT"
echo "   backlog in ${QUEUE}: $(queue_depth) messages"

echo "== 3/4 restarting ${WORKER}-worker"
docker compose start "${WORKER}-worker"

echo "== 4/4 draining (checking every 5s, up to 2m)"
for _ in $(seq 1 24); do
  sleep 5
  depth="$(queue_depth)"
  echo "   ${QUEUE}: ${depth:-?} messages"
  [ "${depth:-1}" = "0" ] && { echo "drained ✔"; exit 0; }
done
echo "did not fully drain in 2m — inspect 'make queues' and worker logs"
exit 1
