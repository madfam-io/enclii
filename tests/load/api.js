/**
 * Enclii API Load Test
 *
 * Tests authenticated API endpoints under load.
 * Requires a valid API token.
 *
 * Usage:
 *   k6 run --env ENCLII_TOKEN=<token> tests/load/api.js
 *   k6 run --env ENCLII_TOKEN=<token> --env TARGET_RPS=1000 tests/load/api.js
 *
 * Get a token:
 *   enclii login && cat ~/.enclii/token
 */

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { CONFIG, STAGES } from './config.js';

// Custom metrics
const apiErrors = new Rate('api_errors');
const projectsLatency = new Trend('projects_list_latency');
const projectsCount = new Counter('projects_requests');

// Test configuration
export const options = {
  scenarios: {
    api_load: {
      executor: 'ramping-vus',
      stages: STAGES.load,
      gracefulRampDown: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],
    api_errors: ['rate<0.01'],
    projects_list_latency: ['p(95)<500'],
  },
};

// Validate token at start
export function setup() {
  if (!CONFIG.authToken) {
    console.error('ERROR: ENCLII_TOKEN environment variable not set');
    console.error('Get a token: enclii login && cat ~/.enclii/token');
    throw new Error('Missing authentication token');
  }

  // Validate token works
  const res = http.get(`${CONFIG.baseUrl}/api/v1/projects`, {
    headers: { 'Authorization': `Bearer ${CONFIG.authToken}` },
  });

  if (res.status === 401) {
    throw new Error('Invalid or expired token. Please re-authenticate.');
  }

  console.log(`Authenticated successfully. Running against ${CONFIG.baseUrl}`);
  return { token: CONFIG.authToken };
}

export default function (data) {
  const headers = {
    'Authorization': `Bearer ${data.token}`,
    'Content-Type': 'application/json',
  };

  group('Projects API', function () {
    // List projects
    const listRes = http.get(`${CONFIG.baseUrl}/api/v1/projects`, {
      headers,
      tags: { name: 'list_projects' },
    });

    projectsLatency.add(listRes.timings.duration);
    projectsCount.add(1);

    const listSuccess = check(listRes, {
      'list projects - status 200': (r) => r.status === 200,
      'list projects - response time < 500ms': (r) => r.timings.duration < 500,
      'list projects - valid JSON': (r) => {
        try {
          JSON.parse(r.body);
          return true;
        } catch {
          return false;
        }
      },
    });

    apiErrors.add(!listSuccess);
  });

  group('Health Endpoints', function () {
    // Health check (baseline)
    const healthRes = http.get(`${CONFIG.baseUrl}/health`, {
      tags: { name: 'health' },
    });

    check(healthRes, {
      'health - status 200': (r) => r.status === 200,
    });

    // Build status (authenticated)
    const buildStatusRes = http.get(`${CONFIG.baseUrl}/api/v1/build/status`, {
      headers,
      tags: { name: 'build_status' },
    });

    check(buildStatusRes, {
      'build status - status 200 or 404': (r) => r.status === 200 || r.status === 404,
    });
  });

  sleep(Math.random() * 0.5 + 0.5);  // 500-1000ms between iterations
}

export function handleSummary(data) {
  const metrics = data.metrics;
  const httpDuration = metrics.http_req_duration;
  const projectsLat = metrics.projects_list_latency;

  const summary = `
================================================================================
                          ENCLII API LOAD TEST RESULTS
================================================================================

TARGET: ${CONFIG.baseUrl}
TEST TYPE: Ramping VU Load Test

OVERALL RESULTS
---------------
Total Requests:       ${metrics.http_reqs.values.count}
Failed Requests:      ${Math.round(metrics.http_req_failed.values.rate * metrics.http_reqs.values.count)}
Success Rate:         ${((1 - metrics.http_req_failed.values.rate) * 100).toFixed(2)}%
Total Data Received:  ${(metrics.data_received.values.count / 1024 / 1024).toFixed(2)} MB
Total Data Sent:      ${(metrics.data_sent.values.count / 1024 / 1024).toFixed(2)} MB

RESPONSE TIMES - ALL REQUESTS (ms)
----------------------------------
  Min:    ${httpDuration.values.min.toFixed(2)}
  Avg:    ${httpDuration.values.avg.toFixed(2)}
  Med:    ${httpDuration.values.med.toFixed(2)}
  P95:    ${httpDuration.values['p(95)'].toFixed(2)}
  P99:    ${httpDuration.values['p(99)'].toFixed(2)}
  Max:    ${httpDuration.values.max.toFixed(2)}

RESPONSE TIMES - LIST PROJECTS (ms)
-----------------------------------
  Requests: ${metrics.projects_requests?.values.count || 0}
  Avg:      ${projectsLat?.values.avg?.toFixed(2) || 'N/A'}
  P95:      ${projectsLat?.values['p(95)']?.toFixed(2) || 'N/A'}
  P99:      ${projectsLat?.values['p(99)']?.toFixed(2) || 'N/A'}

SLO COMPLIANCE
--------------
  API P95 < 500ms:     ${httpDuration.values['p(95)'] < 500 ? 'PASS' : 'FAIL'} (${httpDuration.values['p(95)'].toFixed(0)}ms)
  API P99 < 1000ms:    ${httpDuration.values['p(99)'] < 1000 ? 'PASS' : 'FAIL'} (${httpDuration.values['p(99)'].toFixed(0)}ms)
  Error Rate < 1%:     ${metrics.http_req_failed.values.rate < 0.01 ? 'PASS' : 'FAIL'} (${(metrics.http_req_failed.values.rate * 100).toFixed(2)}%)

================================================================================
`;

  return {
    stdout: summary,
    'tests/load/results/api-summary.json': JSON.stringify(data, null, 2),
  };
}
