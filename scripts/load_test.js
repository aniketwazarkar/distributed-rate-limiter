import http from 'k6/http';
import { check } from 'k6';

export const options = {
    scenarios: {
        // High load simulation
        high_throughput: {
            executor: 'ramping-arrival-rate',
            startRate: 1000,
            timeUnit: '1s',
            preAllocatedVUs: 500,
            maxVUs: 2000,
            stages: [
                { target: 10000, duration: '10s' }, // ramp up to 10k RPS
                { target: 50000, duration: '20s' }, // ramp up to 50k RPS
                { target: 50000, duration: '10s' }, // stay at 50k RPS
                { target: 0, duration: '10s' },     // ramp down
            ],
        },
    },
    thresholds: {
        http_req_duration: ['p(99)<20'], // 99% of requests must complete below 20ms
    },
};

export default function () {
    const url = 'http://localhost:8080/check';
    
    // Randomize users and endpoints to better simulate real distributed traffic mapping to different Redis cluster shards
    const userId = 'user_' + Math.floor(Math.random() * 10000);
    const endpoints = ['/api/v1/orders', '/api/v1/users', '/check', '/api/v1/products'];
    const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
    
    const payload = JSON.stringify({
        user_id: userId,
        endpoint: endpoint,
        ip: "10.0." + Math.floor(Math.random() * 255) + "." + Math.floor(Math.random() * 255)
    });

    const params = {
        headers: { 'Content-Type': 'application/json' },
    };

    const res = http.post(url, payload, params);

    // Assertions or checks
    check(res, {
        'is status 200 or 429': (r) => r.status === 200 || r.status === 429,
        'has rate limit header': (r) => r.headers['X-Ratelimit-Remaining'] !== undefined,
    });
}
