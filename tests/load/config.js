/**
 * Enclii Load Testing Configuration
 *
 * Configure these values for your environment before running tests.
 */

// Environment configuration
export const CONFIG = {
  // Target API URL (production)
  baseUrl: __ENV.ENCLII_BASE_URL || 'https://api.enclii.dev',

  // Authentication
  // Generate a token: enclii login && cat ~/.enclii/token
  authToken: __ENV.ENCLII_TOKEN || '',

  // Target RPS (requests per second)
  targetRPS: parseInt(__ENV.TARGET_RPS || '100'),

  // Test duration
  duration: __ENV.DURATION || '1m',

  // SLO Thresholds
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],  // ms
    http_req_failed: ['rate<0.01'],                   // 1% error rate
  },
};

// Stages for ramping up load
export const STAGES = {
  // Smoke test: minimal load
  smoke: [
    { duration: '30s', target: 5 },
    { duration: '1m', target: 5 },
    { duration: '30s', target: 0 },
  ],

  // Load test: normal expected load
  load: [
    { duration: '2m', target: 50 },
    { duration: '5m', target: 50 },
    { duration: '2m', target: 100 },
    { duration: '5m', target: 100 },
    { duration: '2m', target: 0 },
  ],

  // Stress test: find breaking point
  stress: [
    { duration: '2m', target: 100 },
    { duration: '5m', target: 200 },
    { duration: '5m', target: 500 },
    { duration: '5m', target: 1000 },
    { duration: '2m', target: 0 },
  ],

  // Spike test: sudden traffic spike
  spike: [
    { duration: '1m', target: 10 },
    { duration: '10s', target: 1000 },
    { duration: '1m', target: 1000 },
    { duration: '10s', target: 10 },
    { duration: '1m', target: 0 },
  ],

  // Soak test: long duration for memory leaks
  soak: [
    { duration: '5m', target: 100 },
    { duration: '1h', target: 100 },
    { duration: '5m', target: 0 },
  ],
};
