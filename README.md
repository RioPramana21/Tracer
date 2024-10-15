# Alder

A small order-to-cash service: place an order, reserve stock, price it, invoice it, take
payment, issue refunds.

## Running locally

```
docker compose up -d
for f in backend/migrations/*.sql; do
  psql postgres://alder:alder@localhost:5432/alder -f "$f"
done
cd backend && go run ./cmd/api
```

The API listens on `:8080`.

In another terminal:

```
cd frontend
npm install
npm run dev
```

The app is served at `:5173`.
