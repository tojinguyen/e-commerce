/**
 * k6 load test — order-service
 *
 * Mỗi iteration chọn ngẫu nhiên 1-3 sản phẩm từ PRODUCT_IDS, tạo order với
 * các items đó. order-service sẽ verify price với product-service rồi kick off
 * Temporal saga (ReserveInventory → ProcessPayment → CreateShipment → Confirm).
 *
 * Run:
 *   k6 run k6/order-load.js
 *   k6 run --env BASE_URL=http://localhost k6/order-load.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost';

const PRODUCT_IDS = [
  'ca33fe44-d444-4594-91ff-ce1d10e17a10',
  'e59f750b-b8ba-4817-907e-5c55fe19a933',
  '22744ad3-40d9-4759-acbf-41559433e969',
  '0ea1b51a-ac2f-45e8-b118-51ac79632ec5',
  '7cc2b119-4e07-4960-b895-b554b4186e9a',
  'ea53716d-2120-4737-a744-2ae6b0e125b1',
];

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '30s', target: 20  },  // ramp-up
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

function pickItems() {
  // Shuffle rồi lấy 1-3 sản phẩm đầu để mỗi order có combo khác nhau
  const shuffled = PRODUCT_IDS.slice().sort(() => Math.random() - 0.5);
  const count = Math.floor(Math.random() * 3) + 1;
  return shuffled.slice(0, count).map((id) => ({
    product_id: id,
    sku:        'LOAD-TEST',
    quantity:   1,
    unit_cents: 0, // server override với giá thật từ product-service
  }));
}

export default function () {
  const res = http.post(
    `${BASE_URL}/api/v1/orders`,
    JSON.stringify({
      user_id:  `user-${__VU}`,
      currency: 'USD',
      items:    pickItems(),
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
    if (res.status !== 409) { // 409 = insufficient stock, expected under load
      console.error(`order failed: ${res.status} — ${res.body}`);
    }
  }

  sleep(Math.random() * 1 + 0.5); // 0.5–1.5s think time
}
