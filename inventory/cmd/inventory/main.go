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
	
	if err := godotenv.Load("inventory/.env"); err != nil {
		log.Println("Note: No inventory/.env file found, using system environment variables")
	}

	
	dbURL := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer db.Close()
	stockStore := store.NewStore(db)
	fmt.Println("Connected to Inventory Database (Postgres)")

	
	redisAddr := os.Getenv("REDIS_ADDR")
	redisPw := os.Getenv("REDIS_PW")
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))

	memoryStore := store.NewMemoryStore(redisAddr, redisPw, redisDB)
	fmt.Println("Connected to Redis Memory Store")

	
	publisher, err := mq.NewPublisher("5556")
	if err != nil {
		log.Fatalf("Failed to start ZMQ Publisher: %v", err)
	}
	defer publisher.Close()
	fmt.Println("ZMQ Publisher active on port 5556")

	
	pricingAddr := "localhost:50052"
	conn, err := grpc.Dial(pricingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Could not connect to Pricing Service: %v", err)
	}
	defer conn.Close()
	pricingClient := pb.NewPricingServiceClient(conn)
	fmt.Println("Connected to Pricing Service")

	
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen on port 50051: %v", err)
	}

	inventoryHandler := handler.NewInventoryHandler(
		stockStore,
		memoryStore,
		publisher,
		pricingClient,
	)

	grpcServer := grpc.NewServer()
	pb.RegisterInventoryServiceServer(grpcServer, inventoryHandler)

	
	fmt.Println("Inventory Service running on gRPC port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC Server crashed: %v", err)
	}
}