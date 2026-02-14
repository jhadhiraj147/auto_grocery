package main

import (
	"database/sql"
	"log"
	"net"
	"os"

	"auto_grocery/pricing/internal/handler"
	"auto_grocery/pricing/internal/store"
	pb "auto_grocery/pricing/proto"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
)

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

// main initializes dependencies and starts the pricing gRPC service.
func main() {
	// 1. Load Config
	_ = godotenv.Load("pricing/.env")
	log.Printf("[pricing] loading configuration")

	// 2. Connect to Database
	dbConnString := os.Getenv("DATABASE_URL")
	if dbConnString == "" {
		dbConnString = "postgres://user:password@localhost:5432/pricing_db?sslmode=disable"
		log.Printf("[pricing] DATABASE_URL not set, using fallback connection string")
	}
	log.Printf("[pricing] connecting to postgres")

	db, err := sql.Open("postgres", dbConnString)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Printf("[pricing] postgres connection established")

	// 3. Initialize Store and Handler
	catalogStore := store.NewCatalogStore(db)

	// Create the handler (Logic is now triggered via gRPC call from Inventory)
	pricingHandler := handler.NewPricingHandler(catalogStore)

	// 4. Start gRPC Server
	pricingGRPCAddr := getenv("PRICING_GRPC_ADDR", ":50052")
	lis, err := net.Listen("tcp", pricingGRPCAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("[pricing] gRPC listening on %s", pricingGRPCAddr)

	grpcServer := grpc.NewServer()
	pb.RegisterPricingServiceServer(grpcServer, pricingHandler)

	log.Printf("[pricing] INFO service listening on %s (event-driven mode)", pricingGRPCAddr)
	log.Printf("[pricing] server ready")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
