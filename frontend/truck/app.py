import streamlit as st
import requests
import time as time_module
from datetime import datetime, time, timedelta, timezone

# --- CONFIGURATION ---
BASE_URL = "http://localhost:5050"

st.set_page_config(page_title="Truck Terminal", page_icon="🚛", layout="wide")

# --- COOL ORANGE THEME ---
st.markdown("""
    <style>
    .stApp { background-color: #0E1117; color: #FFFFFF; }
    div.stButton > button:first-child {
        background-image: linear-gradient(to right, #FF8C00, #E65100);
        color: white; border-radius: 5px; border: none; font-weight: bold; width: 100%; height: 3em;
    }
    h1, h2, h3 { color: #FF8C00 !important; font-family: 'Courier New', monospace; }
    [data-testid="stForm"] { border: 1px solid #E65100; background-color: #161B22; }
    .stTextInput > div > div > input, .stNumberInput > div > div > input { color: #FF8C00; }
    </style>
    """, unsafe_allow_html=True)

# --- INITIALIZE STATE ---
if 'restock_items' not in st.session_state:
    st.session_state.restock_items = [{"sku": "", "name": "", "aisle_type": "produce", "quantity": 1, "unit_cost": 0.0}]
if 'restock_order_id' not in st.session_state:
    st.session_state.restock_order_id = None

st.title("🚛 TRUCK OFFLOAD TERMINAL")

# --- SIDEBAR: SUPPLIER DATA ---
with st.sidebar:
    st.header("Supplier Identity")
    st.caption("Required: enter supplier business id and readable supplier name before offloading.")
    supplier_id = st.text_input("Supplier ID", placeholder="e.g. SUPP-NESTLE-01")
    supplier_name = st.text_input("Supplier Name", placeholder="e.g. Nestle Waters")

# --- MAIN: DYNAMIC TABLE ---
st.header("Inventory Manifest")
st.caption("Fill one row per SKU. Use Add Item for more rows. SKU, Name, Quantity, Unit Cost are required for each row.")

st.subheader("Batch Dates")
st.caption("These dates are applied to all items in this offload batch.")
d1, d2 = st.columns(2)
mfd_date = d1.date_input("Manufacture Date")
expiry_date = d2.date_input("Expiry Date", value=(datetime.now().date() + timedelta(days=365)))

# Accurate Aisle Options
AISLE_OPTIONS = ["bread", "dairy", "produce", "meat", "party"]

for i, item in enumerate(st.session_state.restock_items):
    st.markdown(f"**Item {i + 1}**")
    c1, c2, c3, c4, c5 = st.columns([2, 2, 2, 1, 1])
    st.session_state.restock_items[i]["sku"] = c1.text_input("SKU", value=item["sku"], key=f"sku_{i}", placeholder="e.g. APPLE-101")
    st.session_state.restock_items[i]["name"] = c2.text_input("Name", value=item["name"], key=f"name_{i}", placeholder="e.g. Fresh Apples")
    st.session_state.restock_items[i]["aisle_type"] = c3.selectbox("Aisle Type", AISLE_OPTIONS, index=AISLE_OPTIONS.index(item["aisle_type"]) if item["aisle_type"] in AISLE_OPTIONS else 2, key=f"aisle_{i}")
    st.session_state.restock_items[i]["quantity"] = c4.number_input("Quantity", min_value=1, value=item["quantity"], key=f"qty_{i}")
    st.session_state.restock_items[i]["unit_cost"] = c5.number_input("Unit Cost", min_value=0.0, value=item["unit_cost"], key=f"cost_{i}")

# Table Controls
col_add, col_rem, _ = st.columns([1, 1, 4])
if col_add.button("➕ Add Item"):
    st.session_state.restock_items.append({"sku": "", "name": "", "aisle_type": "produce", "quantity": 1, "unit_cost": 0.0})
    st.rerun()
if col_rem.button("➖ Remove Item") and len(st.session_state.restock_items) > 1:
    st.session_state.restock_items.pop()
    st.rerun()

st.markdown("---")

# --- TRANSMISSION ---
if st.button("OFFLOAD TRUCK"):
    if not supplier_id or not supplier_name:
        st.error("Missing mandatory fields: supplier_id or supplier_name")
    else:
        mfd_iso = datetime.combine(mfd_date, time.min, tzinfo=timezone.utc).isoformat().replace("+00:00", "Z")
        exp_iso = datetime.combine(expiry_date, time.min, tzinfo=timezone.utc).isoformat().replace("+00:00", "Z")

        manifest = []
        for it in st.session_state.restock_items:
            if it["sku"]:
                manifest.append({
                    "sku": it["sku"],
                    "name": it["name"],
                    "aisle_type": it["aisle_type"],
                    "quantity": int(it["quantity"]),
                    "unit_cost": float(it["unit_cost"]),
                    "mfd_date": mfd_iso,
                    "expiry_date": exp_iso
                })

        if len(manifest) == 0:
            st.error("Add at least one item with a SKU before offloading")
            st.stop()

        payload = {
            "supplier_id": supplier_id,
            "supplier_name": supplier_name,
            "items": manifest
        }

        try:
            res = requests.post(f"{BASE_URL}/api/truck/restock", json=payload)
            if res.status_code == 201:
                body = res.json()
                order_id = body.get("order_id")
                st.session_state.restock_order_id = order_id
                st.success(f"SUCCESS: {body.get('message', 'Truck registered.')} | ORDER ID: {order_id}")
                with st.status("Robots are offloading stock...", expanded=True) as status_box:
                    for _ in range(120):
                        poll = requests.get(f"{BASE_URL}/api/truck/restock/status", params={"order_id": order_id})
                        if poll.status_code == 200:
                            data = poll.json().get("data", {})
                            state = data.get("Status")
                            total_cost = data.get("TotalCost", 0.0)
                            status_box.write(f"Restock Telemetry: `{state}`")
                            if state == "COMPLETED":
                                status_box.update(label=f"✅ OFFLOAD COMPLETE | FINAL TOTAL COST: ${total_cost}", state="complete")
                                st.balloons()
                                st.session_state.restock_items = [{"sku": "", "name": "", "aisle_type": "produce", "quantity": 1, "unit_cost": 0.0}]
                                st.session_state.restock_order_id = None
                                break
                            if "FAILED" in str(state):
                                status_box.update(label="❌ OFFLOAD FAILED", state="error")
                                break
                        time_module.sleep(2)
            else:
                st.error(f"FAILURE: {res.status_code} - {res.text}")
        except Exception as e:
            st.error(f"CONNECTION ERROR: {str(e)}")