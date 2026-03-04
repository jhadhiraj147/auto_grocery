#include <iostream>
#include <fstream>
#include <cstdlib>
#include <string>
#include <map>
#include <numeric>
#include <vector>
#include <zmq.hpp>
#include "analytics_generated.h" // Generated from your .fbs

using namespace AnalyticsMessages;

std::string GetEnv(const char* key, const std::string& fallback) {
    const char* value = std::getenv(key);
    return (value && *value) ? std::string(value) : fallback;
}

void LoadDotEnv(const std::string& path) {
    std::ifstream file(path);
    if (!file.is_open()) return;

    std::string line;
    while (std::getline(file, line)) {
        if (line.empty() || line[0] == '#') continue;
        auto pos = line.find('=');
        if (pos == std::string::npos) continue;

        std::string key = line.substr(0, pos);
        std::string value = line.substr(pos + 1);
        if (!key.empty()) {
            setenv(key.c_str(), value.c_str(), 0);
        }
    }
}

/**
 * @brief Subscribes to analytics metrics, appends latency records to CSV,
 *        and maintains running counters for total / per-status request counts.
 */
int main() {
    LoadDotEnv("../.env");
    LoadDotEnv("analytics/.env");

    const std::string analyticsSubAddr  = GetEnv("ANALYTICS_ZMQ_SUB_ADDR",   "tcp://127.0.0.1:5557");
    const std::string analyticsCsvPath  = GetEnv("ANALYTICS_OUTPUT_CSV",     "latency_data.csv");
    const std::string analyticsSummPath = GetEnv("ANALYTICS_SUMMARY_FILE",   "summary.txt");

    zmq::context_t context(1);
    zmq::socket_t subscriber(context, zmq::socket_type::sub);
    subscriber.connect(analyticsSubAddr);
    subscriber.set(zmq::sockopt::subscribe, "");

    // Open CSV file for appending
    std::ofstream datafile(analyticsCsvPath, std::ios::app);

    // Write CSV Header if file is empty
    if (datafile.tellp() == 0) {
        datafile << "order_id,status,duration_seconds,timestamp\n";
    }

    // ---- Running counters ----
    uint64_t total_requests = 0;
    std::map<std::string, uint64_t> status_counts;  // e.g. COMPLETED->N, FAILED->N
    std::vector<double> latencies;

    std::cout << "Analytics logging started. Subscribed to " << analyticsSubAddr
              << " | CSV: " << analyticsCsvPath
              << " | Summary: " << analyticsSummPath << std::endl;

    while (true) {
        zmq::message_t msg;
        auto res = subscriber.recv(msg, zmq::recv_flags::none);
        if (!res) continue;

        // Parse Flatbuffer
        auto metric = GetOrderMetric(msg.data());
        if (!metric || !metric->order_id() || !metric->status()) {
            std::cerr << "[analytics] WARN received malformed flatbuffer, skipping" << std::endl;
            continue;
        }

        std::string order_id  = metric->order_id()->str();
        std::string status    = metric->status()->str();
        double      duration  = metric->duration_seconds();
        int64_t     timestamp = metric->timestamp();

        // Append to CSV
        datafile << order_id << ","
                 << status   << ","
                 << duration << ","
                 << timestamp << std::endl;
        datafile.flush();

        // Update counters
        ++total_requests;
        ++status_counts[status];
        latencies.push_back(duration);

        double avg_latency = std::accumulate(latencies.begin(), latencies.end(), 0.0) / latencies.size();

        // Print live summary to stdout
        std::cout << "[analytics] order=" << order_id
                  << " status=" << status
                  << " latency=" << duration << "s"
                  << " | TOTAL=" << total_requests;
        for (auto const& [s, c] : status_counts) {
            std::cout << " " << s << "=" << c;
        }
        std::cout << " avg_latency=" << avg_latency << "s" << std::endl;

        // Rewrite summary file with current snapshot
        std::ofstream summary(analyticsSummPath, std::ios::trunc);
        if (summary.is_open()) {
            summary << "=== Analytics Summary ===\n";
            summary << "total_requests: " << total_requests << "\n";
            for (auto const& [s, c] : status_counts) {
                summary << s << ": " << c << "\n";
            }
            summary << "avg_latency_seconds: " << avg_latency << "\n";
        }
    }

    datafile.close();
    return 0;
}