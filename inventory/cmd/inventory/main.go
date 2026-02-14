package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"

	"auto_grocery/inventory/internal/handler"
	"auto_grocery/inventory/internal/mq"
	"auto_grocery/inventory/internal/store"
	pb "auto_grocery/inventory/proto"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

// main wires inventory dependencies and starts the gRPC server.
func main() {
	_ = godotenv.Load("inventory/.env")

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	stockStore := store.NewStore(db)
	// Dual-DB: Database 0 for Clients, Database 1 for Restocks
	redisAddr := getenv("REDIS_ADDR", "localhost:6379")
	redisPassword := os.Getenv("REDIS_PW")
	memoryStore := store.NewMemoryStore(redisAddr, redisPassword)
	if err := memoryStore.Ping(context.Background()); err != nil {
		log.Fatalf("failed to connect/authenticate to Redis at %s: %v", redisAddr, err)
	}

	robotZMQBindAddr := getenv("ROBOT_ZMQ_BIND_ADDR", "tcp://*:5556")
	publisher, err := mq.NewPublisher(robotZMQBindAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer publisher.Close()

	pricingGRPCAddr := getenv("PRICING_GRPC_ADDR", "localhost:50052")
	pricingConn, err := grpc.NewClient(pricingGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer pricingConn.Close()
	pricingClient := pb.NewPricingServiceClient(pricingConn)

	orderWebhookURL := getenv("ORDERING_ORDER_WEBHOOK_URL", "http://localhost:5050/internal/webhook/update-order")
	restockWebhookURL := getenv("ORDERING_RESTOCK_WEBHOOK_URL", "http://localhost:5050/internal/webhook/update-restock")

	inventoryHandler := handler.NewInventoryHandler(stockStore, memoryStore, publisher, pricingClient, orderWebhookURL, restockWebhookURL)

	inventoryGRPCAddr := getenv("INVENTORY_GRPC_ADDR", ":50051")
	lis, err := net.Listen("tcp", inventoryGRPCAddr)
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterInventoryServiceServer(grpcServer, inventoryHandler)

	log.Printf("[inventory] INFO service listening on %s (redis=%s pricing_grpc=%s robot_bind=%s)", inventoryGRPCAddr, redisAddr, pricingGRPCAddr, robotZMQBindAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
