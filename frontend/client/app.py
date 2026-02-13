import streamlit as st
import requests
import time

# --- CONFIGURATION ---
BASE_URL = "http://localhost:5050"

# --- PAGE SETUP ---
st.set_page_config(page_title="Robot Mart | Client", page_icon="🤖", layout="centered")

# --- COOL ORANGE & DARK THEME CSS ---
st.markdown("""
    <style>
    .stApp { background-color: #0E1117; color: #FFFFFF; }
    /* Orange Buttons */
    div.stButton > button:first-child {
        background-color: #FF4B4B; /* Streamlit's primary red-orange */
        background-image: linear-gradient(to right, #FF8C00, #FF4500);
        color: white; border-radius: 10px; border: none;
        padding: 0.5rem 1rem; font-weight: bold; width: 100%;
    }
    /* Hover effect */
    div.stButton > button:first-child:hover { border: 1px solid white; opacity: 0.9; }
    /* Form Borders & Inputs */
    [data-testid="stForm"] { border: 1px solid #FF8C00; border-radius: 10px; background-color: #161B22; }
    h1, h2, h3 { color: #FF8C00 !important; font-family: 'Courier New', Courier, monospace; }
    /* Sidebar styling */
    [data-testid="stSidebar"] { background-color: #000000; border-right: 1px solid #FF8C00; }
    </style>
    """, unsafe_allow_html=True)

# --- SESSION STATE ---
if 'token' not in st.session_state: st.session_state.token = None
if 'cart_items' not in st.session_state: st.session_state.cart_items = [{"sku": "", "qty": 1}]
if 'order_id' not in st.session_state: st.session_state.order_id = None
if 'confirmed_items' not in st.session_state: st.session_state.confirmed_items = {}

st.title("🤖 MART FOR ROBOTS")
st.write("### `AUTONOMOUS LOGISTICS TERMINAL`")

# --- AUTHENTICATION ---
if not st.session_state.token:
    choice = st.radio("SYSTEM ACCESS", ["LOGIN", "REGISTER"], horizontal=True)
    with st.form("auth"):
        d_id = st.text_input("DEVICE ID")
        pwd = st.text_input("PASSWORD", type="password")
        if choice == "REGISTER":
            email = st.text_input("EMAIL")
            phone = st.text_input("PHONE")
        
        if st.form_submit_button("ESTABLISH UPLINK"):
            if choice == "LOGIN":
                res = requests.post(f"{BASE_URL}/api/client/login", json={"device_id": d_id, "password": pwd})
                if res.status_code == 200:
                    st.session_state.token = res.json().get("access_token")
                    st.rerun()
                else: st.error("❌ Authentication Refused.")
            else:
                res = requests.post(f"{BASE_URL}/api/client/register", 
                                 json={"device_id": d_id, "email": email, "phone": phone, "password": pwd})
                if res.status_code == 201: st.success("✅ Device Registered. Please Login.")
                else: st.error(f"❌ Error: {res.text}")

# --- ORDERING INTERFACE ---
else:
    st.sidebar.markdown("### `STATUS: CONNECTED`")
    if st.sidebar.button("EXIT SYSTEM"):
        st.session_state.clear()
        st.rerun()

    st.header("🛒 Manifest Entry")
    for i, item in enumerate(st.session_state.cart_items):
        c1, c2 = st.columns([3, 1])
        st.session_state.cart_items[i]["sku"] = c1.text_input(f"SKU CODE", value=item["sku"], key=f"s_{i}", placeholder="e.g. APPLE-101")
        st.session_state.cart_items[i]["qty"] = c2.number_input(f"QTY", min_value=1, value=item["qty"], key=f"q_{i}")

    # Row Logic
    b1, b2, _ = st.columns([1, 1, 2])
    if b1.button("➕ ROW"):
        st.session_state.cart_items.append({"sku": "", "qty": 1})
        st.rerun()
    if b2.button("➖ ROW") and len(st.session_state.cart_items) > 1:
        st.session_state.cart_items.pop()
        st.rerun()

    st.markdown("---")

    # --- PREVIEW ---
    if st.button("🔍 INITIATE STOCK SCAN"):
        headers = {"Authorization": f"Bearer {st.session_state.token}"}
        payload = [{"sku": i["sku"], "quantity": int(i["qty"])} for i in st.session_state.cart_items if i["sku"]]
        
        try:
            res = requests.post(f"{BASE_URL}/api/client/order/preview", json={"items": payload}, headers=headers)
            if res.status_code == 201:
                data = res.json()
                st.session_state.order_id = data.get("order_id")
                
                st.session_state.confirmed_items = data.get("items") or {} 
                
                st.warning(f"SCAN SUCCESSFUL | ORDER ID: {st.session_state.order_id}")
                
                if st.session_state.confirmed_items:
                    for sku, qty in st.session_state.confirmed_items.items():
                        st.write(f"🔸 **{sku}**: {qty} units available")
                else:
                    st.error("⚠️ Stock Scan returned 0 items. Please check Inventory.")
            else:
                st.error(f"⚠️ Stock Scan Failed: {res.text}")
        except Exception as e:
            st.error(f"Connection Error: {str(e)}")

    # --- CONFIRM ---
    if st.session_state.order_id:
        if st.button("🚀 DISPATCH ROBOT SQUADRON"):
            headers = {"Authorization": f"Bearer {st.session_state.token}"}
            payload = {"order_id": st.session_state.order_id, "items": st.session_state.confirmed_items}
            
            try:
                res = requests.post(f"{BASE_URL}/api/client/order/confirm", json=payload, headers=headers)
                
                if res.status_code == 200:
                    with st.status("Robots deployed to warehouse floor...", expanded=True) as s:
                        while True:
                            poll = requests.get(f"{BASE_URL}/api/client/orders/last", headers=headers)
                            if poll.status_code == 200:
                                order_data = poll.json().get("data", {})
                                order_status = order_data.get("Status")
                                s.write(f"Robot Telemetry: `{order_status}`")
                                
                                if order_status == "COMPLETED":
                                    # --- NEW: DISPLAY PRICE & RECEIPT ---
                                    final_price = order_data.get("TotalPrice", 0.0)
                                    s.update(label=f"✅ SEQUENCE SUCCESS | FINAL CHARGE: ${final_price}", state="complete")
                                    st.balloons()
                                    
                                    st.markdown("### 🧾 DIGITAL RECEIPT")
                                    st.success(f"**Total Amount Charged:** ${final_price}")
                                    st.json(order_data) # Show full details
                                    
                                    time.sleep(10) # Wait 10s so you can see the receipt
                                    # ------------------------------------
                                    
                                    st.session_state.order_id = None
                                    st.session_state.confirmed_items = {} # Reset
                                    st.rerun() # Refresh to clear form
                                    break
                                elif "FAILED" in str(order_status): 
                                    s.update(label="❌ SEQUENCE CRITICAL FAILURE", state="error")
                                    break
                            time.sleep(2)
                else: st.error("❌ Signal Lost: Robot Dispatch Failed.")
            except Exception as e:
                st.error(f"Dispatch Error: {str(e)}")