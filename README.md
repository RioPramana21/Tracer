# Alder

A small order-to-cash service: place an order, reserve stock, price it, invoice it, take
payment, issue refunds.

## Running locally

```
docker compose up -d
psql postgres://alder:alder@localhost:5432/alder -f backend/migrations/0001_init.sql
cd backend && go run ./cmd/api
```

The API listens on `:8080`.
