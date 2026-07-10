import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";
import { textSummary } from "k6/metrics";

// Custom metrics
const authErrorRate = new Rate("auth_errors");
const requestCounter = new Counter("total_requests");
const validationErrorRate = new Rate("validation_errors");
const connectionErrorRate = new Rate("connection_errors");
const duplicateEmailRate = new Rate("duplicate_email_errors");
const requestBodyErrorRate = new Rate("request_body_errors");
const timeoutErrorRate = new Rate("timeout_errors");
const rateLimitErrorRate = new Rate("rate_limit_errors");

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
    { duration: "2m", target: 100 }, // Ramp up to 100 users
    { duration: "5m", target: 100 }, // Stay at 100 users
    { duration: "2m", target: 200 }, // Ramp up to 200 users
    { duration: "5m", target: 200 }, // Stay at 200 users
    { duration: "2m", target: 300 }, // Ramp up to 300 users
    { duration: "5m", target: 300 }, // Stay at 300 users
    { duration: "2m", target: 0 }, // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ["p(95)<2000"], // 95% of requests should be below 2s
    http_req_failed: ["rate<0.2"], // Less than 20% errors
    validation_errors: ["rate<0.1"], // Less than 10% validation errors
    connection_errors: ["rate<0.1"], // Less than 10% connection errors
    timeout_errors: ["rate<0.1"], // Less than 10% timeout errors
    rate_limit_errors: ["rate<0.1"], // Less than 10% rate limit errors
  },
};

// Test data
const BASE_URL =
  __ENV.BASE_URL || "http://profile-api.default.svc.cluster.local/api/v1";
const AUTH_URL =
  __ENV.AUTH_URL || "http://auth-api.default.svc.cluster.local/api/v1";
const TEST_USER = {
  user_id: "FB",
  password: "FB.com",
};

// Helper function to get authentication token
function getAuthToken() {
  const loginPayload = JSON.stringify({
    user_id: TEST_USER.user_id,
    password: TEST_USER.password,
  });

  const loginResponse = http.post(`${AUTH_URL}/auth/token`, loginPayload, {
    headers: { "Content-Type": "application/json" },
    timeout: "30s", // Increased timeout for stress test
  });

  authTrend.add(loginResponse.timings.duration);
  responseSizeTrend.add(loginResponse.body.length);
  requestCounter.add(1);

  const checks = check(loginResponse, {
    "login status is 200": (r) => r.status === 200,
    "login returns token": (r) => {
      const data = r.json();
      return data && data.token !== undefined;
    },
  });

  if (!checks) {
    authErrorRate.add(1);
    console.error("Auth failed:", {
      status: loginResponse.status,
      body: loginResponse.body,
    });
    return null;
  }

  return loginResponse.json().token;
}

// Helper function to generate test data
function generateTestData() {
  const timestamp = Date.now();
  const random = Math.floor(Math.random() * 1000);

  return {
    first_name: `Test${random}`,
    last_name: `User${random}`,
    email: `test${timestamp}${random}@example.com`,
    phone: `+1${Math.floor(Math.random() * 10000000000)
      .toString()
      .padStart(10, "0")}`,
    bio: `Test bio for user ${random}`,
    image_urls: [`https://example.com/images/test${random}.jpg`],
    address: {
      street: "123 Test St",
      city: "Test City",
      state: "TS",
      country: "Test Country",
      zip_code: "12345",
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
  } else if (response.status === 429) {
    rateLimitErrorRate.add(1);
  } else if (response.status >= 500) {
    connectionErrorRate.add(1);
  }

  if (response.timings.duration > 10000) {
    // 10s timeout
    timeoutErrorRate.add(1);
  }

  console.error(`${operation} failed:`, {
    status: response.status,
    body: response.body,
    duration: response.timings.duration,
  });
}

// Main test function
export default function () {
  // Get authentication token
  const token = getAuthToken();
  if (!token) {
    console.error("Failed to get authentication token");
    return;
  }

  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };

  // Randomly choose operation type
  const operation = Math.random();

  if (operation < 0.3) {
    // 30% chance - List Profiles
    const listResponse = http.get(`${BASE_URL}/profiles`, {
      headers,
      timeout: "30s",
    });
    listTrend.add(listResponse.timings.duration);

    const listChecks = check(listResponse, {
      "list profiles status is 200": (r) => r.status === 200,
      "list profiles has data": (r) => {
        const data = r.json();
        return Array.isArray(data);
      },
    });

    if (!listChecks) {
      handleError(listResponse, "List profiles");
    }
  } else if (operation < 0.6) {
    // 30% chance - Create Profile
    const testData = generateTestData();
    const createPayload = JSON.stringify(testData);

    const createResponse = http.post(`${BASE_URL}/profiles`, createPayload, {
      headers,
      timeout: "30s",
    });
    createTrend.add(createResponse.timings.duration);

    const createChecks = check(createResponse, {
      "create profile status is 201": (r) => r.status === 201,
      "create profile has id": (r) => {
        const data = r.json();
        return data && data.profile && data.profile.id !== undefined;
      },
    });

    if (!createChecks) {
      handleError(createResponse, "Create profile");
      return;
    }

    // Save the created profile ID for potential update/delete
    const createdProfile = createResponse.json();
    const profileId = createdProfile.profile.id;

    // 50% chance to update the profile immediately
    if (Math.random() < 0.5) {
      const updateData = generateTestData();
      const updatePayload = JSON.stringify(updateData);

      const updateResponse = http.put(
        `${BASE_URL}/profiles/${profileId}`,
        updatePayload,
        {
          headers,
          timeout: "30s",
        }
      );
      updateTrend.add(updateResponse.timings.duration);

      const updateChecks = check(updateResponse, {
        "update profile status is 200": (r) => r.status === 200,
        "update profile has updated data": (r) => {
          const data = r.json();
          return (
            data &&
            data.profile &&
            data.profile.first_name === updateData.first_name
          );
        },
      });

      if (!updateChecks) {
        handleError(updateResponse, "Update profile");
      }
    }

    // 30% chance to delete the profile
    if (Math.random() < 0.3) {
      const deleteResponse = http.del(
        `${BASE_URL}/profiles/${profileId}`,
        null,
        {
          headers,
          timeout: "30s",
        }
      );
      deleteTrend.add(deleteResponse.timings.duration);

      const deleteChecks = check(deleteResponse, {
        "delete profile status is 204": (r) => r.status === 204,
      });

      if (!deleteChecks) {
        handleError(deleteResponse, "Delete profile");
      }
    }
  } else {
    // 40% chance - Get Profile
    const profileId = `profile-${Math.floor(Math.random() * 1000)}`;
    const getResponse = http.get(`${BASE_URL}/profiles/${profileId}`, {
      headers,
      timeout: "30s",
    });

    const getChecks = check(getResponse, {
      "get profile status is 200": (r) => r.status === 200,
      "get profile has data": (r) => {
        const data = r.json();
        return data && data.profile && data.profile.id !== undefined;
      },
    });

    if (!getChecks) {
      handleError(getResponse, "Get profile");
    }
  }

  // Add a small sleep between requests to prevent overwhelming the system
  sleep(1);
}

// Summary handler
export function handleSummary(data) {
  return {
    stdout: textSummary(data, { indent: " ", enableColors: true }),
    "summary.json": JSON.stringify(data),
  };
}
