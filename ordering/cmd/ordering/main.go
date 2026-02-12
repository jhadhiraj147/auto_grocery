package main

import (
    "database/sql"
    "fmt"
    "log"
    "net/http"
    "os"

    handlers "auto_grocery/ordering/internal/handlers"
    "auto_grocery/ordering/internal/mq" // <--- IMPORT THIS
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
    if err := godotenv.Load("ordering/.env"); err != nil {
        log.Println("Note: No ordering/.env file found (or failed to load)")
    }

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

    invConn, err := grpc.Dial(inventoryAddr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatalf("❌ Could not connect to Inventory Service: %v", err)
    }
    defer invConn.Close()
    fmt.Println("✅ Inventory Service Client Initialized")

    inventoryClient := pb.NewInventoryServiceClient(invConn)

    // ---------------------------------------------------------
    // 4. ANALYTICS PUBLISHER (ZeroMQ) <--- NEW SECTION
    // ---------------------------------------------------------
    analyticsPort := "5557"
    fmt.Printf("📊 Starting Analytics Publisher on port %s...\n", analyticsPort)
    
    analyticsPub, err := mq.NewAnalyticsPublisher(analyticsPort)
    if err != nil {
        log.Fatalf("❌ Failed to start Analytics Publisher: %v", err)
    }
    defer analyticsPub.Close() // Don't forget to close it!

    // ---------------------------------------------------------
    // 5. INITIALIZE STORES
    // ---------------------------------------------------------
    clientStore := store.NewClientStore(db)
    truckStore := store.NewTruckStore(db)
    orderStore := store.NewOrderStore(db)
    restockStore := store.NewRestockStore(db)

    // ---------------------------------------------------------
    // 6. SETUP ROUTER
    // ---------------------------------------------------------
    // We now pass 'analyticsPub' to the router!
    mux := handlers.NewRouter(clientStore, truckStore, orderStore, restockStore, inventoryClient, analyticsPub)

    // ---------------------------------------------------------
    // 7. START SERVER
    // ---------------------------------------------------------
    port := "5050"
    fmt.Printf("🚀 Ordering Service running on http://localhost:%s\n", port)

    if err := http.ListenAndServe(":"+port, mux); err != nil {
        log.Fatalf("❌ Server crashed: %v", err)
    }
}