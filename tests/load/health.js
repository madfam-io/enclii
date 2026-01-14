/**
 * Enclii Health Check Load Test
 *
 * Tests the /health endpoint under load.
 * This is an unauthenticated endpoint - good for baseline performance.
 *
 * Usage:
 *   k6 run tests/load/health.js
 *   k6 run --env TARGET_RPS=500 tests/load/health.js
 *   k6 run --env ENCLII_BASE_URL=http://localhost:8080 tests/load/health.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';
import { CONFIG, STAGES } from './config.js';

// Custom metrics
const healthErrors = new Rate('health_errors');
const healthDuration = new Trend('health_duration');

// Test configuration
export const options = {
  scenarios: {
    health_check: {
      executor: 'ramping-vus',
      stages: STAGES.load,
      gracefulRampDown: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<200', 'p(99)<500'],  // Health should be fast
    http_req_failed: ['rate<0.001'],                // 0.1% error rate
    health_errors: ['rate<0.001'],
    health_duration: ['p(95)<200'],
  },
};

export default function () {
  const url = `${CONFIG.baseUrl}/health`;

  const res = http.get(url, {
    tags: { name: 'health_check' },
  });

  // Record custom metrics
  healthDuration.add(res.timings.duration);

  const success = check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 200ms': (r) => r.timings.duration < 200,
    'body contains status': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.status === 'healthy';
      } catch {
        return false;
      }
    },
  });

  healthErrors.add(!success);

  sleep(0.1);  // 100ms between requests per VU
}

export function handleSummary(data) {
  const http = data.metrics.http_req_duration;
  if (!http || !http.values) {
    return { stdout: 'No HTTP metrics collected' };
  }

  const summary = `
================================================================================
                          ENCLII HEALTH CHECK LOAD TEST
================================================================================

RESULTS SUMMARY
---------------
Total Requests:    ${data.metrics.http_reqs.values.count}
Failed Requests:   ${data.metrics.http_req_failed.values.passes}
Success Rate:      ${((1 - data.metrics.http_req_failed.values.rate) * 100).toFixed(2)}%

RESPONSE TIMES (ms)
-------------------
  Min:    ${http.values.min.toFixed(2)}
  Avg:    ${http.values.avg.toFixed(2)}
  Med:    ${http.values.med.toFixed(2)}
  P95:    ${http.values['p(95)'].toFixed(2)}
  P99:    ${http.values['p(99)'].toFixed(2)}
  Max:    ${http.values.max.toFixed(2)}

SLO COMPLIANCE
--------------
  P95 < 200ms:     ${http.values['p(95)'] < 200 ? 'PASS' : 'FAIL'}
  Error Rate < 0.1%: ${data.metrics.http_req_failed.values.rate < 0.001 ? 'PASS' : 'FAIL'}

================================================================================
`;

  return {
    stdout: summary,
    'tests/load/results/health-summary.json': JSON.stringify(data, null, 2),
  };
}
