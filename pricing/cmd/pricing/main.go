package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "net"
    "os"
    "time"

    "auto_grocery/pricing/internal/handler"
    "auto_grocery/pricing/internal/logic" 
    "auto_grocery/pricing/internal/store"
    pb "auto_grocery/pricing/proto"

    "github.com/joho/godotenv"
    _ "github.com/lib/pq"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    // 1. Load Environment Variables
    _ = godotenv.Load("pricing/.env")

    // 2. Database Connection
    db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatalf("failed to connect to database: %v", err)
    }
    defer db.Close()

    // 3. Initialize layers using names from your store package
    catalogStore := store.NewCatalogStore(db) 
    pricingHandler := handler.NewPricingHandler(catalogStore)

    // 4. Start the background worker (Non-blocking)
    go startInventoryPriceRefresher(catalogStore)

    // 5. Start gRPC Server on port 50052
    lis, err := net.Listen("tcp", ":50052")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }

    grpcServer := grpc.NewServer()
    pb.RegisterPricingServiceServer(grpcServer, pricingHandler)

    fmt.Println("🚀 Pricing Service running on :50052...")
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}

func startInventoryPriceRefresher(catalogStore *store.CatalogStore) {
    // Allow Inventory Service time to start
    time.Sleep(5 * time.Second)

    // Connect to Inventory Service (Port 50051)
    conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Printf("⚠️ Could not setup Inventory client: %v", err)
        return
    }
    defer conn.Close()

    inventoryClient := pb.NewInventoryServiceClient(conn)
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop() // Added: Prevent ticker leak

    fmt.Println("🔄 Hourly Background Re-Pricer is active")

    for range ticker.C {
        // Create context for the fetch operation
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        
        // A. Request metrics from Inventory
        resp, err := inventoryClient.GetInventoryMetrics(ctx, &pb.GetInventoryMetricsRequest{})
        if err != nil {
            log.Printf("❌ Metrics fetch failed: %v", err)
            cancel()
            continue
        }

        // B. Apply Logic & Update Pricing DB
        for _, metric := range resp.GetMetrics() {
            // Calculate new price based on CP and stock levels
            newPrice := logic.CalculatePrice(metric.UnitCost, int(metric.Quantity))

            // Sync with minimalist 3-column table
            _, err := catalogStore.UpsertItem(ctx, store.Item{
                Sku:       metric.Sku,      // Matches 'Sku' in struct
                UnitPrice: newPrice,        // Matches 'UnitPrice' in struct
            })

            if err != nil {
                log.Printf("❌ Update failed for %s: %v", metric.Sku, err)
            } else {
                fmt.Printf("✅ Re-Priced %s: $%.2f (Cost: $%.2f, Stock: %d)\n", 
                    metric.Sku, newPrice, metric.UnitCost, metric.Quantity)
            }
        }
        cancel() // Always cancel context after the loop iteration
    }
}