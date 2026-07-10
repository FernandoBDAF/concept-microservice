import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";
import { textSummary } from "k6/metrics";
import { randomString } from "https://jslib.k6.io/k6-utils/1.2.0/index.js";

// Custom metrics
const authErrorRate = new Rate("auth_errors");
const requestCounter = new Counter("total_requests");
const validationErrorRate = new Rate("validation_errors");
const connectionErrorRate = new Rate("connection_errors");
const duplicateEmailRate = new Rate("duplicate_email_errors");
const requestBodyErrorRate = new Rate("request_body_errors");

// Duration trends for each operation
const authTrend = new Trend("auth_duration");
const createTrend = new Trend("create_duration");
const updateTrend = new Trend("update_duration");
const deleteTrend = new Trend("delete_duration");
const listTrend = new Trend("list_duration");

// Response size metrics
const responseSizeTrend = new Trend("response_size");

// Test configuration
export const options = {
  stages: [
    { duration: "1m", target: 50 }, // Ramp up to 50 users
    { duration: "3m", target: 50 }, // Stay at 50 users
    { duration: "1m", target: 100 }, // Ramp up to 100 users
    { duration: "3m", target: 100 }, // Stay at 100 users
    { duration: "1m", target: 0 }, // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ["p(95)<500"], // 95% of requests should be below 500ms
    http_req_failed: ["rate<0.1"], // Less than 10% of requests should fail
    validation_errors: ["rate<0.01"], // Less than 1% validation errors
    connection_errors: ["rate<0.01"], // Less than 1% connection errors
    duplicate_email_errors: ["rate<0.01"], // Less than 1% duplicate email errors
    request_body_errors: ["rate<0.01"], // Less than 1% request body errors
  },
};

// Test data
const BASE_URL = __ENV.API_URL || "http://profile-api:80/api/v1";

// Helper function to generate valid profile data
function generateProfileData() {
  // Generate a random number of digits between 10 and 15
  const numDigits = Math.floor(Math.random() * 6) + 10; // Random number between 10 and 15
  // Generate a random number with exactly numDigits digits
  const phoneNumber = Math.floor(Math.random() * Math.pow(10, numDigits))
    .toString()
    .padStart(numDigits, "0");

  return {
    first_name: randomString(8),
    last_name: randomString(8),
    email: `${randomString(8)}@example.com`,
    phone: `${8888888888}`,
    bio: `Test bio for ${randomString(8)}`,
    image_urls: [`https://example.com/${randomString(8)}.jpg`],
    address: {
      street: `${Math.floor(Math.random() * 1000)} Main St`,
      city: "Test City",
      state: "TS",
      zip_code: `${Math.floor(Math.random() * 100000)}`,
      country: "US",
    },
  };
}

// Helper function to handle errors
function handleError(response, operation) {
  if (response.status === 400) {
    validationErrorRate.add(1);
  } else if (response.status === 409) {
    duplicateEmailRate.add(1);
  } else if (response.status === 413) {
    requestBodyErrorRate.add(1);
  } else if (response.status >= 500) {
    connectionErrorRate.add(1);
  }

  console.error(`${operation} failed:`, {
    status: response.status,
    body: response.body,
    error: response.error,
  });
}

export default function () {
  // Get auth token first
  const authRes = http.post(
    `${BASE_URL}/auth/token`,
    JSON.stringify({
      user_id: "FB",
      password: "FB.com",
    }),
    {
      headers: { "Content-Type": "application/json" },
    }
  );

  authTrend.add(authRes.timings.duration);
  responseSizeTrend.add(authRes.body.length);
  requestCounter.add(1);

  console.log("Auth response:", {
    status: authRes.status,
    body: authRes.body,
    error: authRes.error,
  });

  const authChecks = check(authRes, {
    "auth status is 200": (r) => r.status === 200,
    "auth returns token": (r) => r.json("token") !== undefined,
  });

  if (!authChecks) {
    authErrorRate.add(1);
    console.error("Failed to get auth token");
    return;
  }

  const token = authRes.json("token");
  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };

  // Randomly choose operation type
  const operation = Math.random();

  if (operation < 0.3) {
    // 30% chance - List Profiles
    const listRes = http.get(`${BASE_URL}/profiles`, { headers });
    listTrend.add(listRes.timings.duration);

    console.log("List response:", {
      status: listRes.status,
      body: listRes.body,
      error: listRes.error,
    });

    const listChecks = check(listRes, {
      "list status is 200": (r) => r.status === 200,
      "list returns array": (r) => Array.isArray(r.json()),
    });

    if (!listChecks) {
      handleError(listRes, "List profiles");
    }
  } else if (operation < 0.6) {
    // 30% chance - Create Profile
    const createPayload = generateProfileData();
    console.log("Create payload:", JSON.stringify(createPayload));

    const createRes = http.post(
      `${BASE_URL}/profiles`,
      JSON.stringify(createPayload),
      { headers }
    );
    createTrend.add(createRes.timings.duration);

    console.log("Create response:", {
      status: createRes.status,
      body: createRes.body,
      error: createRes.error,
    });

    const createChecks = check(createRes, {
      "create status is 201": (r) => r.status === 201,
      "create has profile id": (r) => r.json("profile.id") !== undefined,
    });

    if (!createChecks) {
      handleError(createRes, "Create profile");
    } else {
      const profileId = createRes.json("profile.id");
      sleep(1);

      // Get profile
      const getRes = http.get(`${BASE_URL}/profiles/${profileId}`, { headers });
      console.log("Get response:", {
        status: getRes.status,
        body: getRes.body,
        error: getRes.error,
      });

      check(getRes, {
        "get status is 200": (r) => r.status === 200,
        "get returns correct profile": (r) =>
          r.json("profile.id") === profileId,
      });
      sleep(1);

      // 50% chance to update the profile
      if (Math.random() < 0.5) {
        const updatePayload = generateProfileData();
        console.log("Update payload:", JSON.stringify(updatePayload));

        const updateRes = http.put(
          `${BASE_URL}/profiles/${profileId}`,
          JSON.stringify(updatePayload),
          { headers }
        );
        updateTrend.add(updateRes.timings.duration);

        console.log("Update response:", {
          status: updateRes.status,
          body: updateRes.body,
          error: updateRes.error,
        });

        const updateChecks = check(updateRes, {
          "update status is 200": (r) => r.status === 200,
          "update returns updated profile": (r) =>
            r.json("profile.id") === profileId,
        });

        if (!updateChecks) {
          handleError(updateRes, "Update profile");
        }
      }

      // 30% chance to delete the profile
      if (Math.random() < 0.3) {
        const deleteRes = http.del(`${BASE_URL}/profiles/${profileId}`, null, {
          headers,
        });
        deleteTrend.add(deleteRes.timings.duration);

        console.log("Delete response:", {
          status: deleteRes.status,
          body: deleteRes.body,
          error: deleteRes.error,
        });

        const deleteChecks = check(deleteRes, {
          "delete status is 204": (r) => r.status === 204,
        });

        if (!deleteChecks) {
          handleError(deleteRes, "Delete profile");
        }
      }
    }
  } else {
    // 40% chance - Get Profile
    // Generate a random UUID for the profile ID
    const randomId = "00000000-0000-0000-0000-000000000000".replace(/0/g, () =>
      Math.floor(Math.random() * 16).toString(16)
    );

    const getRes = http.get(`${BASE_URL}/profiles/${randomId}`, { headers });

    console.log("Get response:", {
      status: getRes.status,
      body: getRes.body,
      error: getRes.error,
    });

    const getChecks = check(getRes, {
      "get status is 200 or 404": (r) => r.status === 200 || r.status === 404,
    });

    if (!getChecks) {
      handleError(getRes, "Get profile");
    }
  }

  // Random sleep between 0.1 and 2 seconds
  sleep(Math.random() * 1.9 + 0.1);
}

// Summary handler
export function handleSummary(data) {
  return {
    stdout: textSummary(data, { indent: " ", enableColors: true }),
    "summary.json": JSON.stringify(data),
  };
}
