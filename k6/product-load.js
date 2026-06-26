/**
 * k6 load test — product-service
 *
 * Scenarios:
 *   1. browse   — GET /api/v1/products/:id          (read-heavy, most common)
 *   2. search   — GET /api/v1/products/search?q=... (ES full-text, heavier)
 *   3. suggest  — GET /api/v1/products/suggest?q=.. (ES completion)
 *   4. create   — POST /api/v1/products             (write, less frequent)
 *
 * Run:
 *   k6 run k6/product-load.js
 *   k6 run --env BASE_URL=http://localhost:8080 k6/product-load.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost';

// Custom metrics
const errorRate = new Rate('errors');
const searchDuration = new Trend('search_duration', true);

export const options = {
  stages: [
    { duration: '30s', target: 50  },  // ramp-up
    { duration: '1m',  target: 200 },  // normal load
    { duration: '2m',  target: 500 },  // high load — trigger HPA (CPU > 70%)
    { duration: '1m',  target: 200 },  // step down
    { duration: '30s', target: 0   },  // ramp-down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],   // 95% requests under 500ms
    errors:            ['rate<0.01'],   // error rate under 1%
  },
};

// Seed product IDs — replace với real IDs từ DB của mày
const PRODUCT_IDS = __ENV.PRODUCT_IDS
  ? __ENV.PRODUCT_IDS.split(',')
  : ['PLACEHOLDER_ID_1', 'PLACEHOLDER_ID_2'];

const SEARCH_TERMS = ['phone', 'laptop', 'shirt', 'book', 'headphone'];

export function setup() {
  // Tạo 1 product để lấy ID thực, dùng cho browse scenario
  const res = http.post(
    `${BASE_URL}/api/v1/products`,
    JSON.stringify({
      name: 'k6 Test Product',
      sku: `K6-TEST-${Date.now()}`,
      price_cents: 10000,
      currency: 'USD',
      stock: 9999,
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );

  if (res.status === 201) {
    const body = JSON.parse(res.body);
    return { productId: body.id };
  }
  return { productId: null };
}

export default function (data) {
  const rand = Math.random();

  if (rand < 0.50) {
    // 50% — browse single product (cache-friendly read)
    scenarioBrowse(data.productId);
  } else if (rand < 0.80) {
    // 30% — full-text search (ES load)
    scenarioSearch();
  } else if (rand < 0.95) {
    // 15% — autocomplete suggest
    scenarioSuggest();
  } else {
    // 5% — create product (write load)
    scenarioCreate();
  }

  sleep(Math.random() * 0.5); // 0–500ms think time
}

function scenarioBrowse(productId) {
  if (!productId) return;
  const res = http.get(`${BASE_URL}/api/v1/products/${productId}`);
  check(res, {
    'browse: status 200': (r) => r.status === 200,
  }) || errorRate.add(1);
}

function scenarioSearch() {
  const q = SEARCH_TERMS[Math.floor(Math.random() * SEARCH_TERMS.length)];
  const res = http.get(`${BASE_URL}/api/v1/products/search?q=${q}&size=10`);
  searchDuration.add(res.timings.duration);
  check(res, {
    'search: status 200': (r) => r.status === 200,
  }) || errorRate.add(1);
}

function scenarioSuggest() {
  const prefixes = ['ph', 'la', 'sh', 'bo', 'he'];
  const q = prefixes[Math.floor(Math.random() * prefixes.length)];
  const res = http.get(`${BASE_URL}/api/v1/products/suggest?q=${q}&size=5`);
  check(res, {
    'suggest: status 200': (r) => r.status === 200,
  }) || errorRate.add(1);
}

function scenarioCreate() {
  const res = http.post(
    `${BASE_URL}/api/v1/products`,
    JSON.stringify({
      name: `Load Test Product ${Date.now()}`,
      sku: `LT-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
      price_cents: Math.floor(Math.random() * 100000) + 1000,
      currency: 'USD',
      stock: 100,
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  check(res, {
    'create: status 201': (r) => r.status === 201,
  }) || errorRate.add(1);
}
