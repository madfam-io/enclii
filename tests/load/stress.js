/**
 * Enclii Stress Test
 *
 * Tests API performance under extreme load to find breaking points.
 * Target: 1,000 RPS
 *
 * Usage:
 *   k6 run --env ENCLII_TOKEN=<token> tests/load/stress.js
 *
 * WARNING: Only run against staging/test environments unless authorized.
 */

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { CONFIG, STAGES } from './config.js';

// Custom metrics
const errorRate = new Rate('errors');
const throughput = new Counter('throughput');
const p99Trend = new Trend('custom_p99');

// Stress test stages - ramping up to 1000 RPS
export const options = {
  scenarios: {
    stress_test: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      preAllocatedVUs: 500,
      maxVUs: 2000,
      stages: [
        { duration: '1m', target: 100 },   // Warm up
        { duration: '2m', target: 250 },   // Moderate load
        { duration: '2m', target: 500 },   // High load
        { duration: '3m', target: 1000 },  // Target: 1000 RPS
        { duration: '2m', target: 1000 },  // Sustain peak
        { duration: '1m', target: 0 },     // Cool down
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000', 'p(99)<2000'],  // More lenient for stress
    http_req_failed: ['rate<0.05'],                   // 5% error rate acceptable under stress
    errors: ['rate<0.05'],
  },
};

export function setup() {
  if (!CONFIG.authToken) {
    // For stress test, health endpoint doesn't require auth
    console.log('No token provided - testing unauthenticated endpoints only');
    return { authenticated: false };
  }

  // Validate token
  const res = http.get(`${CONFIG.baseUrl}/api/v1/projects`, {
    headers: { 'Authorization': `Bearer ${CONFIG.authToken}` },
  });

  if (res.status === 401) {
    console.log('Token invalid - testing unauthenticated endpoints only');
    return { authenticated: false };
  }

  console.log('Token valid - testing authenticated endpoints');
  return { authenticated: true, token: CONFIG.authToken };
}

export default function (data) {
  const baseUrl = CONFIG.baseUrl;

  // Mix of endpoints to simulate realistic traffic
  const rand = Math.random();

  if (rand < 0.6) {
    // 60% - Health checks (unauthenticated, fast)
    const res = http.get(`${baseUrl}/health`, {
      tags: { name: 'health', type: 'read' },
    });
    throughput.add(1);
    p99Trend.add(res.timings.duration);

    const success = check(res, {
      'health: status 200': (r) => r.status === 200,
    });
    errorRate.add(!success);

  } else if (rand < 0.9 && data.authenticated) {
    // 30% - List projects (authenticated, medium cost)
    const res = http.get(`${baseUrl}/api/v1/projects`, {
      headers: { 'Authorization': `Bearer ${data.token}` },
      tags: { name: 'projects', type: 'read' },
    });
    throughput.add(1);
    p99Trend.add(res.timings.duration);

    const success = check(res, {
      'projects: status 200': (r) => r.status === 200,
    });
    errorRate.add(!success);

  } else {
    // 10% - Build status (authenticated, light)
    if (data.authenticated) {
      const res = http.get(`${baseUrl}/api/v1/build/status`, {
        headers: { 'Authorization': `Bearer ${data.token}` },
        tags: { name: 'build_status', type: 'read' },
      });
      throughput.add(1);
      p99Trend.add(res.timings.duration);

      check(res, {
        'build_status: valid response': (r) => r.status === 200 || r.status === 404,
      });
    } else {
      // Fallback to health if not authenticated
      const res = http.get(`${baseUrl}/health`, {
        tags: { name: 'health', type: 'read' },
      });
      throughput.add(1);
      p99Trend.add(res.timings.duration);
    }
  }

  // No sleep - maximum throughput for stress test
}

export function handleSummary(data) {
  const metrics = data.metrics;
  const httpDuration = metrics.http_req_duration;
  const reqs = metrics.http_reqs;

  // Calculate actual RPS
  const testDuration = (data.state.testRunDurationMs || 660000) / 1000;
  const actualRPS = reqs.values.count / testDuration;

  const summary = `
================================================================================
                          ENCLII STRESS TEST RESULTS
================================================================================

TARGET: ${CONFIG.baseUrl}
GOAL: 1,000 RPS with P95 < 1s

THROUGHPUT
----------
Total Requests:    ${reqs.values.count.toLocaleString()}
Test Duration:     ${testDuration.toFixed(0)} seconds
Average RPS:       ${actualRPS.toFixed(0)}
Peak RPS:          ~1,000 (target)

RESPONSE TIMES (ms)
-------------------
  Min:    ${httpDuration.values.min.toFixed(2)}
  Avg:    ${httpDuration.values.avg.toFixed(2)}
  Med:    ${httpDuration.values.med.toFixed(2)}
  P90:    ${httpDuration.values['p(90)'].toFixed(2)}
  P95:    ${httpDuration.values['p(95)'].toFixed(2)}
  P99:    ${httpDuration.values['p(99)'].toFixed(2)}
  Max:    ${httpDuration.values.max.toFixed(2)}

ERROR ANALYSIS
--------------
Error Rate:        ${(metrics.http_req_failed.values.rate * 100).toFixed(2)}%
Failed Requests:   ${Math.round(metrics.http_req_failed.values.rate * reqs.values.count).toLocaleString()}

SLO COMPLIANCE (Under Stress)
-----------------------------
  1,000 RPS Achieved:  ${actualRPS >= 900 ? 'PASS' : 'FAIL'} (${actualRPS.toFixed(0)} RPS)
  P95 < 1000ms:        ${httpDuration.values['p(95)'] < 1000 ? 'PASS' : 'FAIL'} (${httpDuration.values['p(95)'].toFixed(0)}ms)
  P99 < 2000ms:        ${httpDuration.values['p(99)'] < 2000 ? 'PASS' : 'FAIL'} (${httpDuration.values['p(99)'].toFixed(0)}ms)
  Error Rate < 5%:     ${metrics.http_req_failed.values.rate < 0.05 ? 'PASS' : 'FAIL'} (${(metrics.http_req_failed.values.rate * 100).toFixed(2)}%)

BREAKING POINT ANALYSIS
-----------------------
${httpDuration.values['p(99)'] > 2000 || metrics.http_req_failed.values.rate > 0.05 ?
  'System showed signs of stress. Consider scaling resources.' :
  'System handled stress well. Capacity appears adequate.'}

================================================================================
`;

  return {
    stdout: summary,
    'tests/load/results/stress-summary.json': JSON.stringify(data, null, 2),
  };
}
