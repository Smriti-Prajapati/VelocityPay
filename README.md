# VelocityPay

A digital wallet backend I built in Go to understand how payment systems actually work under the hood — not just CRUD, but real concurrency, atomicity, and event-driven design.

Live demo → [smartspend-qrul.onrender.com](https://smartspend-qrul.onrender.com)

---

## Why I built this

I wanted to understand what happens when two people send money at the exact same time. How does a bank guarantee that ₹500 isn't deducted twice? How does fraud detection work without blocking real users? Building this forced me to answer those questions with actual code.

<img width="1919" height="1010" alt="image" src="https://github.com/user-attachments/assets/4150d40a-fd32-4786-ad09-686f8f74b312" />

<img width="1919" height="1011" alt="image" src="https://github.com/user-attachments/assets/455d0f32-eb9a-4368-9b0c-b46f394b514c" />

<img width="1914" height="1012" alt="image" src="https://github.com/user-attachments/assets/e8e3c19c-2eee-456b-85be-0be5771d8b87" />

<img width="1919" height="1009" alt="image" src="https://github.com/user-attachments/assets/efa85ad8-8c23-4cf8-90e1-f1fa9fe21c7d" />

---

## Stack

Go · Gin · PostgreSQL · Redis · RabbitMQ · Docker · Prometheus

---

## How the transfer works

This was the interesting part. A transfer isn't just two UPDATEs — it goes through:

1. Redis idempotency check (prevents duplicate requests)
2. Fraud engine evaluation (5 rules, composite risk score)
3. Worker pool (10 goroutines, buffered channel)
4. `BEGIN` → debit sender → credit receiver → `COMMIT`
5. DB constraint `CHECK (balance >= 0)` makes overdraft physically impossible
6. RabbitMQ event triggers notifications and audit log

If anything fails between steps, it rolls back. No partial states.

---

## Fraud detection

Five rules run before every transfer:

- **Velocity** — blocks >10 transfers in 30 seconds
- **Large amount** — flags transfers above a threshold
- **Rapid depletion** — flags when >80% of balance leaves in one transfer
- **Unusual hour** — flags transfers between 1–5 AM
- **New account** — flags accounts under 24 hours old

Scores are additive. Score ≥ 80 blocks the transaction.

---

## Running locally

```bash
git clone https://github.com/Smriti-Prajapati/VelocityPay.git
cd VelocityPay
cp .env.example .env
docker compose up --build
```

Open `http://localhost:8080`

---

## Environment variables

| Variable | Description |
|---|---|
| `JWT_SECRET` | Token signing secret |
| `DB_HOST` / `DB_NAME` / `DB_PASSWORD` | Neon PostgreSQL |
| `REDIS_ADDR` / `REDIS_PASSWORD` | Upstash Redis |
| `RABBITMQ_URL` | CloudAMQP |

---

## Tests

```bash
go test ./internal/... -v
go test -race ./internal/...
```

No Docker needed — everything uses in-memory stubs.

---

## What I learned

The hardest part wasn't writing the code — it was thinking through what happens when things fail halfway. If the worker pool is full, what do you return? If RabbitMQ is down, do you fail the transfer or just lose the notification? Every edge case forced a real engineering decision.

The fraud engine also made me think about false positive rates. Being too aggressive blocks real users. Too lenient and fraud gets through. Getting the scoring thresholds right required testing different combinations of rules.

---

Developed By Smriti Prajapati 
