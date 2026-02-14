#include <iostream>
#include <string>
#include <vector>
#include <map>
#include <thread>
#include <chrono>
#include <cstdlib>
#include <fstream>
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

class RobotWorker {
public:
    /**
     * @brief Constructs a robot worker bound to a single aisle.
     */
    RobotWorker(std::shared_ptr<Channel> channel, std::string aisle, std::string zmqSubAddr)
        : stub_(InventoryService::NewStub(channel)), aisle_type_(aisle), zmq_sub_addr_(zmqSubAddr) {}

    /**
     * @brief Starts the receive-filter-work-report processing loop.
     */
    void Run() {
        zmq::context_t context(1);
        zmq::socket_t subscriber(context, zmq::socket_type::sub);
        subscriber.connect(zmq_sub_addr_);
        subscriber.set(zmq::sockopt::subscribe, ""); 
        std::cout << "[robot] connected SUB socket to " << zmq_sub_addr_ << std::endl;

        // Updated log message to only use aisle
        std::cout << "Robot started for aisle: " << aisle_type_ << std::endl;

        while (true) {
            zmq::message_t msg;
            auto res = subscriber.recv(msg, zmq::recv_flags::none);
            if (!res) {
                std::cout << "[robot] recv returned no message for aisle=" << aisle_type_ << std::endl;
                continue;
            }
            std::cout << "[robot] raw broadcast received bytes=" << msg.size() << " aisle=" << aisle_type_ << std::endl;
            
            auto broadcast = RobotMessages::GetOrderBroadcast(msg.data());

            std::string order_type = broadcast->order_type()->str();
            std::string order_id = broadcast->order_id()->str();
            std::cout << "Received " << order_type << " Job: " << order_id << std::endl;
            
            bool found_work = false;
            std::map<std::string, int32_t> processed_items;

            std::cout << "[robot] filtering items for aisle=" << aisle_type_ << std::endl;

            for (auto item : *broadcast->items()) {
                std::cout << "[robot] candidate item sku=" << item->sku()->str() << " aisle=" << item->aisle()->str() << " qty=" << item->quantity() << std::endl;
                if (item->aisle()->str() == aisle_type_) {
                    found_work = true;
                    std::cout << "Picking " << item->quantity() << "x " << item->sku()->str() << std::endl;
                    processed_items[item->sku()->str()] = item->quantity();
                    std::cout << "[robot] start work sleep sku=" << item->sku()->str() << " order=" << order_id << std::endl;
                    
                    std::this_thread::sleep_for(std::chrono::milliseconds(5000));
                    std::cout << "[robot] finished work sku=" << item->sku()->str() << " order=" << order_id << std::endl;
                }
            }

            if (!found_work) {
                std::cout << "[robot] no matching aisle work for order=" << order_id << " aisle=" << aisle_type_ << std::endl;
            }

            ReportToInventory(order_id, order_type, found_work, processed_items);
        }
    }

private:
    /**
     * @brief Reports per-order processing status back to inventory via gRPC.
     */
    void ReportToInventory(const std::string& order_id, const std::string& order_type, bool worked, const std::map<std::string, int32_t>& items) {
        ReportJobStatusRequest request;
        ReportJobStatusResponse response;
        ClientContext context;

        request.set_order_id(order_id);
        request.set_order_type(order_type);
        request.set_status(worked ? "SUCCESS" : "NO_OP");
        std::cout << "[robot] reporting status order=" << order_id << " type=" << order_type << " status=" << request.status() << std::endl;
        
        auto* req_items = request.mutable_processed_items(); 
        for (auto const& [sku, qty] : items) {
            (*req_items)[sku] = qty;
            std::cout << "[robot] report item sku=" << sku << " qty=" << qty << std::endl;
        }

        Status status = stub_->ReportJobStatus(&context, request, &response);

        if (status.ok()) {
            std::cout << "Reported status for Order: " << order_id << " (" << request.status() << ")" << std::endl;
            std::cout << "[robot] grpc report success order=" << order_id << std::endl;
        } else {
            std::cerr << "gRPC failed: " << status.error_message() << std::endl;
            std::cerr << "[robot] grpc report failed order=" << order_id << " code=" << status.error_code() << " message=" << status.error_message() << std::endl;
        }
    }

    std::unique_ptr<InventoryService::Stub> stub_;
    std::string aisle_type_;
    std::string zmq_sub_addr_;
};

int main(int argc, char** argv) {
    /**
     * @brief Starts a robot process for the provided aisle argument.
     */
    if (argc < 2) {
        std::cerr << "Usage: robot_exe <aisle_name>" << std::endl;
        return 1;
    }
    
    std::string aisle = argv[1];
    LoadDotEnv("../.env");
    LoadDotEnv("robots/.env");

    const std::string inventoryGrpcAddr = GetEnv("INVENTORY_GRPC_ADDR", "localhost:50051");
    const std::string robotZmqSubAddr = GetEnv("ROBOT_ZMQ_SUB_ADDR", "tcp://localhost:5556");
    
    RobotWorker robot(grpc::CreateChannel(inventoryGrpcAddr, grpc::InsecureChannelCredentials()), aisle, robotZmqSubAddr);
    robot.Run();

    return 0;
}