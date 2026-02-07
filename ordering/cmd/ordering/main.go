package main

import (
	"context" // <--- ADDED THIS
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"    // <--- ADDED THIS

	handlers "auto_grocery/ordering/internal/handlers"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// ---------------------------------------------------------
	// 1. CONFIGURATION
	// ---------------------------------------------------------
	_ = godotenv.Load("ordering/.env")

	// ---------------------------------------------------------
	// 2. DATABASE CONNECTION
	// ---------------------------------------------------------
	dbConnString := os.Getenv("DATABASE_URL")
	if dbConnString == "" {
		dbConnString = "postgres://user:password@localhost:5432/ordering_db?sslmode=disable"
		fmt.Println("⚠️  DATABASE_URL not set, using default fallback")
	}

	db, err := sql.Open("postgres", dbConnString)
	if err != nil {
		log.Fatalf("❌ Failed to open DB driver: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Failed to ping Database: %v", err)
	}
	fmt.Println("✅ Connected to Ordering Database")

	// ---------------------------------------------------------
	// 3. INVENTORY SERVICE CONNECTION (gRPC)
	// ---------------------------------------------------------
	inventoryAddr := "localhost:50051"
	fmt.Printf("⏳ Connecting to Inventory Service at %s...\n", inventoryAddr)

	// --- FIX: ADD TIMEOUT ---
	// We give it exactly 5 seconds to find the Inventory Service.
	// If it takes longer, we assume it's down and crash.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	invConn, err := grpc.DialContext(ctx, inventoryAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), // Still blocks, but respects the 5s timeout above
	)
	if err != nil {
		log.Fatalf("❌ Could not connect to Inventory Service: %v", err)
	}
	defer invConn.Close()
	fmt.Println("✅ Connected to Inventory Service")

	// Create the gRPC Client
	inventoryClient := pb.NewInventoryServiceClient(invConn)

	// ---------------------------------------------------------
	// 4. INITIALIZE STORES
	// ---------------------------------------------------------
	clientStore := store.NewClientStore(db)
	truckStore := store.NewTruckStore(db)
	orderStore := store.NewOrderStore(db)
	restockStore := store.NewRestockStore(db)

	// ---------------------------------------------------------
	// 5. SETUP ROUTER
	// ---------------------------------------------------------
	mux := handlers.NewRouter(clientStore, truckStore, orderStore, restockStore, inventoryClient)

	// ---------------------------------------------------------
	// 6. START SERVER
	// ---------------------------------------------------------
	port := "5050"
	fmt.Printf("🚀 Ordering Service running on http://localhost:%s\n", port)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("❌ Server crashed: %v", err)
	}
}