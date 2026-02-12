# Auto-Grocery Client GUI

Streamlit-based client for the Auto-Grocery system.

## Client Types

1. **Smart Refrigerator** – Place grocery orders (IoT fridge simulation)
2. **Truck** – Submit restock orders (supply chain)

## Setup

```bash
cd client
pip install -r requirements.txt
```

## Run

**Important:** Start the Ordering Service first. The Streamlit client connects to it at `localhost:5050`.

```bash
# Terminal 1: Start Ordering Service (from project root)
cd ordering && go run ./cmd/ordering

# Terminal 2: Start Streamlit client
cd client && streamlit run app.py
```

Default URL: http://localhost:8501

## Configuration

- **Ordering API**: Set `ORDERING_API_URL` (default: `http://localhost:5050`)

```bash
export ORDERING_API_URL=http://localhost:5050
streamlit run app.py
```

## Usage

### Smart Refrigerator

1. **Register** – Create an account (device_id, password, email, phone)
2. **Login** – Get access token
3. **Place Order** – Select items from 5 aisles (Bread, Dairy, Meat, Produce, Party Supply)
4. **Preview & Reserve** – Reserve items via Inventory
5. **Confirm** or **Cancel** – Finalize or cancel the order
6. **Order History** – View past orders

### Truck Restock

1. Fill in truck info (Truck ID, Plate, Driver, Supplier ID)
2. Add items (SKU, name, aisle, quantity, dates, unit cost)
3. Submit – Truck is auto-registered; restock is sent to Inventory
