import pandas as pd
import matplotlib.pyplot as plt

# 1. Load the data
df = pd.read_csv('latency_data1.csv')

# 2. Pre-process: Convert Unix timestamp to readable DateTime
df['timestamp'] = pd.to_datetime(df['timestamp'], unit='s')
df = df.sort_values('timestamp')

# --- Plot 1: Latency Trend over Time ---
plt.figure(figsize=(10, 5))
plt.plot(df['timestamp'], df['duration_seconds'], marker='o', linestyle='-', color='#2ca02c')
plt.title('Robot Processing Latency Trend')
plt.xlabel('Time of Completion')
plt.ylabel('Duration (seconds)')
plt.grid(True, alpha=0.3)
plt.xticks(rotation=45)
plt.tight_layout()
plt.savefig('latency_trend.png')

# --- Plot 2: Latency per Order (Bar Chart) ---
plt.figure(figsize=(10, 5))
# Use last 8 characters of Order ID for readability
df['label'] = df['order_id'].str[-8:]
# Sorting by duration as per common visualization standards
df_sorted = df.sort_values('duration_seconds', ascending=True)

plt.bar(df_sorted['label'], df_sorted['duration_seconds'], color='skyblue')
plt.title('Latency Comparison by Order ID')
plt.xlabel('Order ID (Last 8 chars)')
plt.ylabel('Duration (seconds)')
plt.xticks(rotation=45)
plt.tight_layout()
plt.savefig('latency_bars.png')

print("Graphs saved as latency_trend.png and latency_bars.png")