#include <iostream>
#include <fstream>
#include <zmq.hpp>
#include "analytics_generated.h" // Generated from your .fbs

using namespace AnalyticsMessages;

int main() {
    zmq::context_t context(1);
    zmq::socket_t subscriber(context, zmq::socket_type::sub);
    subscriber.connect("tcp://127.0.0.1:5557");
    subscriber.set(zmq::sockopt::subscribe, "");

    // Open CSV file for writing
    std::ofstream datafile("latency_data.csv", std::ios::app);
    
    // Write CSV Header if file is empty
    if (datafile.tellp() == 0) {
        datafile << "order_id,status,duration_seconds,timestamp\n";
    }

    std::cout << "Analytics logging started. Saving to latency_data.csv..." << std::endl;

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