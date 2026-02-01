# Auto-Grocery: Automated Grocery Ordering & Delivery System

**Architecture Style:** Event-Driven Microservices (Distributed System)
**Repository Strategy:** Monorepo
**Core Stack:** Go (Backend), Python (Frontend), gRPC, ZeroMQ, Protobuf

## 1. System Overview

This system simulates a fully automated, cloud-based grocery warehouse. It handles orders from "Smart Clients" (IoT fridges) and restocking supplies from trucks. A central "Inventory Brain" coordinates a fleet of autonomous robots to fetch or restock items in real-time, while tracking costs and performance metrics.

## 2. Microservices Architecture

The system is composed of **5 distinct microservices**, each with strict isolation and specific responsibilities.

| Service Name | Role & Analogy | Responsibility | Database / State | Protocols |
| :--- | :--- | :--- | :--- | :--- |
| **1. Ordering Service** | **The Gateway**<br>*(Front Desk)* | • **Public API:** Accepts HTTP/JSON requests from clients.<br>• **Translation:** Converts JSON to Protobuf.<br>• **Latency Tracking:** Measures request time for Analytics. | **User DB**<br>(User credentials, Order history) | **In:** HTTP (REST)<br>**Out:** gRPC |
| **2. Inventory Service** | **The Brain**<br>*(Manager)* | • **Orchestrator:** Central hub managing the workflow.<br>• **State Management:** Tracks real-time stock levels.<br>• **Broadcasting:** Publishes tasks to Robots via ZeroMQ.<br>• **Billing:** Asks Pricing Service for costs. | **Stock DB**<br>(Item counts, Aisle location) | **In:** gRPC<br>**Out:** ZeroMQ (PUB) |
| **3. Robot Service** | **The Workers**<br>*(Aisle Bots)* | • **5 Instances:** Bread, Meat, Produce, Dairy, Party.<br>• **Execution:** Listens for orders, simulates fetch time (`sleep`), and updates Inventory.<br>• **Isolation:** Robots only process tasks for their specific aisle. | **Stateless**<br>(Knows own category only) | **In:** ZeroMQ (SUB)<br>**Out:** gRPC |
| **4. Pricing Service** | **The Accountant**<br>*(Calculator)* | • **Valuation:** Calculates total bill based on fetched items.<br>• **Logic:** Applies unit prices to quantities. | **Catalog DB**<br>(Read-only Price List) | **In:** gRPC<br>**Out:** gRPC |
| **5. Analytics Service** | **The Observer**<br>*(Scoreboard)* | • **Monitoring:** Passive listener for system stats.<br>• **Reporting:** Records latency, success/fail rates, and throughput. | **TimeSeries Log**<br>(Append-only metrics) | **In:** ZeroMQ (PULL) |

## 3. Communication Protocols

This system intentionally uses multiple protocols to demonstrate different communication patterns:

1.  **Client ↔ Ordering (HTTP + JSON):** Standard web compatibility for browsers/GUIs.
2.  **Ordering ↔ Inventory (gRPC + Protobuf):** Strict, high-performance internal contracts.
3.  **Inventory → Robots (ZeroMQ + Flatbuffers):** **Broadcasting (Pub/Sub).** The Inventory publishes a task *once*, and all relevant robots receive it instantly. Flatbuffers ensures zero-copy deserialization speed.
4.  **Robot → Inventory (gRPC):** **Reliability.** Robots need a guaranteed acknowledgement that their work was recorded.

## 4. Port Configuration (Localhost Development)

For Milestone 1 & 2, all services run on `localhost` using specific ports to simulate a distributed network.

* **Ordering Service:** Port `5000` (HTTP)
* **Inventory Service:**
    * Port `50051` (gRPC Server - Input)
    * Port `5555` (ZeroMQ Publisher - Output)
* **Pricing Service:** Port `50052` (gRPC)
* **Analytics Service:** Port `5556` (ZeroMQ)
* **Robots:** (No open ports; they connect outbound to `5555`)
* **Frontend (Streamlit):** Port `8501`

## 5. Prerequisites

* **Go 1.21+**
* **Python 3.10+**
* **Protobuf Compiler (`protoc`)**
* **ZeroMQ Library** (`libzmq`)

## 6. Project Structure

```text
auto-grocery/
├── proto/               # Shared Protocol Buffers & Flatbuffers
├── ordering/            # HTTP Gateway Service
├── inventory/           # Core Orchestrator Service
├── pricing/             # Pricing Logic Service
├── robots/              # Robot Workers
├── analytics/           # Metrics & Logging Service
├── client/              # Python Streamlit Frontend
└── docker-compose.yml   # (Milestone 3) Deployment Config
