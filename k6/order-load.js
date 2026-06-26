/**
 * k6 load test — order-service
 *
 * Flow:
 *   1. setup()   — tạo 1 product thật để lấy product_id + price
 *   2. default() — POST /api/v1/orders liên tục để trigger Temporal saga
 *   3. teardown() — không cần cleanup, orders ở trạng thái CONFIRMED/FAILED trong DB
 *
 * Lưu ý: order-service verify price với product-service nên product-service
 * phải đang chạy. Chạy cả 2 script cùng lúc để thấy cả 2 HPA scale.
 *
 * Run:
 *   k6 run k6/order-load.js
 *   k6 run --env BASE_URL=http://localhost k6/order-load.js
 *   k6 run --env PRODUCT_ID=<uuid> --env CURRENCY=USD k6/order-load.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const BASE_URL      = __ENV.BASE_URL  || 'http://localhost';
const PRODUCT_SVC   = __ENV.PRODUCT_BASE_URL || BASE_URL;

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '30s', target: 20  },  // ramp-up nhẹ — order tốn resource hơn product
    { duration: '1m',  target: 100 },  // normal
    { duration: '2m',  target: 300 },  // high load — trigger HPA cho order-service + order-worker
    { duration: '1m',  target: 100 },  // step down
    { duration: '30s', target: 0   },  // ramp-down
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'],  // order chậm hơn do có Temporal workflow
    errors:            ['rate<0.05'],   // cho phép 5% error (stock có thể hết)
  },
};

export function setup() {
  // Dùng PRODUCT_ID từ env nếu có, không thì tạo mới
  if (__ENV.PRODUCT_ID) {
    return { productId: __ENV.PRODUCT_ID, currency: __ENV.CURRENCY || 'USD' };
  }

  const res = http.post(
    `${PRODUCT_SVC}/api/v1/products`,
    JSON.stringify({
      name: 'k6 Order Test Product',
      sku: `ORDER-TEST-${Date.now()}`,
      price_cents: 50000,   // 500 USD
      currency: 'USD',
      stock: 99999,         // stock cao để không bị INSUFFICIENT_STOCK
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );

  if (res.status !== 201) {
    console.error(`setup: failed to create product — ${res.status} ${res.body}`);
    return { productId: null };
  }

  const product = JSON.parse(res.body);
  console.log(`setup: created product ${product.id} (${product.sku})`);
  return { productId: product.id, currency: product.currency || 'USD' };
}

export default function (data) {
  if (!data.productId) {
    console.warn('no product id — skipping iteration');
    sleep(1);
    return;
  }

  const quantity = Math.floor(Math.random() * 3) + 1; // 1–3 items

  const res = http.post(
    `${BASE_URL}/api/v1/orders`,
    JSON.stringify({
      user_id:  `user-${__VU}`,         // VU = virtual user, unique per concurrent user
      currency: data.currency,
      items: [
        {
          product_id: data.productId,
          sku:        'ORDER-TEST',
          quantity:   quantity,
          unit_cents: 0,                 // server sẽ override với giá thật
        },
      ],
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );

  const ok = check(res, {
    'order: status 202': (r) => r.status === 202,
    'order: has id':     (r) => {
      try { return !!JSON.parse(r.body).id; } catch { return false; }
    },
  });

  if (!ok) {
    errorRate.add(1);
    if (res.status !== 409) { // 409 = insufficient stock, expected
      console.error(`order failed: ${res.status} — ${res.body}`);
    }
  }

  sleep(Math.random() * 1 + 0.5); // 0.5–1.5s think time (order nặng hơn)
}
