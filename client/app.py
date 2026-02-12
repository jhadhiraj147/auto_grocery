"""
Auto-Grocery Client GUI
Streamlit frontend for Smart Refrigerator (grocery orders) and Truck (restock).
"""

import streamlit as st

from api import (
    client_register,
    client_login,
    order_preview,
    order_confirm,
    order_cancel,
    order_history,
    order_last,
    truck_register,
    truck_restock,
)
from items import ITEMS_BY_AISLE, AISLE_TYPES

st.set_page_config(
    page_title="Auto-Grocery",
    page_icon="🛒",
    layout="wide",
    initial_sidebar_state="expanded",
)

# Custom styles for a modern look
st.markdown("""
<style>
    .main-header {
        font-size: 2rem;
        font-weight: 700;
        color: #1e3a5f;
        margin-bottom: 0.5rem;
    }
    .sub-header {
        color: #5a7d9a;
        font-size: 1rem;
        margin-bottom: 2rem;
    }
    .success-box {
        padding: 1rem 1.5rem;
        border-radius: 0.5rem;
        background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%);
        border-left: 4px solid #28a745;
        margin: 1rem 0;
    }
    .error-box {
        padding: 1rem 1.5rem;
        border-radius: 0.5rem;
        background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%);
        border-left: 4px solid #dc3545;
        margin: 1rem 0;
    }
    .order-card {
        padding: 1rem;
        border-radius: 0.5rem;
        background: #f8f9fa;
        border: 1px solid #dee2e6;
        margin: 0.5rem 0;
    }
</style>
""", unsafe_allow_html=True)


def init_session():
    if "access_token" not in st.session_state:
        st.session_state.access_token = None
    if "device_id" not in st.session_state:
        st.session_state.device_id = None
    if "pending_order_id" not in st.session_state:
        st.session_state.pending_order_id = None
    if "pending_items" not in st.session_state:
        st.session_state.pending_items = None


# ---------- Smart Refrigerator (Client) ----------

def render_client_register():
    st.subheader("Register Smart Refrigerator")
    with st.form("register_form"):
        device_id = st.text_input("Device ID", placeholder="e.g. fridge-kitchen-001")
        password = st.text_input("Password", type="password")
        email = st.text_input("Email", placeholder="user@example.com")
        phone = st.text_input("Phone", placeholder="+1-555-0123")
        submitted = st.form_submit_button("Register")
        if submitted:
            if not device_id or not password:
                st.error("Device ID and Password are required.")
            else:
                data, code = client_register(device_id, password, email, phone)
                if code == 201:
                    st.success("Registration successful! You can now log in.")
                elif code == 0:
                    st.error(data.get("error", "Connection failed."))
                else:
                    st.error(data.get("error", data) if isinstance(data, dict) else str(data))


def render_client_login():
    st.subheader("Login")
    with st.form("login_form"):
        device_id = st.text_input("Device ID", value=st.session_state.get("device_id", ""))
        password = st.text_input("Password", type="password")
        submitted = st.form_submit_button("Login")
        if submitted:
            if not device_id or not password:
                st.error("Device ID and Password are required.")
            else:
                data, code = client_login(device_id, password)
                if code == 200 and "access_token" in data:
                    st.session_state.access_token = data["access_token"]
                    st.session_state.device_id = device_id
                    st.session_state.pending_order_id = None
                    st.session_state.pending_items = None
                    st.success("Logged in successfully!")
                    st.rerun()
                elif code == 0:
                    st.error(data.get("error", "Connection failed."))
                else:
                    st.error("Invalid credentials.")


def render_grocery_order():
    st.subheader("Place Grocery Order")
    st.caption("Select items from the five aisles. Order at least one item.")

    cart = {}
    for aisle in AISLE_TYPES:
        items = ITEMS_BY_AISLE[aisle]
        with st.expander(f"🥖 {aisle}", expanded=(aisle == "Produce")):
            cols = st.columns(len(items))
            for i, item in enumerate(items):
                with cols[i]:
                    qty = st.number_input(
                        item["name"],
                        min_value=0,
                        max_value=50,
                        value=0,
                        key=f"order_{item['sku']}",
                    )
                    if qty > 0:
                        cart[item["sku"]] = qty

    if not cart:
        st.info("Add at least one item to your cart.")
        return

    col1, col2, col3 = st.columns([1, 1, 2])
    with col1:
        if st.button("Preview & Reserve", use_container_width=True):
            items_list = [{"sku": sku, "quantity": qty} for sku, qty in cart.items()]
            data, code = order_preview(st.session_state.access_token, items_list)
            if code in (200, 201):
                order_id = data.get("order_id") or data.get("OrderId")
                st.session_state.pending_order_id = order_id
                st.session_state.pending_items = data.get("items", cart)
                st.success(f"Reserved! Order ID: {order_id or 'N/A'}")
                st.rerun()
            elif code == 0:
                st.error(data.get("error", "Connection failed."))
            else:
                st.error(data.get("error", str(data)) if isinstance(data, dict) else "Reservation failed.")

    if st.session_state.pending_order_id:
        with col2:
            if st.button("Confirm Order", type="primary", use_container_width=True):
                data, code = order_confirm(st.session_state.access_token, st.session_state.pending_order_id)
                if code == 200:
                    total = data.get("total_price") or data.get("TotalPrice") or 0
                    st.session_state.pending_order_id = None
                    st.session_state.pending_items = None
                    st.success(f"Order completed! Total: ${total:.2f}")
                    st.balloons()
                    st.rerun()
                elif code == 0:
                    st.error(data.get("error", "Connection failed."))
                else:
                    st.error(data.get("error", str(data)) if isinstance(data, dict) else "Confirm failed.")
        with col3:
            if st.button("Cancel Order", use_container_width=True):
                data, code = order_cancel(st.session_state.access_token, st.session_state.pending_order_id)
                if code == 200:
                    st.session_state.pending_order_id = None
                    st.session_state.pending_items = None
                    st.info("Order cancelled.")
                    st.rerun()
                elif code == 0:
                    st.error(data.get("error", "Connection failed."))

        if st.session_state.pending_items:
            st.markdown("**Reserved items:**")
            for sku, qty in st.session_state.pending_items.items():
                st.write(f"- {sku}: {qty}")


def render_order_history():
    st.subheader("Order History")
    data, code = order_history(st.session_state.access_token)
    if code == 0:
        st.error(data.get("error", "Connection failed."))
        return
    if code != 200:
        st.warning("Could not load history.")
        return
    history = data.get("data") or []
    if not history:
        st.info("No orders yet.")
        return
    for o in history:
        order_id = o.get("order_id") or o.get("OrderID", "N/A")
        status = o.get("status") or o.get("Status", "")
        total = o.get("total_price") or o.get("TotalPrice") or 0
        created = o.get("created_at") or o.get("CreatedAt", "")
        with st.container():
            st.markdown(f"""
            <div class="order-card">
                <strong>Order {order_id}</strong> — 
                Status: {status} — 
                Total: ${float(total):.2f} — 
                {created}
            </div>
            """, unsafe_allow_html=True)


# ---------- Truck (Restock) ----------

def render_truck_restock():
    st.subheader("Truck Restock")
    st.caption("Submit a restock order. Truck will be auto-registered if needed.")

    with st.form("restock_form"):
        st.markdown("**Truck Info**")
        t1, t2 = st.columns(2)
        with t1:
            truck_id = st.text_input("Truck ID", placeholder="TRUCK-001")
            plate = st.text_input("Plate Number", placeholder="ABC-1234")
            driver = st.text_input("Driver Name", placeholder="John Smith")
        with t2:
            supplier_id = st.text_input("Supplier ID", placeholder="SUP-001")
            contact = st.text_input("Contact Info", placeholder="Optional")
            location = st.text_input("Location", placeholder="Loading Dock A")

        st.markdown("**Items to Unload**")
        st.caption("Add items with SKU, name, aisle, quantity, dates, and unit cost.")

        items_data = []
        n_items = st.number_input("Number of item types", min_value=1, max_value=20, value=3)
        for i in range(int(n_items)):
            c1, c2, c3, c4 = st.columns(4)
            with c1:
                sku = st.text_input(f"SKU {i+1}", placeholder="BREAD-WHITE", key=f"r_sku_{i}")
                aisle = st.selectbox(
                    f"Aisle {i+1}",
                    AISLE_TYPES,
                    key=f"r_aisle_{i}",
                )
                qty = st.number_input(f"Qty {i+1}", min_value=1, value=1, key=f"r_qty_{i}")
            with c2:
                name = st.text_input(f"Name {i+1}", placeholder="White Bread", key=f"r_name_{i}")
                mfd = st.text_input(f"Mfd Date {i+1}", placeholder="2025-01-01", key=f"r_mfd_{i}")
            with c3:
                expiry = st.text_input(f"Expiry {i+1}", placeholder="2025-06-01", key=f"r_exp_{i}")
            with c4:
                unit_cost = st.number_input(f"Unit Cost ${i+1}", min_value=0.0, value=1.99, step=0.01, key=f"r_cost_{i}")

            if sku and name:
                items_data.append({
                    "sku": sku,
                    "name": name,
                    "aisle_type": aisle,
                    "quantity": int(qty),
                    "mfd_date": mfd or "",
                    "expiry_date": expiry or "",
                    "unit_cost": float(unit_cost),
                })

        submitted = st.form_submit_button("Submit Restock")
        if submitted:
            if not truck_id or not plate or not driver:
                st.error("Truck ID, Plate Number, and Driver Name are required.")
            elif not supplier_id:
                st.error("Supplier ID is required.")
            elif not items_data:
                st.error("Add at least one item.")
            else:
                data, code = truck_restock(
                    truck_id=truck_id,
                    plate_number=plate,
                    driver_name=driver,
                    supplier_id=supplier_id,
                    items=items_data,
                    contact_info=contact,
                    location=location,
                )
                if code == 201:
                    st.success(f"Restock accepted! Order ID: {data.get('order_id', 'N/A')}")
                elif code == 0:
                    st.error(data.get("error", "Connection failed."))
                else:
                    st.error(data.get("error", str(data)) if isinstance(data, dict) else "Restock failed.")


def render_truck_register():
    st.subheader("Register Truck (Optional)")
    with st.form("truck_register_form"):
        truck_id = st.text_input("Truck ID", placeholder="TRUCK-001")
        plate = st.text_input("Plate Number", placeholder="ABC-1234")
        driver = st.text_input("Driver Name", placeholder="John Smith")
        submitted = st.form_submit_button("Register")
        if submitted:
            if truck_id and plate and driver:
                data, code = truck_register(truck_id, plate, driver)
                if code == 201:
                    st.success("Truck registered!")
                elif code == 0:
                    st.error(data.get("error", "Connection failed."))
                else:
                    st.error(str(data))


# ---------- Main ----------

def main():
    init_session()

    st.markdown('<p class="main-header">🛒 Auto-Grocery</p>', unsafe_allow_html=True)
    st.markdown(
        '<p class="sub-header">Smart Refrigerator Orders & Truck Restocking</p>',
        unsafe_allow_html=True,
    )

    mode = st.sidebar.radio(
        "Select Client",
        ["🥶 Smart Refrigerator", "🚚 Truck Restock"],
        label_visibility="collapsed",
    )

    if mode == "🥶 Smart Refrigerator":
        if st.session_state.access_token:
            tab1, tab2, tab3 = st.tabs(["Place Order", "Order History", "Logout"])
            with tab1:
                render_grocery_order()
            with tab2:
                render_order_history()
            with tab3:
                if st.button("Logout"):
                    st.session_state.access_token = None
                    st.session_state.device_id = None
                    st.session_state.pending_order_id = None
                    st.session_state.pending_items = None
                    st.rerun()
        else:
            tab1, tab2 = st.tabs(["Login", "Register"])
            with tab1:
                render_client_login()
            with tab2:
                render_client_register()

    else:  # Truck
        tab1, tab2 = st.tabs(["Submit Restock", "Register Truck"])
        with tab1:
            render_truck_restock()
        with tab2:
            render_truck_register()


if __name__ == "__main__":
    main()
