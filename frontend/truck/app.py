import streamlit as st
import requests

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

st.title("🚛 TRUCK OFFLOAD TERMINAL")

# --- SIDEBAR: TRUCK & SUPPLIER DATA ---
with st.sidebar:
    st.header("Truck Identity")
    truck_id = st.text_input("truck_id")
    plate_number = st.text_input("plate_number")
    driver_name = st.text_input("driver_name")
    contact_info = st.text_input("contact_info")
    location = st.text_input("location")
    supplier_id = st.text_input("supplier_id")

# --- MAIN: DYNAMIC TABLE ---
st.header("Inventory Manifest")

# Table Headers
h1, h2, h3, h4, h5 = st.columns([2, 2, 2, 1, 1])
h1.write("SKU")
h2.write("Name")
h3.write("Aisle Type")
h4.write("Quantity")
h5.write("Unit Cost")

# Accurate Aisle Options
AISLE_OPTIONS = ["bread", "dairy", "produce", "meat", "party"]

for i, item in enumerate(st.session_state.restock_items):
    c1, c2, c3, c4, c5 = st.columns([2, 2, 2, 1, 1])
    st.session_state.restock_items[i]["sku"] = c1.text_input("sku", value=item["sku"], label_visibility="collapsed", key=f"sku_{i}")
    st.session_state.restock_items[i]["name"] = c2.text_input("name", value=item["name"], label_visibility="collapsed", key=f"name_{i}")
    st.session_state.restock_items[i]["aisle_type"] = c3.selectbox("aisle_type", AISLE_OPTIONS, index=AISLE_OPTIONS.index(item["aisle_type"]) if item["aisle_type"] in AISLE_OPTIONS else 2, label_visibility="collapsed", key=f"aisle_{i}")
    st.session_state.restock_items[i]["quantity"] = c4.number_input("quantity", min_value=1, value=item["quantity"], label_visibility="collapsed", key=f"qty_{i}")
    st.session_state.restock_items[i]["unit_cost"] = c5.number_input("unit_cost", min_value=0.0, value=item["unit_cost"], label_visibility="collapsed", key=f"cost_{i}")

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
    if not truck_id or not plate_number or not supplier_id:
        st.error("Missing mandatory fields: truck_id, plate_number, or supplier_id")
    else:
        manifest = []
        for it in st.session_state.restock_items:
            if it["sku"]:
                manifest.append({
                    "sku": it["sku"],
                    "name": it["name"],
                    "aisle_type": it["aisle_type"],
                    "quantity": int(it["quantity"]),
                    "unit_cost": float(it["unit_cost"]),
                    "mfd_date": "2024-01-01",
                    "expiry_date": "2025-01-01"
                })

        payload = {
            "truck_id": truck_id,
            "plate_number": plate_number,
            "driver_name": driver_name,
            "contact_info": contact_info,
            "location": location,
            "supplier_id": supplier_id,
            "items": manifest
        }

        try:
            res = requests.post(f"{BASE_URL}/api/truck/restock", json=payload)
            if res.status_code == 201:
                st.success("SUCCESS: Truck Offloaded and Inventory Updated.")
                st.balloons()
                st.session_state.restock_items = [{"sku": "", "name": "", "aisle_type": "produce", "quantity": 1, "unit_cost": 0.0}]
            else:
                st.error(f"FAILURE: {res.status_code} - {res.text}")
        except Exception as e:
            st.error(f"CONNECTION ERROR: {str(e)}")