#include <iostream>
#include <string>
#include <vector>
#include <thread>
#include <chrono>
#include <zmq.hpp>

// Generated Flatbuffers headers
#include "order_generated.h"

// Generated gRPC headers
#include <grpcpp/grpcpp.h>
#include "inventory.grpc.pb.h"

using grpc::Channel;
using grpc::ClientContext;
using grpc::Status;
using inventory::InventoryService;
using inventory::ReportJobStatusRequest;
using inventory::ReportJobStatusResponse;

class RobotWorker {
public:
    RobotWorker(std::shared_ptr<Channel> channel, std::string aisle, std::string id)
        : stub_(InventoryService::NewStub(channel)), aisle_type_(aisle), robot_id_(id) {}

    void Run() {
        // 1. Setup ZeroMQ Subscriber
        zmq::context_t context(1);
        zmq::socket_t subscriber(context, zmq::socket_type::sub);
        subscriber.connect("tcp://localhost:5556"); // Inventory PUB address
        subscriber.set(zmq::sockopt::subscribe, ""); 

        std::cout << "Robot [" << robot_id_ << "] started for aisle: " << aisle_type_ << std::endl;

        while (true) {
            zmq::message_t msg;
            auto res = subscriber.recv(msg, zmq::recv_flags::none);
            
            // 2. Parse Flatbuffer message
            auto broadcast = RobotMessages::GetOrderBroadcast(msg.data());
            std::string order_id = broadcast->order_id()->str();
            
            bool found_work = false;
            std::map<std::string, int32_t> processed_items;

            for (auto item : *broadcast->items()) {
                // Check if this item belongs to this robot's aisle
                if (item->aisle()->str() == aisle_type_) {
                    found_work = true;
                    std::cout << "Picking " << item->quantity() << "x " << item->sku()->str() << std::endl;
                    processed_items[item->sku()->str()] = item->quantity();
                    
                    // Simulate work
                    std::this_thread::sleep_for(std::chrono::milliseconds(5000));
                }
            }

            // 3. Report back to Inventory via gRPC
            ReportToInventory(order_id, found_work, processed_items);
        }
    }

private:
    void ReportToInventory(const std::string& order_id, bool worked, const std::map<std::string, int32_t>& items) {
        ReportJobStatusRequest request;
        ReportJobStatusResponse response;
        ClientContext context;

        request.set_order_id(order_id);
        request.set_robot_id(robot_id_);
        request.set_status(worked ? "SUCCESS" : "NO_OP");
        
        auto* req_items = request.mutable_items();
        for (auto const& [sku, qty] : items) {
            (*req_items)[sku] = qty;
        }

        Status status = stub_->ReportJobStatus(&context, request, &response);

        if (status.ok()) {
            std::cout << "Reported status for Order: " << order_id << " (" << request.status() << ")" << std::endl;
        } else {
            std::cerr << "gRPC failed: " << status.error_message() << std::endl;
        }
    }

    std::unique_ptr<InventoryService::Stub> stub_;
    std::string aisle_type_;
    std::string robot_id_;
};

int main(int argc, char** argv) {
    if (argc < 3) {
        std::cerr << "Usage: robot_exe <aisle_name> <robot_id>" << std::endl;
        return 1;
    }
    
    std::string aisle = argv[1];
    std::string id = argv[2];
    
    // Create channel to Inventory gRPC server
    RobotWorker robot(grpc::CreateChannel("localhost:50051", grpc::InsecureChannelCredentials()), aisle, id);
    robot.Run();

    return 0;
}