#include <zmq.hpp>
#include <fstream>
#include "messages_generated.h"

int main() {
    zmq::context_t ctx(1);
    zmq::socket_t sub(ctx, ZMQ_SUB);
    sub.connect("tcp://localhost:5556");
    sub.set(zmq::sockopt::subscribe, "");

    std::ofstream csv("metrics.csv", std::ios::app);

    while (true) {
        zmq::message_t msg;
        sub.recv(msg, zmq::recv_flags::none);
        auto data = auto_grocery::fb::GetTaskMessage(msg.data());

        if (data->type() == auto_grocery::fb::MessageType_ANALYTICS_DATA) {
            // Log for Milestone 3 Graphs
            csv << data->order_id()->str() << "," << data->latency_ms() << std::endl;
        }
    }
}