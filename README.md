# Auto Grocery — Distributed Smart Store Platform

Auto Grocery is a multi-service system that automates customer order fulfillment and truck restock operations using microservices, gRPC, HTTP, ZeroMQ, PostgreSQL, Redis, and Streamlit-based UIs.

---

## 1) System Architecture (High Level)

Core backend services:
- `ordering` (Go, HTTP) — API gateway/orchestrator for client + truck flows
- `inventory` (Go, gRPC) — stock reservation, robot dispatch, and completion orchestration
- `pricing` (Go, gRPC) — price updates and bill calculation
- `robots` (C++) — worker processes subscribed by aisle to robot tasks
- `analytics` (C++) — subscriber service that logs completion latency metrics

Frontends:
- `frontend/client` (Streamlit)
- `frontend/truck` (Streamlit)

Stateful infrastructure:
- PostgreSQL (service-owned DBs for ordering/inventory/pricing)
- Redis (inventory runtime state: in-flight orders, counters, finalize guard)

---

## 2) Runtime Communication Topology

### APIs and Message Buses
- Client UI -> `ordering` via HTTP (`:5050`)
- Truck UI -> `ordering` via HTTP (`:5050`)
- `ordering` -> `inventory` via gRPC (`:50051`)
- `inventory` -> `pricing` via gRPC (`:50052`)
- `inventory` -> `robots` via ZeroMQ PUB/SUB (`:5556`)
- `robots` -> `inventory` via gRPC status callback (`ReportJobStatus`)
- `inventory` -> `ordering` via internal webhook HTTP callbacks:
  - `/internal/webhook/update-order`
  - `/internal/webhook/update-restock`
- `ordering` -> `analytics` via ZeroMQ PUB/SUB (`:5557`)

### Security on Internal HTTP
- Internal webhook endpoints are protected with `X-Internal-Secret`.
- Secret must match between `ordering/.env` and `inventory/.env`.

---

## 3) End-to-End Business Flows

### A. Customer Order Flow
1. Client authenticates with `ordering` (`register/login/refresh`).
2. `preview` call reserves stock through `inventory.ReserveItems`.
3. `ordering` stores order as `PENDING` in PostgreSQL.
4. `confirm` call triggers `inventory.ProcessCustomerOrder`.
5. `inventory` publishes robot tasks over ZMQ (`order_type=CUSTOMER`).
6. Robot workers process aisle-matching items and report via `ReportJobStatus`.
7. On completion threshold, `inventory` finalizes:
	- Computes bill through `pricing.CalculateBill`
	- Webhooks `ordering` with final status + total price
8. `ordering` updates DB and publishes analytics latency metric.

### B. Truck Restock Flow
1. Truck UI submits manifest to `ordering`.
2. `ordering` saves restock order and calls `inventory.RestockItemsOrder`.
3. `inventory` publishes robot tasks (`order_type=RESTOCK`).
4. Robot workers report status back to `inventory`.
5. On completion, `inventory`:
	- Upserts inventory stock in PostgreSQL
	- Sends stock metrics to `pricing.UpdateStockMetrics`
	- Webhooks `ordering` with final status + total cost
6. `ordering` updates restock status and publishes analytics metric.

---

## 4) Service Responsibilities

### `ordering`
- Public HTTP API for client and truck operations
- JWT auth and refresh-token lifecycle
- Internal webhook receiver for completion updates
- Persists customer/truck order state in PostgreSQL
- Publishes analytics events via ZMQ

### `inventory`
- Authoritative stock reservation/release logic
- Robot dispatch producer and progress aggregator
- Redis-backed ephemeral workflow state
- Finalization coordinator (client billing + restock stock sync)

### `pricing`
- Maintains SKU price catalog
- Calculates order bills
- Recalculates prices from stock/cost metrics

### `robots`
- Aisle-scoped workers; argument must match item `aisle_type` values (e.g., `bread`, `meat`, `produce`, `dairy`, `party`)
- Consumes task broadcasts and reports per-order status

### `analytics`
- Subscribes to completion metrics
- Persists latency rows to CSV

---

## 5) Why This Tech Stack

### Go for Core Services (`ordering`, `inventory`, `pricing`)
- Strong fit for networked microservices
- Efficient concurrency model
- Good gRPC + PostgreSQL + Redis ecosystem support
- Fast compile/build and operational simplicity

### C++ for `robots` and `analytics`
- Suitable for low-latency worker-style components
- Fine-grained control for message parsing and runtime behavior
- Natural pairing with existing CMake + protobuf/grpc/zmq toolchain

### gRPC for Service-to-Service RPC
- Strongly typed contracts via `.proto`
- Efficient binary protocol and stable interface evolution
- Clear separation of service boundaries

### ZeroMQ for Event/Broadcast Channels
- Lightweight, high-throughput PUB/SUB pattern
- Excellent fit for one-to-many robot task dispatch
- Decouples producer/consumer lifecycle

### PostgreSQL for Persistent Domain State
- ACID transactions for order + inventory correctness
- Relational model suits order headers/items and catalog data
- Mature tooling and portability

### Redis for Ephemeral Workflow State
- Fast counters and transient order progress tracking
- One-time finalization guard (`SETNX`) to avoid duplicate completion actions

### Streamlit for UI Prototypes
- Fast iteration for demonstration-grade operational dashboards
- Minimal frontend overhead while integrating live APIs

---

## 6) Configuration and Ports

Endpoints are env-driven (no hardcoded host/port requirements).

Default local bindings/targets:
- Ordering HTTP: `:5050`
- Inventory gRPC: `:50051`
- Pricing gRPC: `:50052`
- Robot ZMQ bus: `tcp://*:5556`
- Analytics ZMQ bus: `tcp://*:5557`

Env files:
- `ordering/.env`
- `inventory/.env`
- `pricing/.env`
- `robots/.env`
- `analytics/.env`

These `.env` files are intentionally committed with local/demo defaults (no external API keys required).

---

## 7) Documentation Artifacts

Instantiation guide:
- `HowToInstantiateServices.txt`

Per-service deep-dive guides:
- `ordering/OrderingService_Guide.txt`
- `inventory/InventoryService_Guide.txt`
- `pricing/PricingService_Guide.txt`
- `robots/RobotsService_Guide.txt`
- `analytics/AnalyticsService_Guide.txt`

---

## 8) Quick Verification (Runtime)

Example listener check:
```bash
lsof -nP -iTCP:5050 -iTCP:50051 -iTCP:50052 -iTCP:5556 -iTCP:5557 -sTCP:LISTEN
```

Example established dependency check:
```bash
lsof -nP -iTCP:5432 -iTCP:6379 -sTCP:ESTABLISHED
```

---

## 9) Contributors

- **Dhiraj Jha** — Inventory, DB (PostgreSQL + Redis), Pricing, Ordering
- **Saugat Lamichhane** — Robot, Analytics, Debugging
- **Diamond GC** — Truck UI, Client UI, Testing

