#include <zmq.hpp>
#include <thread>
#include <chrono>
#include "messages_generated.h"
#include "inventory.grpc.pb.h"

void run_robot(std::string assigned_aisle, std::shared_ptr<grpc::ChannelInterface> channel) {
    zmq::context_t ctx(1);
    zmq::socket_t sub(ctx, ZMQ_SUB);
    sub.connect("tcp://localhost:5555");
    sub.set(zmq::sockopt::subscribe, "");

    auto stub = inventory::InventoryService::NewStub(channel);

    while (true) {
        zmq::message_t msg;
        sub.recv(msg, zmq::recv_flags::none);
        auto task = auto_grocery::fb::GetTaskMessage(msg.data());

        bool work_done = false;
        for (auto item : *task->items()) {
            if (item->aisle()->str() == assigned_aisle) {
                // Requirement: Simulate work with sleep
                std::this_thread::sleep_for(std::chrono::milliseconds(500));
                work_done = true;
            }
        }

        // gRPC Reporting
        grpc::ClientContext context;
        inventory::ReportJobStatusRequest report;
        report.set_order_id(task->order_id()->str());
        report.set_status(work_done ? "SUCCESS" : "NO_OP");
        inventory::ReportJobStatusResponse reply;
        stub->ReportJobStatus(&context, report, &reply);
    }
}