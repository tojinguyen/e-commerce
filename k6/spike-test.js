/**
 * k6 spike test — đột ngột tăng traffic gấp 10x để xem HPA phản ứng nhanh thế nào.
 * Dùng cho product-service (endpoint GET đơn giản, dễ tạo load lớn).
 *
 * Run:
 *   k6 run k6/spike-test.js
 */

import http from 'k6/http';
import { check } from 'k6';
import { Rate } from 'k6/metrics';

const BASE_URL  = __ENV.BASE_URL || 'http://localhost';
const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '10s', target: 50   }, // baseline bình thường
    { duration: '10s', target: 1000 }, // SPIKE — đột ngột x20 để trigger HPA ngay
    { duration: '3m',  target: 1000 }, // giữ spike để quan sát HPA scale-out
    { duration: '10s', target: 50   }, // drop đột ngột
    { duration: '5m',  target: 50   }, // giữ thấp để quan sát HPA scale-in (5 phút cooldown)
  ],
  thresholds: {
    http_req_duration: ['p(99)<3000'], // nới rộng threshold khi spike
    errors:            ['rate<0.10'],
  },
};

export function setup() {
  const res = http.post(
    `${BASE_URL}/api/v1/products`,
    JSON.stringify({
      name: 'Spike Test Product',
      sku: `SPIKE-${Date.now()}`,
      price_cents: 9900,
      currency: 'USD',
      stock: 9999,
    }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  if (res.status === 201) return { id: JSON.parse(res.body).id };
  return { id: null };
}

export default function (data) {
  if (!data.id) return;
  const res = http.get(`${BASE_URL}/api/v1/products/${data.id}`);
  check(res, { 'status 200': (r) => r.status === 200 }) || errorRate.add(1);
}
