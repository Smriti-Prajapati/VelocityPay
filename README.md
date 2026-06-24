# VelocityPay

A digital wallet platform I built to practice real backend engineering — not just hello world APIs, but actual fintech concepts like atomic transactions, fraud detection, and event-driven architecture.

The stack is Go, PostgreSQL, Redis, RabbitMQ, and Docker. The frontend is plain HTML/CSS/JS — no React, no frameworks.

---

## What it actually does

- Create a wallet, add money, send to other users
- Every transfer is atomic — money cannot disappear mid-transaction
- Fraud engine evaluates every transfer before it goes through
- Real-time notifications via RabbitMQ event consumers
- Spending analytics, audit logs, refund requests

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go + Gin |
| Database | PostgreSQL |
| Cache | Redis |
| Message Broker | RabbitMQ |
| Auth | JWT + bcrypt |
| Containers | Docker + Docker Compose |
| Monitoring | Prometheus + Grafana |
| Frontend | HTML5, CSS3, Vanilla JS |

---

## Running it locally

You just need Docker.

```bash
git clone https://github.com/Smriti-Prajapati/VelocityPay.git
cd VelocityPay

cp .env.example .env
# open .env and set a strong JWT_SECRET

docker compose up --build
```

Then open `http://localhost:8080`

| Service | URL | Credentials |
|---------|-----|-------------|
| App | http://localhost:8080 | — |
| RabbitMQ | http://localhost:15672 | guest / guest |
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3000 | admin / admin |

---

## Environment Variables

Copy `.env.example` to `.env` and fill these in:

| Variable | What it is |
|----------|------------|
| `JWT_SECRET` | Secret key for signing tokens — make this strong |
| `DB_HOST` | Postgres host |
| `DB_NAME` | Database name |
| `DB_USER` / `DB_PASSWORD` | Postgres credentials |
| `REDIS_ADDR` | Redis address |
| `RABBITMQ_URL` | RabbitMQ connection string |
| `APP_ENV` | `development` or `production` |

---

## How transfers work

This was the most interesting part to build. A transfer isn't two UPDATE statements — it's:

1. Check Redis for a duplicate idempotency key
2. Run the fraud engine (5 rules, composite risk score)
3. Create a pending transaction record
4. Send the job to a 10-goroutine worker pool
5. Inside the worker: `BEGIN → debit sender → credit receiver → COMMIT`
6. The DB has a `CHECK (balance >= 0)` constraint — makes it physically impossible to overdraft
7. Publish a RabbitMQ event → notification consumer picks it up

If anything fails, it rolls back and the transaction is marked failed. No partial states.

---

## Fraud Detection

Five rules run on every transfer before money moves:

| Rule | Triggers when |
|------|--------------|
| Velocity | More than 10 transfers in 30 seconds |
| Large transaction | Single transfer above ₹50,000 |
| Rapid depletion | Transfer uses more than 80% of balance |
| Unusual hour | Transfer between 1–5 AM UTC |
| New account | Account is less than 24 hours old |

Scores add up. If the total score hits 80 or above, the transfer is blocked. Otherwise it's allowed and the flags are saved for review.

---

## API Endpoints

```
POST   /api/v1/auth/register
POST   /api/v1/auth/login

POST   /api/v1/wallet/create
POST   /api/v1/wallet/add-money
GET    /api/v1/wallet/balance

POST   /api/v1/transactions/transfer
GET    /api/v1/transactions/history
GET    /api/v1/transactions/:id

POST   /api/v1/refunds
GET    /api/v1/refunds
PUT    /api/v1/refunds/:id/process

GET    /api/v1/notifications
PUT    /api/v1/notifications/read-all

GET    /api/v1/analytics/dashboard
GET    /api/v1/analytics/monthly
GET    /api/v1/analytics/platform

GET    /api/v1/fraud/alerts/me
GET    /api/v1/audit/me
```

All protected routes require `Authorization: Bearer <token>`.

---

## Tests

```bash
# Run all tests
go test ./internal/... -v

# With race detector
go test -race ./internal/...
```

56 unit tests across all service packages. No Docker needed — everything uses in-memory stubs.

---

## Project Layout

```
cmd/api/            → main.go, wires everything together
internal/
  ├── users/        → register, login, profile
  ├── wallet/       → create, add money, balance
  ├── transaction/  → atomic transfers, worker pool
  ├── refund/       → refund requests and reversal
  ├── notification/ → RabbitMQ consumer, in-app alerts
  ├── fraud/        → 5-rule fraud engine
  ├── analytics/    → spending reports, platform stats
  ├── audit/        → compliance logs
  ├── auth/         → JWT
  └── middleware/   → auth, rate limit, CORS, logging
migrations/         → PostgreSQL up/down files
web/                → frontend pages and assets
docker/             → Prometheus + Grafana config
```

---
Developed by **Smriti Prajapati**  

