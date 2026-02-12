package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	"auto_grocery/inventory/internal/handler"
	"auto_grocery/inventory/internal/store"

	pb "auto_grocery/inventory/proto"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	err := godotenv.Load("inventory/.env")
	if err != nil {
		log.Println("Note: inventory/.env file not found, using system vars")
	}

	dbConnString := os.Getenv("DATABASE_URL")
	if dbConnString == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dbConnString)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping Inventory DB: %v", err)
	}
	fmt.Println("Connected to Inventory Database")

	inventoryStore := store.NewStore(db)

	pricingAddr := "localhost:50052"

	conn, err := grpc.NewClient(pricingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to create Pricing client: %v", err)
	}
	defer conn.Close()

	pricingClient := pb.NewPricingServiceClient(conn)

	inventoryHandler := handler.NewInventoryHandler(inventoryStore, pricingClient)

	port := "50051"
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterInventoryServiceServer(grpcServer, inventoryHandler)

	fmt.Printf("Inventory Service running on port :%s...\n", port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC: %v", err)
	}
}