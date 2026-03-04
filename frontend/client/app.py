import streamlit as st
import requests
import time
import json
import re
from pathlib import Path

try:
    from streamlit_autorefresh import st_autorefresh
except Exception:
    st_autorefresh = None

# ── CONFIGURATION ─────────────────────────────────────────────────────────────
BASE_URL                   = "http://127.0.0.1:5050"
AUTH_CACHE_FILE            = Path(__file__).parent / ".client_auth_cache.json"
AUTO_RERUN_MS              = 60_000
TOKEN_REFRESH_INTERVAL_SEC = 720   # access token lifetime 15 min → refresh at 12 min
REQUEST_TIMEOUT_SEC        = 5

# ── PAGE CONFIG ───────────────────────────────────────────────────────────────
st.set_page_config(
    page_title="AutoGrocery | Smart Cart",
    page_icon="🛒",
    layout="wide",
    initial_sidebar_state="collapsed",
)

# ── THEME / CSS ───────────────────────────────────────────────────────────────
st.markdown("""
<style>
/* Base */
html, body, [data-testid="stAppViewContainer"], [data-testid="stApp"] {
    background-color: #0A0F1E;
    color: #E2E8F0;
    font-family: 'Inter', 'Segoe UI', system-ui, sans-serif;
}

/* Sidebar */
[data-testid="stSidebar"] {
    background: linear-gradient(180deg, #0F172A 0%, #1E1B4B 100%);
    border-right: 1px solid #2E3799;
}
[data-testid="stSidebar"] * { color: #C7D2FE !important; }

/* Buttons */
div.stButton > button {
    background: linear-gradient(135deg, #4F46E5 0%, #7C3AED 100%);
    color: #FFFFFF !important;
    border: none;
    border-radius: 8px;
    padding: 0.5rem 1.2rem;
    font-weight: 600;
    font-size: 0.875rem;
    letter-spacing: 0.02em;
    transition: opacity 0.2s, transform 0.1s;
    width: 100%;
}
div.stButton > button:hover  { opacity: 0.85; transform: translateY(-1px); border: none; }
div.stButton > button:active { transform: translateY(0px); }
div.stButton > button:disabled {
    background: #1F2937 !important;
    color: #4B5563 !important;
    cursor: not-allowed;
}

/* Inputs */
input, textarea,
[data-testid="stTextInput"] input,
[data-baseweb="input"] input {
    background-color: #1F2937 !important;
    border: 1px solid #374151 !important;
    border-radius: 8px !important;
    color: #F1F5F9 !important;
}
input:focus {
    border-color: #6366F1 !important;
    box-shadow: 0 0 0 2px rgba(99,102,241,0.2) !important;
}

[data-testid="stNumberInput"] input {
    background-color: #1F2937 !important;
    border: 1px solid #374151 !important;
    color: #F1F5F9 !important;
}

/* Form container */
[data-testid="stForm"] {
    background: #111827;
    border: 1px solid #2E3799;
    border-radius: 12px;
    padding: 1.5rem;
}

/* Tabs */
button[data-baseweb="tab"] {
    background: transparent !important;
    color: #9CA3AF !important;
    font-weight: 500;
}
button[data-baseweb="tab"][aria-selected="true"] {
    color: #818CF8 !important;
    border-bottom: 2px solid #6366F1 !important;
}

/* Metrics */
[data-testid="stMetric"] {
    background: #111827;
    border: 1px solid #1F2937;
    border-radius: 10px;
    padding: 0.9rem;
}
[data-testid="stMetricLabel"] { color: #9CA3AF !important; font-size: 0.78rem !important; }
[data-testid="stMetricValue"] { color: #818CF8 !important; font-weight: 700 !important; }

/* Dataframe */
[data-testid="stDataFrame"] { border-radius: 10px; overflow: hidden; }

/* Dividers */
hr { border-color: #1F2937 !important; margin: 0.6rem 0; }

/* Headings — normalize sizes and remove anchor link icon */
h1 { color: #818CF8 !important; font-weight: 700; letter-spacing: -0.02em; margin-top: 0 !important; }
h2 { color: #A5B4FC !important; font-weight: 700; font-size: 1.55rem !important; margin-top: 0 !important; margin-bottom: 0.15rem !important; }
h3 { color: #C7D2FE !important; font-weight: 600; font-size: 0.95rem !important; margin-top: 0 !important; }
/* Hide the anchor icon Streamlit appends to headings */
h1 a, h2 a, h3 a { display: none !important; }

/* ── Crush Streamlit's default top gap ─────────────────────────────────── */
[data-testid="block-container"] {
    padding-top: 1rem !important;
    padding-bottom: 1rem !important;
}
[data-testid="stVerticalBlock"] { gap: 0.35rem !important; }
.element-container { margin-bottom: 0 !important; }
/* tighten column gaps */
[data-testid="stHorizontalBlock"] { gap: 0.5rem !important; align-items: center !important; }

/* Hide Streamlit deploy button */
[data-testid="stAppDeployButton"] { display: none !important; }

/* Cards / badges */
.ag-card {
    background: #111827;
    border: 1px solid #1F2937;
    border-radius: 12px;
    padding: 1.25rem 1.5rem;
    margin-bottom: 1rem;
}
.ag-badge {
    display: inline-block;
    padding: 2px 10px;
    border-radius: 20px;
    font-size: 0.72rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
}
.ag-badge-green  { background: #064E3B; color: #6EE7B7; }
.ag-badge-red    { background: #450A0A; color: #FCA5A5; }
.ag-badge-yellow { background: #451A03; color: #FCD34D; }
.ag-badge-blue   { background: #1E1B4B; color: #A5B4FC; }

/* Hide sidebar toggle arrow */
[data-testid="collapsedControl"] { display: none !important; }
/* ── Topbar sign-out pill ────────────────────────────────────────── */
.ag-topbar-right { display:flex; align-items:center; justify-content:flex-end; gap:8px; padding-top:3px; }
.ag-topbar-right div.stButton { display:inline-flex !important; width:auto !important; }
.ag-topbar-right div.stButton > button {
    background: linear-gradient(135deg,#4C0519,#7F1D1D) !important;
    border: 1px solid #BE123C !important;
    color: #FDA4AF !important;
    font-size: 0.72rem !important;
    font-weight: 700 !important;
    padding: 0 12px !important;
    height: 26px !important;
    line-height: 26px !important;
    width: auto !important;
    border-radius: 20px !important;
    letter-spacing: 0.06em !important;
    text-transform: uppercase !important;
    box-shadow: 0 0 8px rgba(190,18,60,0.3) !important;
}
.ag-topbar-right div.stButton > button:hover {
    background: linear-gradient(135deg,#6B0F28,#991B1B) !important;
    box-shadow: 0 0 14px rgba(239,68,68,0.45) !important;
    opacity: 1 !important;
    transform: none !important;
    border-color: #F43F5E !important;
    color: #FECDD3 !important;
}
/* vertically center content inside topbar columns */
.ag-tb-cell > div[data-testid="stVerticalBlock"] {
    display:flex; align-items:center; justify-content:center; height:36px;
}
.ag-tb-cell div.stButton > button {
    background: linear-gradient(135deg,#4C0519,#7F1D1D) !important;
    border: 1px solid #BE123C !important;
    color: #FDA4AF !important;
    font-size: 0.72rem !important;
    font-weight: 700 !important;
    padding: 0 14px !important;
    height: 26px !important;
    min-height: 26px !important;
    line-height: 1 !important;
    width: auto !important;
    border-radius: 20px !important;
    letter-spacing: 0.06em !important;
    text-transform: uppercase !important;
    box-shadow: 0 0 8px rgba(190,18,60,0.3) !important;
}
.ag-tb-cell div.stButton > button:hover {
    background: linear-gradient(135deg,#6B0F28,#991B1B) !important;
    box-shadow: 0 0 14px rgba(239,68,68,0.45) !important;
    opacity: 1 !important; transform: none !important;
    border-color: #F43F5E !important; color: #FECDD3 !important;
}

/* ── Status card animations ────────────────────────────────────────── */
.ag-status-card {
    background: #111827;
    border: 1px solid #1F2937;
    border-radius: 12px;
    padding: 1.2rem 0.9rem;
    text-align: center;
    position: relative;
    overflow: hidden;
}
/* sonar rings (scanning) */
@keyframes ag-sonar {
    0%   { transform: translate(-50%,-50%) scale(0.4); opacity: 0.9; }
    100% { transform: translate(-50%,-50%) scale(2.6); opacity: 0; }
}
.ag-sonar-wrap {
    position: relative;
    width: 44px; height: 44px;
    margin: 0.4rem auto 0.6rem;
}
.ag-ring {
    position: absolute;
    border: 2px solid #6366F1;
    border-radius: 50%;
    width: 44px; height: 44px;
    top: 50%; left: 50%;
    animation: ag-sonar 1.8s ease-out infinite;
}
.ag-ring:nth-child(2) { animation-delay: 0.55s; }
.ag-ring:nth-child(3) { animation-delay: 1.1s; }
/* pulsing yellow dot (pending) */
@keyframes ag-pulse {
    0%,100% { box-shadow: 0 0 0 0 rgba(250,204,21,0.55); }
    50%     { box-shadow: 0 0 0 12px rgba(250,204,21,0); }
}
.ag-pulse-dot {
    width: 14px; height: 14px;
    background: #FACC15;
    border-radius: 50%;
    margin: 0.5rem auto 0.45rem;
    animation: ag-pulse 1.4s ease-in-out infinite;
}
/* bouncing robot (dispatching) */
@keyframes ag-bounce {
    0%,100% { transform: translateY(0); }
    40%     { transform: translateY(-12px); }
    65%     { transform: translateY(-5px); }
}
.ag-robot-bounce {
    display: inline-block;
    font-size: 2.2rem;
    animation: ag-bounce 1s ease-in-out infinite;
}
/* spinning gear (cancelling) */
@keyframes ag-spin {
    from { transform: rotate(0deg); }
    to   { transform: rotate(360deg); }
}
.ag-spin-icon {
    display: inline-block;
    font-size: 1.8rem;
    animation: ag-spin 0.9s linear infinite;
    margin: 0.4rem 0;
}
/* blinking dots */
@keyframes ag-blink {
    0%,80%,100% { opacity: 0.1; }
    40%         { opacity: 1; }
}
.ag-dots { margin-top: 0.4rem; }
.ag-dots span {
    display: inline-block;
    width: 5px; height: 5px;
    background: currentColor;
    border-radius: 50%;
    margin: 0 2px;
    animation: ag-blink 1.3s infinite;
}
.ag-dots span:nth-child(2) { animation-delay: 0.22s; }
.ag-dots span:nth-child(3) { animation-delay: 0.44s; }
/* done checkmark pop-in */
@keyframes ag-pop {
    0%   { transform: scale(0);   opacity: 0; }
    70%  { transform: scale(1.25); }
    100% { transform: scale(1);   opacity: 1; }
}
.ag-done-icon {
    font-size: 2rem;
    display: block;
    margin: 0.2rem 0 0.4rem;
    animation: ag-pop 0.45s cubic-bezier(.36,.07,.19,.97) forwards;
}
</style>
""", unsafe_allow_html=True)

# ── SESSION STATE ─────────────────────────────────────────────────────────────
_defaults = {
    "token": None, "refresh_token": None, "device_id": None,
    "cart_items": [{"sku": "", "qty": 1}],
    "order_id": None, "cart_snapshot": None, "confirmed_items": {},
    "auth_restored": False, "last_refresh_ts": 0.0, "is_dispatching": False, "last_receipt": None,
    "status_anim": None, "_trigger_scan": False, "_trigger_cancel": False, "_trigger_dispatch": False,
}
for _k, _v in _defaults.items():
    if _k not in st.session_state:
        st.session_state[_k] = _v

# ── AUTH CACHE ────────────────────────────────────────────────────────────────
def load_auth_cache():
    try:
        if AUTH_CACHE_FILE.exists():
            return json.loads(AUTH_CACHE_FILE.read_text())
    except Exception as err:
        print(f"[client-ui] load_auth_cache err={err}")
    return {}

def save_auth_cache(payload):
    try:
        AUTH_CACHE_FILE.write_text(json.dumps(payload))
    except Exception as err:
        print(f"[client-ui] save_auth_cache err={err}")

def clear_auth_cache():
    try:
        if AUTH_CACHE_FILE.exists():
            AUTH_CACHE_FILE.unlink()
    except Exception as err:
        print(f"[client-ui] clear_auth_cache err={err}")

# ── TOKEN MANAGEMENT ──────────────────────────────────────────────────────────
def refresh_access_token() -> bool:
    if not st.session_state.refresh_token:
        return False
    try:
        res = requests.post(
            f"{BASE_URL}/api/client/refresh",
            headers={"Authorization": f"Bearer {st.session_state.refresh_token}"},
            timeout=REQUEST_TIMEOUT_SEC,
        )
    except requests.RequestException as err:
        print(f"[client-ui] refresh request failed err={err}")
        return False
    if res.status_code == 200:
        st.session_state.token           = res.json().get("access_token")
        st.session_state.last_refresh_ts = time.time()
        save_auth_cache({"device_id": st.session_state.device_id,
                         "refresh_token": st.session_state.refresh_token})
        return True
    return False

def _force_logout(reason: str):
    print(f"[client-ui] force_logout reason={reason}")
    for _k in ["token", "refresh_token", "device_id", "order_id",
               "cart_snapshot", "is_dispatching"]:
        st.session_state[_k] = None
    st.session_state.confirmed_items = {}
    st.session_state.last_refresh_ts = 0.0
    clear_auth_cache()
    st.warning("Your session has expired. Please log in again.")
    st.rerun()

def maybe_auto_refresh_access_token():
    if not st.session_state.token or not st.session_state.refresh_token:
        return
    if time.time() - st.session_state.last_refresh_ts < TOKEN_REFRESH_INTERVAL_SEC:
        return
    print("[client-ui] periodic token refresh triggered")
    if not refresh_access_token():
        _force_logout("refresh_token_expired")

def auth_request(method: str, path: str, payload=None):
    headers = {}
    if st.session_state.token:
        headers["Authorization"] = f"Bearer {st.session_state.token}"
    try:
        _fn = requests.get if method == "GET" else requests.post
        r   = _fn(f"{BASE_URL}{path}", json=payload, headers=headers,
                  timeout=REQUEST_TIMEOUT_SEC)
    except requests.RequestException as err:
        print(f"[client-ui] request failed path={path} err={err}")
        return None
    if r.status_code == 401 and st.session_state.refresh_token:
        print(f"[client-ui] {path} 401 — retrying after refresh")
        if refresh_access_token():
            headers["Authorization"] = f"Bearer {st.session_state.token}"
            try:
                _fn = requests.get if method == "GET" else requests.post
                r   = _fn(f"{BASE_URL}{path}", json=payload, headers=headers,
                          timeout=REQUEST_TIMEOUT_SEC)
            except requests.RequestException as err:
                print(f"[client-ui] retry failed path={path} err={err}")
                return None
        else:
            _force_logout("refresh_token_rejected_by_server")
    return r

# ── SESSION RESTORE ───────────────────────────────────────────────────────────
if not st.session_state.auth_restored:
    st.session_state.auth_restored = True
    _cached = load_auth_cache()
    if _cached.get("refresh_token"):
        st.session_state.refresh_token = _cached["refresh_token"]
        st.session_state.device_id     = _cached.get("device_id")
        if refresh_access_token():
            print(f"[client-ui] session restored device_id={st.session_state.device_id}")
        else:
            st.session_state.refresh_token = None
            st.session_state.device_id     = None
            clear_auth_cache()

if st_autorefresh is not None:
    st_autorefresh(interval=AUTO_RERUN_MS, key="client_auto_rerun")

# ── RECEIPT RENDERER ──────────────────────────────────────────────────────────
def _show_receipt(order_data: dict):
    fp    = order_data.get("TotalPrice", 0.0)
    oid   = order_data.get("OrderID", "—")
    items = order_data.get("Items") or {}
    st.markdown("---")
    st.markdown("### 🧾 Receipt")
    st.markdown(
        f'<div class="ag-card">'
        f'  <div style="display:flex;justify-content:space-between;align-items:center;">'
        f'    <div>'
        f'      <div style="color:#9CA3AF;font-size:0.72rem;text-transform:uppercase;'
        f'           letter-spacing:.08em">Order ID</div>'
        f'      <code style="color:#818CF8;font-size:0.78rem">{oid}</code>'
        f'    </div>'
        f'    <div style="text-align:right">'
        f'      <div style="color:#9CA3AF;font-size:0.72rem;text-transform:uppercase;'
        f'           letter-spacing:.08em">Total Charged</div>'
        f'      <div style="color:#10B981;font-size:1.8rem;font-weight:700">${fp:.2f}</div>'
        f'    </div>'
        f'  </div>'
        f'  <hr style="border-color:#1F2937;margin:0.75rem 0">'
        f'  <div style="color:#6B7280;font-size:0.75rem">'
        f'    Payment processed automatically via on-file account</div>'
        f'</div>',
        unsafe_allow_html=True,
    )
    if items:
        st.markdown("**Items delivered:**")
        _scols = st.columns(min(len(items), 5))
        for _si, (_sku, _qty) in enumerate(items.items()):
            _scols[_si % 5].metric(_sku, f"{_qty} units")

# =============================================================================
#  AUTH GATE — LOGIN / REGISTER
# =============================================================================
if not st.session_state.token:

    _, _mid, _ = st.columns([1, 2, 1])
    with _mid:
        st.markdown("<br>", unsafe_allow_html=True)
        st.markdown("# 🛒 AutoGrocery")
        st.markdown("##### Autonomous Robot Grocery Platform")
        st.markdown("---")

        _tab_in, _tab_up = st.tabs(["Sign In", "Create Account"])

        # Sign-in tab
        with _tab_in:
            with st.form("login_form"):
                st.markdown("#### Welcome back")
                _login_dev = st.text_input("Device ID", placeholder="e.g. fridge-001")
                _login_pwd = st.text_input("Password",  type="password")
                _login_btn = st.form_submit_button("Sign In →", use_container_width=True)
            if _login_btn:
                if not _login_dev or not _login_pwd:
                    st.error("Both Device ID and password are required.")
                else:
                    try:
                        _r = requests.post(
                            f"{BASE_URL}/api/client/login",
                            json={"device_id": _login_dev, "password": _login_pwd},
                            timeout=REQUEST_TIMEOUT_SEC,
                        )
                    except requests.RequestException:
                        st.error("Cannot reach backend — is the ordering service running?")
                        st.stop()
                    if _r.status_code == 200:
                        _d = _r.json()
                        st.session_state.token         = _d.get("access_token")
                        st.session_state.refresh_token = _d.get("refresh_token")
                        st.session_state.device_id     = _login_dev
                        save_auth_cache({"device_id": _login_dev,
                                         "refresh_token": st.session_state.refresh_token})
                        st.rerun()
                    else:
                        st.error("Invalid credentials. Check your Device ID and password.")

        # Register tab
        with _tab_up:
            with st.form("register_form"):
                st.markdown("#### Register a smart device")
                _reg_dev   = st.text_input("Device ID",  placeholder="e.g. fridge-home-01")
                _reg_pwd   = st.text_input("Password",   type="password")
                _reg_email = st.text_input("Email",      placeholder="you@example.com")
                _reg_phone = st.text_input("Phone",      placeholder="+1-555-0100")
                _reg_btn   = st.form_submit_button("Create Account →", use_container_width=True)
            if _reg_btn:
                try:
                    _r = requests.post(
                        f"{BASE_URL}/api/client/register",
                        json={"device_id": _reg_dev, "email": _reg_email,
                              "phone": _reg_phone, "password": _reg_pwd},
                        timeout=REQUEST_TIMEOUT_SEC,
                    )
                except requests.RequestException:
                    st.error("Backend unreachable.")
                    st.stop()
                if _r.status_code == 201:
                    st.success("Account created! Switch to Sign In to log in.")
                else:
                    st.error(f"Registration failed: {_r.text}")

# =============================================================================
#  MAIN APP (authenticated)
# =============================================================================
else:
    maybe_auto_refresh_access_token()

    # ── Top bar ──────────────────────────────────────────────────────────────
    _did = st.session_state.device_id or "—"
    _tb_logo, _tb_dev, _tb_active, _tb_out = st.columns([5, 2, 1.5, 1.5])
    with _tb_logo:
        st.markdown(
            '<p style="margin:0;padding:4px 0 0;font-size:1.12rem;font-weight:700;'
            'color:#A5B4FC;letter-spacing:-0.01em">🛒 AutoGrocery</p>',
            unsafe_allow_html=True,
        )
    with _tb_dev:
        st.markdown(
            f'<div style="display:flex;align-items:center;justify-content:flex-end;height:38px">'
            f'<span style="font-family:monospace;font-size:0.75rem;color:#C7D2FE;font-weight:600;'
            f'background:linear-gradient(135deg,#1E1B4B,#172554);'
            f'padding:0 12px;height:26px;line-height:26px;display:inline-flex;align-items:center;'
            f'border-radius:7px;border:1px solid #3730A3;letter-spacing:0.03em">{_did}</span>'
            f'</div>',
            unsafe_allow_html=True,
        )
    with _tb_active:
        st.markdown(
            f'<div style="display:flex;align-items:center;justify-content:center;height:38px">'
            f'<span style="background:linear-gradient(135deg,#064E3B,#065F46);'
            f'color:#6EE7B7;font-size:0.65rem;font-weight:800;'
            f'padding:0 10px;height:26px;line-height:26px;display:inline-flex;align-items:center;'
            f'border-radius:20px;letter-spacing:0.1em;text-transform:uppercase;'
            f'border:1px solid #059669;box-shadow:0 0 8px rgba(16,185,129,0.25)">● ACTIVE</span>'
            f'</div>',
            unsafe_allow_html=True,
        )
    with _tb_out:
        st.markdown('<div class="ag-tb-cell">', unsafe_allow_html=True)
        if st.button("→ SIGN OUT", key="signout_btn"):
            st.session_state.clear()
            clear_auth_cache()
            st.rerun()
        st.markdown('</div>', unsafe_allow_html=True)
    st.markdown('<hr style="margin:0.4rem 0 0.6rem 0">', unsafe_allow_html=True)

    _page = "New Order"

    # ─────────────────────────────────────────────────────────────────────────
    #  PAGE: NEW ORDER
    # ─────────────────────────────────────────────────────────────────────────
    if _page == "New Order":
        st.markdown(
            '<h2 style="margin:0.6rem 0 0.2rem 0;font-size:1.6rem;font-weight:700;color:#A5B4FC">'
            'New Grocery Order</h2>'
            '<p style="margin:0 0 0.7rem 0;font-size:0.82rem;color:#6B7280">'
            'Build your cart, scan warehouse stock availability, '
            'then dispatch the robot fleet to retrieve your items.</p>'
            '<hr style="border-color:#1F2937;margin:0 0 0.8rem 0">',
            unsafe_allow_html=True,
        )

        # Cart on the left, order status on the right
        _col_cart, _col_status = st.columns([3, 1], gap="medium")

        with _col_cart:
            st.markdown(
                '<p style="margin:0 0 0.5rem 0;font-size:0.85rem;font-weight:700;'
                'color:#C7D2FE;text-transform:uppercase;letter-spacing:0.06em">Cart</p>',
                unsafe_allow_html=True,
            )
            for _i, _item in enumerate(st.session_state.cart_items):
                _c1, _c2, _c3 = st.columns([4, 1, 1])
                _new_sku = _c1.text_input(
                    "SKU", value=_item["sku"], key=f"sku_{_i}",
                    placeholder="e.g. APPLE-101",
                    label_visibility="collapsed",
                )
                _new_qty = _c2.number_input(
                    "Qty", min_value=1, value=_item["qty"], key=f"qty_{_i}",
                    label_visibility="collapsed",
                )
                _rm = _c3.button(
                    "✕", key=f"rm_{_i}",
                    disabled=(len(st.session_state.cart_items) == 1),
                )
                if _rm:
                    st.session_state.cart_items.pop(_i)
                    if st.session_state.order_id:
                        st.session_state.order_id        = None
                        st.session_state.confirmed_items = {}
                        st.session_state.cart_snapshot   = None
                    st.rerun()
                st.session_state.cart_items[_i]["sku"] = _new_sku
                st.session_state.cart_items[_i]["qty"] = _new_qty

            if st.button("＋ Add Item", key="add_row"):
                st.session_state.cart_items.append({"sku": "", "qty": 1})
                if st.session_state.order_id:
                    st.session_state.order_id        = None
                    st.session_state.confirmed_items = {}
                    st.session_state.cart_snapshot   = None
                st.rerun()

        with _col_status:
            st.markdown(
                '<p style="margin:0 0 0.5rem 0;font-size:0.85rem;font-weight:700;'
                'color:#C7D2FE;text-transform:uppercase;letter-spacing:0.06em">Status</p>',
                unsafe_allow_html=True,
            )
            _sanim = st.session_state.get("status_anim")
            if _sanim == "scanning":
                st.markdown(
                    '<div class="ag-status-card">'  
                    '<div class="ag-sonar-wrap">'
                    '<div class="ag-ring"></div><div class="ag-ring"></div><div class="ag-ring"></div>'
                    '</div>'
                    '<div style="color:#818CF8;font-weight:700;font-size:0.88rem">Scanning Warehouse</div>'
                    '<div style="color:#6B7280;font-size:0.75rem;margin-top:0.2rem">Checking live inventory</div>'
                    '<div class="ag-dots" style="color:#6366F1"><span></span><span></span><span></span></div>'
                    '</div>',
                    unsafe_allow_html=True,
                )
            elif _sanim == "cancelling":
                st.markdown(
                    '<div class="ag-status-card">'
                    '<div class="ag-spin-icon">⚙️</div>'
                    '<div style="color:#F87171;font-weight:700;font-size:0.88rem">Cancelling Order</div>'
                    '<div class="ag-dots" style="color:#F87171"><span></span><span></span><span></span></div>'
                    '</div>',
                    unsafe_allow_html=True,
                )
            elif _sanim == "dispatching" or st.session_state.get("is_dispatching"):
                st.markdown(
                    '<div class="ag-status-card">'
                    '<div class="ag-robot-bounce">🤖</div>'
                    '<div style="color:#818CF8;font-weight:700;font-size:0.88rem;margin-top:0.2rem">Robots Deployed</div>'
                    '<div style="color:#6B7280;font-size:0.75rem">En route to your location</div>'
                    '<div class="ag-dots" style="color:#6366F1"><span></span><span></span><span></span></div>'
                    '</div>',
                    unsafe_allow_html=True,
                )
            elif _sanim == "pending" or st.session_state.order_id:
                _oid = st.session_state.order_id or ""
                _short = (_oid[:14] + "…") if len(_oid) > 14 else _oid
                _n = len(st.session_state.confirmed_items or {})
                st.markdown(
                    '<div class="ag-status-card">'
                    '<div class="ag-pulse-dot"></div>'
                    '<div style="color:#FACC15;font-weight:700;font-size:0.88rem">Stock Reserved</div>'
                    f'<code style="color:#818CF8;font-size:0.66rem;display:block;margin:0.35rem 0">{_short}</code>'
                    f'<span class="ag-badge ag-badge-yellow">PENDING DISPATCH</span>'
                    f'<div style="color:#6B7280;font-size:0.72rem;margin-top:0.45rem">{_n} SKU(s) reserved</div>'
                    '</div>',
                    unsafe_allow_html=True,
                )
            elif st.session_state.get("last_receipt"):
                _lr   = st.session_state.last_receipt
                _lrid = _lr.get("OrderID", "")
                _lrshort = (_lrid[:14] + "…") if len(_lrid) > 14 else _lrid
                _lrprice = _lr.get("TotalPrice", 0.0)
                st.markdown(
                    '<div class="ag-status-card">'
                    '<span class="ag-done-icon">✅</span>'
                    '<div style="color:#6EE7B7;font-weight:700;font-size:0.85rem">Last Order Complete</div>'
                    f'<div style="color:#10B981;font-size:1.5rem;font-weight:800;margin:0.3rem 0">${_lrprice:.2f}</div>'
                    f'<code style="color:#818CF8;font-size:0.68rem">{_lrshort}</code>'
                    '</div>',
                    unsafe_allow_html=True,
                )
            else:
                st.markdown(
                    '<div class="ag-status-card" style="padding:1.8rem 1rem">'
                    '<div style="font-size:2.2rem">📦</div>'
                    '<div style="color:#6B7280;font-size:0.82rem;margin-top:6px">No active order</div>'
                    '</div>',
                    unsafe_allow_html=True,
                )

        st.markdown("---")

        # ── Warehouse scan results panel (full-width, shown after a successful scan) ──
        if (
            st.session_state.order_id
            and st.session_state.confirmed_items
            and not st.session_state.get("is_dispatching")
            and st.session_state.get("status_anim") != "scanning"
        ):
            _ci       = st.session_state.confirmed_items
            _cart_map = {
                _it["sku"].strip(): int(_it["qty"])
                for _it in st.session_state.cart_items if _it["sku"].strip()
            }
            _scan_oid   = st.session_state.order_id
            _short_scan = (_scan_oid[:28] + "…") if len(_scan_oid) > 28 else _scan_oid
            _rows_html  = ""
            for _sk, _avail in _ci.items():
                _req = _cart_map.get(_sk, "—")
                if isinstance(_req, int) and _avail >= _req:
                    _avail_cell = f'<span style="color:#10B981;font-weight:700">✓ {_avail}</span>'
                elif _avail > 0:
                    _avail_cell = f'<span style="color:#FACC15;font-weight:700">⚠ {_avail}</span>'
                else:
                    _avail_cell = '<span style="color:#F87171;font-weight:700">✗ 0</span>'
                _rows_html += (
                    f'<tr style="border-bottom:1px solid #1F2937">'
                    f'<td style="padding:9px 14px;color:#C7D2FE;font-family:monospace;font-size:0.82rem">'
                    f'{_sk}</td>'
                    f'<td style="padding:9px 14px;color:#9CA3AF;font-size:0.82rem;text-align:center">'
                    f'{_req}</td>'
                    f'<td style="padding:9px 14px;font-size:0.82rem;text-align:center">'
                    f'{_avail_cell}</td>'
                    f'</tr>'
                )
            _th = (
                '<th style="padding:7px 14px;color:#6B7280;font-size:0.68rem;text-transform:uppercase;'
                'letter-spacing:0.09em;font-weight:600;text-align:{align}">{label}</th>'
            )
            _thead = (
                '<tr style="border-bottom:2px solid #374151">' +
                _th.format(align="left",   label="SKU") +
                _th.format(align="center", label="Requested") +
                _th.format(align="center", label="Available in Warehouse") +
                '</tr>'
            )
            st.markdown(
                '<div style="background:#0F172A;border:1px solid #1F2937;border-radius:12px;'
                'padding:1rem 1.2rem;margin-bottom:1.2rem">'
                '<div style="display:flex;align-items:center;justify-content:space-between;'
                'margin-bottom:0.75rem">'
                '<div style="display:flex;align-items:center;gap:10px">'
                '<div class="ag-pulse-dot" style="margin:0"></div>'
                '<span style="color:#FACC15;font-weight:700;font-size:0.85rem;'
                'text-transform:uppercase;letter-spacing:0.07em">Warehouse Scan Results</span>'
                '</div>'
                f'<code style="color:#4B5563;font-size:0.68rem">{_short_scan}</code>'
                '</div>'
                f'<table style="width:100%;border-collapse:collapse">'
                f'<thead>{_thead}</thead>'
                f'<tbody>{_rows_html}</tbody>'
                '</table>'
                '</div>',
                unsafe_allow_html=True,
            )

        # Detect if cart was edited after the last scan
        _current_cart = sorted(
            [(_it["sku"].strip(), int(_it["qty"]))
             for _it in st.session_state.cart_items if _it["sku"].strip()]
        )
        _cart_dirty = (
            bool(st.session_state.order_id)
            and st.session_state.cart_snapshot is not None
            and _current_cart != st.session_state.cart_snapshot
        )

        # Action bar
        _dispatching = st.session_state.get("is_dispatching", False)

        if _dispatching:
            _scan_clicked = _cancel_clicked = _dispatch_clicked = False
        else:
            if _cart_dirty:
                st.warning("⚠️ Cart changed since last scan — re-scan to update the reservation before dispatching.")
            _b1, _b2, _b3 = st.columns(3)
            with _b1:
                _scan_clicked = st.button(
                    "🔍 Scan Stock Availability", key="scan_btn", use_container_width=True
                )
            with _b2:
                _cancel_clicked = st.button(
                    "🗑 Cancel Order", key="cancel_btn",
                    disabled=(not st.session_state.order_id or _cart_dirty),
                    use_container_width=True,
                )
            with _b3:
                _dispatch_clicked = st.button(
                    "🚀 Dispatch Robots", key="dispatch_btn",
                    disabled=(
                        not st.session_state.order_id
                        or not st.session_state.confirmed_items
                        or _cart_dirty
                    ),
                    use_container_width=True,
                )

        # Two-pass animation triggers: first click sets animated state + reruns;
        # second pass (via _trigger_* flag) actually executes the action.
        if _scan_clicked and not st.session_state.get("_trigger_scan"):
            _pre_payload = [
                _it["sku"].strip()
                for _it in st.session_state.cart_items if _it["sku"].strip()
            ]
            if not _pre_payload:
                st.warning("Add at least one SKU before scanning.")
            else:
                st.session_state.status_anim   = "scanning"
                st.session_state._trigger_scan = True
                st.rerun()
        if _cancel_clicked and not st.session_state.get("_trigger_cancel"):
            st.session_state.status_anim     = "cancelling"
            st.session_state._trigger_cancel = True
            st.rerun()
        if _dispatch_clicked and not st.session_state.get("_trigger_dispatch"):
            st.session_state.status_anim       = "dispatching"
            st.session_state._trigger_dispatch = True
            st.session_state.is_dispatching    = True
            st.rerun()

        _do_scan     = bool(st.session_state.get("_trigger_scan"))
        _do_cancel   = bool(st.session_state.get("_trigger_cancel"))
        _do_dispatch = bool(st.session_state.get("_trigger_dispatch"))

        # ─────────────────────────────────────────────────────────────
        #  SCAN handler
        # ─────────────────────────────────────────────────────────────
        if _do_scan:
            st.session_state._trigger_scan = False
            _payload = [
                {"sku": _it["sku"].strip(), "quantity": int(_it["qty"])}
                for _it in st.session_state.cart_items if _it["sku"].strip()
            ]
            if not _payload:
                st.warning("Add at least one SKU before scanning.")
                st.stop()

            if st.session_state.order_id:
                try:
                    auth_request("POST", "/api/client/order/cancel",
                                 {"order_id": st.session_state.order_id})
                except Exception:
                    pass
                st.session_state.order_id        = None
                st.session_state.confirmed_items = {}
                st.session_state.cart_snapshot   = None

            with st.spinner("Scanning warehouse inventory…"):
                _res = auth_request("POST", "/api/client/order/preview",
                                    {"items": _payload})

            if _res is None:
                st.error("Backend unreachable — is the ordering service running?")
                st.stop()

            if _res.status_code == 201:
                _data = _res.json()
                st.session_state.order_id        = _data.get("order_id")
                st.session_state.confirmed_items = _data.get("items") or {}
                st.session_state.cart_snapshot   = sorted(
                    [(_it["sku"].strip(), int(_it["qty"]))
                     for _it in st.session_state.cart_items if _it["sku"].strip()]
                )
                if st.session_state.confirmed_items:
                    st.session_state.status_anim = "pending"
                    st.rerun()
                else:
                    st.session_state.status_anim = None
                    st.error("None of the requested items are currently in stock.")
                    st.info("Use the **Truck UI** (port 8501) to restock, then retry.")

            elif _res.status_code == 409:
                _body = _res.text
                if "pending order:" in _body.lower():
                    _m = re.search(
                        r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}',
                        _body, re.IGNORECASE,
                    )
                    if _m:
                        _oid = _m.group(0)
                        _ostatus = None
                        try:
                            _sr = auth_request("GET", "/api/client/orders/last")
                            if _sr and _sr.status_code == 200:
                                _ostatus = _sr.json().get("data", {}).get("Status")
                        except Exception:
                            pass

                        if _ostatus == "PROCESSING":
                            st.session_state.order_id        = _oid
                            st.session_state.confirmed_items = {}
                            st.session_state.cart_snapshot   = None
                            st.session_state.status_anim     = "dispatching"
                            st.session_state.is_dispatching  = True
                            st.info("A previous order is still being processed. Resuming watch…")
                            with st.status("Monitoring in-flight order…", expanded=True) as _s:
                                while True:
                                    _poll = auth_request("GET", "/api/client/orders/last")
                                    if _poll is None:
                                        _s.update(
                                            label="Connection lost — watchdog will auto-resolve.",
                                            state="error",
                                        )
                                        break
                                    _od  = _poll.json().get("data", {})
                                    _os  = _od.get("Status")
                                    _s.write(f"Status: **`{_os}`**")
                                    if _os == "COMPLETED":
                                        _fp = _od.get("TotalPrice", 0.0)
                                        _s.update(
                                            label=f"Order complete — ${_fp:.2f} charged",
                                            state="complete",
                                        )
                                        _show_receipt(_od)
                                        st.session_state.last_receipt     = _od
                                        st.session_state.order_id        = None
                                        st.session_state.confirmed_items = {}
                                        st.session_state.cart_snapshot   = None
                                        if "order_history_cache" in st.session_state:
                                            del st.session_state["order_history_cache"]
                                        st.rerun()
                                        break
                                    elif "FAILED" in str(_os):
                                        _s.update(label="Order failed.", state="error")
                                        st.session_state.order_id        = None
                                        st.session_state.confirmed_items = {}
                                        st.session_state.cart_snapshot   = None
                                        break
                                    time.sleep(2)
                        elif _ostatus in ("COMPLETED", "FAILED", "CANCELLING", "CANCELLED"):
                            st.session_state.order_id    = None
                            st.session_state.status_anim = None
                            st.info(f"Previous order already **{_ostatus}**. You may start a new scan.")
                        else:
                            st.session_state.order_id        = _oid
                            st.session_state.confirmed_items = {}
                            st.session_state.cart_snapshot   = None
                            st.session_state.status_anim     = "pending"
                            st.warning("You have an unfinished pending order — cancel or dispatch it.")
                            st.rerun()
                    else:
                        st.session_state.status_anim = None
                        st.error(f"Scan rejected: {_body}")
                else:
                    st.session_state.status_anim = None
                    _t = _body.lower()
                    if "insufficient" in _t or "stock" in _t:
                        st.error("Insufficient warehouse stock for one or more items.")
                        st.info("Restock via the Truck UI, then retry.")
                    else:
                        st.error(f"Scan failed ({_res.status_code}): {_body}")
            else:
                st.session_state.status_anim = None
                st.error(f"Scan failed ({_res.status_code}): {_res.text}")
        # ─────────────────────────────────────────────────────────────
        if _do_cancel and st.session_state.order_id:
            st.session_state._trigger_cancel = False
            with st.spinner("Cancelling order and releasing reservation…"):
                _res = auth_request("POST", "/api/client/order/cancel",
                                    {"order_id": st.session_state.order_id})
            if _res is None:
                st.error("Backend unreachable.")
            elif _res.status_code == 200:
                st.session_state.status_anim     = None
                st.session_state.order_id        = None
                st.session_state.confirmed_items = {}
                st.session_state.cart_snapshot   = None
                st.rerun()
            else:
                st.session_state.status_anim = "pending"
                st.error(f"Cancel failed ({_res.status_code}): {_res.text}")

        # ─────────────────────────────────────────────────────────────
        #  DISPATCH handler
        # ─────────────────────────────────────────────────────────────
        if _do_dispatch and st.session_state.order_id:
            st.session_state._trigger_dispatch = False
            _current_cart = sorted(
                [(_it["sku"].strip(), int(_it["qty"]))
                 for _it in st.session_state.cart_items if _it["sku"].strip()]
            )
            if st.session_state.cart_snapshot != _current_cart:
                st.session_state.status_anim    = "pending"
                st.session_state.is_dispatching = False
                st.error(
                    "Cart was modified after the stock scan — re-scan before dispatching."
                )
                st.stop()

            def _clear_order_state():
                st.session_state.order_id          = None
                st.session_state.confirmed_items   = {}
                st.session_state.cart_snapshot     = None
                st.session_state.is_dispatching    = False
                st.session_state.status_anim       = None
                st.session_state._trigger_dispatch = False

            with st.spinner("Sending dispatch command to warehouse…"):
                _res = auth_request("POST", "/api/client/order/confirm", {
                    "order_id": st.session_state.order_id,
                    "items":    st.session_state.confirmed_items,
                })

            if _res is None:
                st.error("Backend unreachable during dispatch.")
                _clear_order_state()
                st.rerun()
            elif _res.status_code == 200:
                with st.status(
                    "Robot fleet deployed — monitoring warehouse telemetry…",
                    expanded=True,
                ) as _s:
                    while True:
                        _poll = auth_request("GET", "/api/client/orders/last")
                        if _poll is None:
                            _s.update(
                                label="Connection lost — watchdog will auto-resolve the order.",
                                state="error",
                            )
                            time.sleep(2)
                            _clear_order_state()
                            st.rerun()
                            break
                        _od = _poll.json().get("data", {})
                        _os = _od.get("Status")
                        _s.write(f"Warehouse status: **`{_os}`**")
                        if _os == "COMPLETED":
                            _fp = _od.get("TotalPrice", 0.0)
                            _s.update(
                                label=f"Delivery complete — ${_fp:.2f} charged",
                                state="complete",
                            )
                            st.balloons()
                            _show_receipt(_od)
                            st.session_state.last_receipt = _od
                            if "order_history_cache" in st.session_state:
                                del st.session_state["order_history_cache"]
                            _clear_order_state()
                            st.rerun()
                            break
                        elif "FAILED" in str(_os):
                            _s.update(label="Order failed — robots reported an issue.",
                                      state="error")
                            time.sleep(3)
                            _clear_order_state()
                            st.rerun()
                            break
                        time.sleep(2)
            else:
                st.error(f"Dispatch rejected ({_res.status_code}): {_res.text}")
                _clear_order_state()

    # ─────────────────────────────────────────────────────────────────────────
    #  PERSISTENT RECEIPT
    # ─────────────────────────────────────────────────────────────────────────
    if st.session_state.get("last_receipt"):
        st.markdown("---")
        _show_receipt(st.session_state.last_receipt)
        if st.button("✕ Clear Receipt", key="clear_receipt_btn"):
            st.session_state.last_receipt = None
            st.rerun()

    # ─────────────────────────────────────────────────────────────────────────
    #  ORDER HISTORY (inline, auto-loads)
    # ─────────────────────────────────────────────────────────────────────────
    st.markdown("---")
    _oh_l, _oh_r = st.columns([8, 1])
    with _oh_l:
        st.markdown("## Order History")
    with _oh_r:
        if st.button("↻", key="refresh_hist_btn", help="Refresh order history"):
            if "order_history_cache" in st.session_state:
                del st.session_state["order_history_cache"]
            st.rerun()
    st.caption("All orders placed from this device — newest first.")

    if "order_history_cache" not in st.session_state:
        _hres = auth_request("GET", "/api/client/orders")
        if _hres and _hres.status_code == 200:
            st.session_state.order_history_cache = _hres.json().get("data", [])
        else:
            st.session_state.order_history_cache = []

    _hist_rows = st.session_state.get("order_history_cache", [])
    if _hist_rows:
        import pandas as pd
        _hdf = pd.DataFrame(_hist_rows)
        st.dataframe(_hdf, use_container_width=True, hide_index=True)
        st.caption(f"{len(_hist_rows)} order(s) on record for this device.")
    else:
        st.info("No orders yet — place your first order above.")
