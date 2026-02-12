package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"auto_grocery/inventory/internal/handler"
	"auto_grocery/inventory/internal/mq"
	"auto_grocery/inventory/internal/store"
	pb "auto_grocery/inventory/proto"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 1. Load configuration
	if err := godotenv.Load("inventory/.env"); err != nil {
		log.Println("Note: No inventory/.env file found, using system environment variables")
	}

	// 2. Database Connection (Postgres)
	dbURL := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("❌ Failed to open database connection: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Failed to connect to Postgres: %v", err)
	}
	defer db.Close()
	stockStore := store.NewStore(db)
	fmt.Println("✅ Connected to Inventory Database (Postgres)")

	// 3. Redis Connection (Memory Store)
	redisAddr := os.Getenv("REDIS_ADDR")
	redisPw := os.Getenv("REDIS_PW")
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))

	memoryStore := store.NewMemoryStore(redisAddr, redisPw, redisDB)
	fmt.Println("✅ Connected to Redis Memory Store")

	// 4. ZeroMQ Publisher Setup
	// We'll use port 5556 for the robot broadcast
	publisher, err := mq.NewPublisher("5556")
	if err != nil {
		log.Fatalf("❌ Failed to start ZMQ Publisher: %v", err)
	}
	defer publisher.Close()
	fmt.Println("✅ ZMQ Publisher active on port 5556")

	// 5. Pricing Service Connection (gRPC Client)
	pricingAddr := "localhost:50052" // Adjust based on your pricing service port
	conn, err := grpc.Dial(pricingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("❌ Could not connect to Pricing Service: %v", err)
	}
	defer conn.Close()
	pricingClient := pb.NewPricingServiceClient(conn)
	fmt.Println("✅ Connected to Pricing Service")

	// 6. Initialize gRPC Server and Handler
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Failed to listen on port 50051: %v", err)
	}

	inventoryHandler := handler.NewInventoryHandler(
		stockStore,
		memoryStore,
		publisher,
		pricingClient,
	)

	grpcServer := grpc.NewServer()
	pb.RegisterInventoryServiceServer(grpcServer, inventoryHandler)

	// 7. Start Server
	fmt.Println("🚀 Inventory Service running on gRPC port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ gRPC Server crashed: %v", err)
	}
}