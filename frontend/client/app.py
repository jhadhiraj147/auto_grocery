import streamlit as st
import requests
import time
import json
from pathlib import Path

try:
    from streamlit_autorefresh import st_autorefresh
except Exception:
    st_autorefresh = None

# --- CONFIGURATION ---
BASE_URL = "http://localhost:5050"
AUTH_CACHE_FILE = Path(__file__).parent / ".client_auth_cache.json"
AUTO_RERUN_MS = 60_000
TOKEN_REFRESH_INTERVAL_SEC = 240
REQUEST_TIMEOUT_SEC = 5


def load_auth_cache():
    try:
        if AUTH_CACHE_FILE.exists():
            return json.loads(AUTH_CACHE_FILE.read_text())
    except Exception as err:
        print(f"[client-ui] failed to load auth cache err={err}")
    return {}


def save_auth_cache(payload):
    try:
        AUTH_CACHE_FILE.write_text(json.dumps(payload))
    except Exception as err:
        print(f"[client-ui] failed to save auth cache err={err}")


def clear_auth_cache():
    try:
        if AUTH_CACHE_FILE.exists():
            AUTH_CACHE_FILE.unlink()
    except Exception as err:
        print(f"[client-ui] failed to clear auth cache err={err}")

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
if 'refresh_token' not in st.session_state: st.session_state.refresh_token = None
if 'device_id' not in st.session_state: st.session_state.device_id = None
if 'cart_items' not in st.session_state: st.session_state.cart_items = [{"sku": "", "qty": 1}]
if 'order_id' not in st.session_state: st.session_state.order_id = None
if 'confirmed_items' not in st.session_state: st.session_state.confirmed_items = {}
if 'auth_restored' not in st.session_state: st.session_state.auth_restored = False
if 'last_refresh_ts' not in st.session_state: st.session_state.last_refresh_ts = 0.0


def refresh_access_token():
    refresh_token = st.session_state.refresh_token
    if not refresh_token:
        return False
    headers = {"Authorization": f"Bearer {refresh_token}"}
    try:
        res = requests.post(f"{BASE_URL}/api/client/refresh", headers=headers, timeout=REQUEST_TIMEOUT_SEC)
    except requests.RequestException as err:
        print(f"[client-ui] refresh request failed err={err}")
        return False
    print(f"[client-ui] refresh response status={res.status_code} body={res.text}")
    if res.status_code == 200:
        data = res.json()
        st.session_state.token = data.get("access_token")
        st.session_state.last_refresh_ts = time.time()
        save_auth_cache({
            "device_id": st.session_state.device_id,
            "refresh_token": st.session_state.refresh_token,
        })
        return True
    return False


def maybe_auto_refresh_access_token():
    if not st.session_state.token or not st.session_state.refresh_token:
        return
    now = time.time()
    if now - st.session_state.last_refresh_ts < TOKEN_REFRESH_INTERVAL_SEC:
        return
    print("[client-ui] periodic token refresh triggered")
    ok = refresh_access_token()
    if not ok:
        print("[client-ui] periodic token refresh failed")


def auth_request(method, path, payload=None):
    headers = {}
    if st.session_state.token:
        headers["Authorization"] = f"Bearer {st.session_state.token}"

    try:
        if method == "GET":
            res = requests.get(f"{BASE_URL}{path}", headers=headers, timeout=REQUEST_TIMEOUT_SEC)
        else:
            res = requests.post(f"{BASE_URL}{path}", json=payload, headers=headers, timeout=REQUEST_TIMEOUT_SEC)
    except requests.RequestException as err:
        print(f"[client-ui] request failed path={path} err={err}")
        return None

    if res.status_code == 401 and st.session_state.refresh_token:
        print(f"[client-ui] {path} returned 401, attempting token refresh")
        if refresh_access_token():
            headers["Authorization"] = f"Bearer {st.session_state.token}"
            try:
                if method == "GET":
                    res = requests.get(f"{BASE_URL}{path}", headers=headers, timeout=REQUEST_TIMEOUT_SEC)
                else:
                    res = requests.post(f"{BASE_URL}{path}", json=payload, headers=headers, timeout=REQUEST_TIMEOUT_SEC)
            except requests.RequestException as err:
                print(f"[client-ui] retry request failed path={path} err={err}")
                return None
    return res


if not st.session_state.auth_restored:
    st.session_state.auth_restored = True
    cached = load_auth_cache()
    cached_refresh = cached.get("refresh_token")
    cached_device = cached.get("device_id")
    if cached_refresh:
        st.session_state.refresh_token = cached_refresh
        st.session_state.device_id = cached_device
        print(f"[client-ui] restoring session from cache device_id={cached_device}")
        if refresh_access_token():
            print(f"[client-ui] session restored successfully device_id={cached_device}")
        else:
            print("[client-ui] session restore failed, clearing cache")
            st.session_state.refresh_token = None
            st.session_state.device_id = None
            clear_auth_cache()

st.title("🤖 MART FOR ROBOTS")
st.write("### `AUTONOMOUS LOGISTICS TERMINAL`")

if st_autorefresh is not None:
    st_autorefresh(interval=AUTO_RERUN_MS, key="client_auto_rerun")

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
                print(f"[client-ui] login attempt device_id={d_id}")
                try:
                    res = requests.post(
                        f"{BASE_URL}/api/client/login",
                        json={"device_id": d_id, "password": pwd},
                        timeout=REQUEST_TIMEOUT_SEC,
                    )
                except requests.RequestException as err:
                    print(f"[client-ui] login request failed err={err}")
                    st.error("❌ Backend unreachable. Please start ordering service.")
                    res = None
                if res is None:
                    st.stop()
                if res.status_code == 200:
                    data = res.json()
                    st.session_state.token = data.get("access_token")
                    st.session_state.refresh_token = data.get("refresh_token")
                    st.session_state.device_id = d_id
                    save_auth_cache({
                        "device_id": d_id,
                        "refresh_token": st.session_state.refresh_token,
                    })
                    print(f"[client-ui] login success device_id={d_id}")
                    st.rerun()
                else:
                    print(f"[client-ui] login failed device_id={d_id} status={res.status_code} body={res.text}")
                    st.error("❌ Authentication Refused.")
            else:
                print(f"[client-ui] register attempt device_id={d_id}")
                try:
                    res = requests.post(
                        f"{BASE_URL}/api/client/register",
                        json={"device_id": d_id, "email": email, "phone": phone, "password": pwd},
                        timeout=REQUEST_TIMEOUT_SEC,
                    )
                except requests.RequestException as err:
                    print(f"[client-ui] register request failed err={err}")
                    st.error("❌ Backend unreachable. Please start ordering service.")
                    res = None
                if res is None:
                    st.stop()
                if res.status_code == 201: st.success("✅ Device Registered. Please Login.")
                else:
                    print(f"[client-ui] register failed device_id={d_id} status={res.status_code} body={res.text}")
                    st.error(f"❌ Error: {res.text}")

# --- ORDERING INTERFACE ---
else:
    maybe_auto_refresh_access_token()
    st.sidebar.markdown("### `STATUS: CONNECTED`")
    st.sidebar.caption(f"Device: {st.session_state.device_id or '-'}")
    if st.sidebar.button("🔄 REFRESH ACCESS TOKEN"):
        if refresh_access_token():
            st.sidebar.success("Access token refreshed")
        else:
            st.sidebar.error("Refresh failed. Please login again.")
    if st.sidebar.button("EXIT SYSTEM"):
        st.session_state.clear()
        clear_auth_cache()
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
        payload = [{"sku": i["sku"], "quantity": int(i["qty"])} for i in st.session_state.cart_items if i["sku"]]
        print(f"[client-ui] preview request payload={payload}")
        
        try:
            res = auth_request("POST", "/api/client/order/preview", {"items": payload})
            if res is None:
                st.error("Connection Error: Backend unreachable.")
                st.stop()
            print(f"[client-ui] preview response status={res.status_code} body={res.text}")
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
            print(f"[client-ui] preview exception err={e}")
            st.error(f"Connection Error: {str(e)}")

    # --- CONFIRM ---
    if st.session_state.order_id:
        if st.button("🗑️ CANCEL CURRENT ORDER"):
            cancel_payload = {"order_id": st.session_state.order_id}
            try:
                cancel_res = auth_request("POST", "/api/client/order/cancel", cancel_payload)
                if cancel_res is None:
                    st.error("Cancel Error: Backend unreachable.")
                    st.stop()
                print(f"[client-ui] cancel response status={cancel_res.status_code} body={cancel_res.text}")
                if cancel_res.status_code == 200:
                    st.success("Order cancelled and reservation released")
                    st.session_state.order_id = None
                    st.session_state.confirmed_items = {}
                    st.rerun()
                else:
                    st.error(f"Cancel failed: {cancel_res.text}")
            except Exception as e:
                st.error(f"Cancel Error: {str(e)}")

        if st.button("🚀 DISPATCH ROBOT SQUADRON"):
            payload = {"order_id": st.session_state.order_id, "items": st.session_state.confirmed_items}
            print(f"[client-ui] confirm request order={st.session_state.order_id} items={st.session_state.confirmed_items}")
            
            try:
                res = auth_request("POST", "/api/client/order/confirm", payload)
                if res is None:
                    st.error("Dispatch Error: Backend unreachable.")
                    st.stop()
                print(f"[client-ui] confirm response status={res.status_code} body={res.text}")
                
                if res.status_code == 200:
                    with st.status("Robots deployed to warehouse floor...", expanded=True) as s:
                        while True:
                            poll = auth_request("GET", "/api/client/orders/last")
                            if poll is None:
                                s.update(label="❌ BACKEND UNREACHABLE", state="error")
                                break
                            print(f"[client-ui] poll status_code={poll.status_code}")
                            if poll.status_code == 200:
                                order_data = poll.json().get("data", {})
                                order_status = order_data.get("Status")
                                print(f"[client-ui] poll order={order_data.get('OrderID')} status={order_status}")
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
                print(f"[client-ui] dispatch exception err={e}")
                st.error(f"Dispatch Error: {str(e)}")

    st.markdown("---")
    st.subheader("📚 Order History")
    if st.button("LOAD ORDER HISTORY"):
        try:
            history_res = auth_request("GET", "/api/client/orders")
            if history_res is None:
                st.error("History Error: Backend unreachable.")
                st.stop()
            print(f"[client-ui] history response status={history_res.status_code} body={history_res.text}")
            if history_res.status_code == 200:
                history_data = history_res.json().get("data", [])
                if history_data:
                    st.dataframe(history_data, use_container_width=True)
                else:
                    st.info("No past orders found")
            else:
                st.error(f"History fetch failed: {history_res.text}")
        except Exception as e:
            st.error(f"History Error: {str(e)}")