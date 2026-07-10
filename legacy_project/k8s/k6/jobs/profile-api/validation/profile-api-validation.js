import http from "k6/http";
import { check, sleep } from "k6";
import { randomString } from "https://jslib.k6.io/k6-utils/1.2.0/index.js";

const BASE_URL = __ENV.API_URL || "http://profile-api:80/api/v1";

// Test configuration
export const options = {
  vus: 5,
  duration: "30s",
  thresholds: {
    http_req_duration: ["p(95)<500"], // 95% of requests should be below 500ms
    http_req_failed: ["rate<0.1"], // Less than 10% of requests should fail
  },
};

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

  console.log("Auth response:", {
    status: authRes.status,
    body: authRes.body,
    error: authRes.error,
  });

  check(authRes, {
    "auth status is 200": (r) => r.status === 200,
    "auth returns token": (r) => r.json("token") !== undefined,
  });

  if (authRes.status !== 200) {
    console.error("Failed to get auth token");
    return;
  }

  const token = authRes.json("token");
  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };

  // Create profile
  const createPayload = generateProfileData();
  console.log("Create payload:", JSON.stringify(createPayload));

  const createRes = http.post(
    `${BASE_URL}/profiles`,
    JSON.stringify(createPayload),
    {
      headers: headers,
    }
  );

  console.log("Create response:", {
    status: createRes.status,
    body: createRes.body,
    error: createRes.error,
  });

  check(createRes, {
    "create status is 201": (r) => r.status === 201,
    "create has profile id": (r) => r.json("profile.id") !== undefined,
  });

  if (createRes.status === 201) {
    const profileId = createRes.json("profile.id");
    sleep(1);

    // Get profile
    const getRes = http.get(`${BASE_URL}/profiles/${profileId}`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    console.log("Get response:", {
      status: getRes.status,
      body: getRes.body,
      error: getRes.error,
    });

    check(getRes, {
      "get status is 200": (r) => r.status === 200,
      "get returns correct profile": (r) => r.json("profile.id") === profileId,
    });
    sleep(1);

    // Update profile
    const updatePayload = generateProfileData();
    console.log("Update payload:", JSON.stringify(updatePayload));

    const updateRes = http.put(
      `${BASE_URL}/profiles/${profileId}`,
      JSON.stringify(updatePayload),
      {
        headers: headers,
      }
    );

    console.log("Update response:", {
      status: updateRes.status,
      body: updateRes.body,
      error: updateRes.error,
    });

    check(updateRes, {
      "update status is 200": (r) => r.status === 200,
      "update returns updated profile": (r) =>
        r.json("profile.id") === profileId,
    });
    sleep(1);

    // Delete profile
    const deleteRes = http.del(`${BASE_URL}/profiles/${profileId}`, null, {
      headers: { Authorization: `Bearer ${token}` },
    });
    console.log("Delete response:", {
      status: deleteRes.status,
      body: deleteRes.body,
      error: deleteRes.error,
    });

    check(deleteRes, {
      "delete status is 204": (r) => r.status === 204,
    });
  }

  sleep(1);
}
