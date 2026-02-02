package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	"auto-grocery/pricing/internal/handler"
	"auto-grocery/pricing/internal/store"
	pb "auto-grocery/pricing/proto"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
)

func main() {
	// 1. Load the .env file from the microservice folder
	// If you run this from pricing/cmd/pricing, the path is ../../.env
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Println("Note: .env file not found, using system environment variables")
	}

	// 2. Use the "Shortcut" Connection String from your .env
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set in the environment")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// 3. Initialize layers
	catalogStore := store.NewCatalogStore(db)
	pricingHandler := handler.NewPricingHandler(catalogStore)

	// 4. Start the gRPC listener on the port defined in your README
	lis, err := net.Listen("tcp", ":50052") // Port 50052 as per architecture
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPricingServiceServer(grpcServer, pricingHandler)

	fmt.Println("🚀 Pricing Service is running on port :50052...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}