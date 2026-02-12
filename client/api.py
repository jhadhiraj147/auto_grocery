"""API client for Auto-Grocery Ordering Service."""

import os
import requests
from typing import Optional

BASE_URL = os.environ.get("ORDERING_API_URL", "http://localhost:5050")

CONNECTION_ERROR_MSG = (
    f"Cannot connect to Ordering Service at {BASE_URL}. "
    "Make sure it's running (e.g. from the ordering directory: go run ./cmd/ordering)."
)


def _headers(token: Optional[str] = None) -> dict:
    h = {"Content-Type": "application/json"}
    if token:
        h["Authorization"] = f"Bearer {token}"
    return h


def _request(method: str, path: str, **kwargs) -> tuple[dict, int]:
    """Make HTTP request. Returns (response_dict, status_code). On connection error, returns ({error: str}, 0)."""
    try:
        r = requests.request(method, f"{BASE_URL}{path}", timeout=10, **kwargs)
        return r.json() if r.text else {}, r.status_code
    except requests.exceptions.ConnectionError:
        return {"error": CONNECTION_ERROR_MSG}, 0


# --- Client (Smart Refrigerator) ---

def client_register(device_id: str, password: str, email: str, phone: str) -> tuple[dict, int]:
    """Register a smart refrigerator client."""
    return _request("POST", "/api/client/register", json={
        "device_id": device_id,
        "password": password,
        "email": email,
        "phone": phone,
    }, headers=_headers())


def client_login(device_id: str, password: str) -> tuple[dict, int]:
    """Login and get access/refresh tokens."""
    return _request("POST", "/api/client/login", json={
        "device_id": device_id,
        "password": password,
    }, headers=_headers())


def order_preview(token: str, items: list[dict]) -> tuple[dict, int]:
    """Preview order (reserve items). items: [{"sku": "...", "quantity": N}]"""
    return _request("POST", "/api/client/order/preview", json={"items": items}, headers=_headers(token))


def order_confirm(token: str, order_id: str) -> tuple[dict, int]:
    """Confirm order and checkout."""
    return _request("POST", "/api/client/order/confirm", json={"order_id": order_id}, headers=_headers(token))


def order_cancel(token: str, order_id: str) -> tuple[dict, int]:
    """Cancel a pending order."""
    return _request("POST", "/api/client/order/cancel", json={"order_id": order_id}, headers=_headers(token))


def order_history(token: str) -> tuple[dict, int]:
    """Get order history."""
    return _request("GET", "/api/client/orders", headers=_headers(token))


def order_last(token: str) -> tuple[dict, int]:
    """Get last order."""
    return _request("GET", "/api/client/orders/last", headers=_headers(token))


# --- Truck ---

def truck_register(truck_id: str, plate_number: str, driver_name: str) -> tuple[dict, int]:
    """Register a truck."""
    return _request("POST", "/api/truck/register", json={
        "truck_id": truck_id,
        "plate_number": plate_number,
        "driver_name": driver_name,
    }, headers=_headers())


def truck_restock(
    truck_id: str,
    plate_number: str,
    driver_name: str,
    supplier_id: str,
    items: list[dict],
    contact_info: str = "",
    location: str = "",
) -> tuple[dict, int]:
    """Submit restock order. items: [{"sku", "name", "aisle_type", "quantity", "mfd_date", "expiry_date", "unit_cost"}]"""
    return _request("POST", "/api/truck/restock", json={
        "truck_id": truck_id,
        "plate_number": plate_number,
        "driver_name": driver_name,
        "contact_info": contact_info,
        "location": location,
        "supplier_id": supplier_id,
        "items": items,
    }, headers=_headers())
