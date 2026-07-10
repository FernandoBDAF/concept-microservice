// k6 load simulation for the API path (auth → api-service → RabbitMQ).
// Run via the root Makefile: `make sim-smoke` / `sim-load` / `sim-burst`,
// or directly:
//   docker run --rm -i --network microservices_default \
//     -e API_URL=http://api-service:8080 -e AUTH_URL=http://auth-service:3000 \
//     grafana/k6 run - < scripts/simulate/api-load.js
// Watch it live on Grafana (localhost:3001) → "Lab Overview".

import http from "k6/http";
import { check, sleep } from "k6";

const API = __ENV.API_URL || "http://api-service:8080";
const AUTH = __ENV.AUTH_URL || "http://auth-service:3000";
const PASSWORD = "Sim-load-pass-123!";

export const options = {
  vus: Number(__ENV.SIM_VUS || 5),
  duration: __ENV.SIM_DURATION || "1m",
  thresholds: {
    http_req_failed: ["rate<0.05"],
    http_req_duration: ["p(95)<800"],
  },
};

export function setup() {
  const email = `sim-${Date.now()}@lab.dev`;
  const params = { headers: { "Content-Type": "application/json" } };

  const reg = http.post(
    `${AUTH}/v1/users`,
    JSON.stringify({ email, password: PASSWORD }),
    params
  );
  check(reg, { "user created": (r) => r.status === 201 || r.status === 200 });

  const login = http.post(
    `${AUTH}/v1/auth/login`,
    JSON.stringify({ email, password: PASSWORD }),
    params
  );
  check(login, { "login ok": (r) => r.status === 200 });

  const token = login.json("data.access_token");
  if (!token) throw new Error(`no access_token in login response: ${login.body}`);
  return { token };
}

export default function (data) {
  const params = {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${data.token}`,
    },
  };

  // Create a profile
  const create = http.post(
    `${API}/api/v1/profiles`,
    JSON.stringify({
      first_name: "Sim",
      last_name: `VU${__VU}`,
      email: `sim-${__VU}-${__ITER}-${Date.now()}@lab.dev`,
      bio: "k6 simulation",
    }),
    params
  );
  check(create, { "profile created": (r) => r.status === 200 || r.status === 201 });
  const profileId = create.json("id");

  // Read it back + list a page
  if (profileId) {
    const get = http.get(`${API}/api/v1/profiles/${profileId}`, params);
    check(get, { "profile fetched": (r) => r.status === 200 });
  }
  const list = http.get(`${API}/api/v1/profiles?page=1&page_size=10`, params);
  check(list, { "profiles listed": (r) => r.status === 200 });

  // Feed the async pipelines (~50% of iterations)
  if (profileId && __ITER % 2 === 0) {
    const email = http.post(
      `${API}/api/v1/profiles/${profileId}/tasks/email`,
      JSON.stringify({
        email_type: "welcome",
        recipient: `sim-${__VU}@lab.dev`,
        subject: "Simulated welcome",
        template_id: "welcome-template",
        variables: { first_name: "Sim" },
      }),
      params
    );
    check(email, { "email task accepted": (r) => r.status === 200 || r.status === 202 });

    const task = http.post(
      `${API}/api/v1/profiles/${profileId}/tasks/profile`,
      JSON.stringify({
        task_type: "sync",
        profile_id: profileId,
        user_id: `vu-${__VU}`,
        data: { source: "k6" },
      }),
      params
    );
    check(task, { "profile task accepted": (r) => r.status === 200 || r.status === 202 });
  }

  sleep(1);
}
