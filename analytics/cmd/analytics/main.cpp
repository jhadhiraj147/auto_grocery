#include <iostream>
#include <fstream>
#include <cstdlib>
#include <string>
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
 * @brief Subscribes to analytics metrics and appends latency records to CSV.
 */
int main() {
    LoadDotEnv("../.env");
    LoadDotEnv("analytics/.env");

    const std::string analyticsSubAddr = GetEnv("ANALYTICS_ZMQ_SUB_ADDR", "tcp://127.0.0.1:5557");
    const std::string analyticsCsvPath = GetEnv("ANALYTICS_OUTPUT_CSV", "latency_data.csv");

    zmq::context_t context(1);
    zmq::socket_t subscriber(context, zmq::socket_type::sub);
    subscriber.connect(analyticsSubAddr);
    subscriber.set(zmq::sockopt::subscribe, "");

    // Open CSV file for writing
    std::ofstream datafile(analyticsCsvPath, std::ios::app);
    
    // Write CSV Header if file is empty
    if (datafile.tellp() == 0) {
        datafile << "order_id,status,duration_seconds,timestamp\n";
    }

    std::cout << "Analytics logging started. Subscribed to " << analyticsSubAddr << " and saving to " << analyticsCsvPath << "..." << std::endl;

    while (true) {
        zmq::message_t msg;
        auto res = subscriber.recv(msg, zmq::recv_flags::none);

        // Parse Flatbuffer
        auto metric = GetOrderMetric(msg.data());

        // Store in CSV [cite: 145]
        datafile << metric->order_id()->str() << ","
                 << metric->status()->str() << ","
                 << metric->duration_seconds() << ","
                 << metric->timestamp() << std::endl;

        std::cout << "Logged Order: " << metric->order_id()->str() 
                  << " | Latency: " << metric->duration_seconds() << "s" << std::endl;
    }
    
    datafile.close();
    return 0;
}