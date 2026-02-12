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
	_ = godotenv.Load("pricing/.env")

	dbConnString := os.Getenv("DATABASE_URL")
	if dbConnString == "" {
		dbConnString = "postgres://user:password@localhost:5432/pricing_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbConnString)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	catalogStore := store.NewCatalogStore(db)
	pricingHandler := handler.NewPricingHandler(catalogStore)

	go startInventoryPriceRefresher(catalogStore)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPricingServiceServer(grpcServer, pricingHandler)

	fmt.Println("Pricing Service running on :50052...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func startInventoryPriceRefresher(catalogStore *store.CatalogStore) {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Could not setup Inventory client: %v", err)
		return
	}
	defer conn.Close()

	inventoryClient := pb.NewInventoryServiceClient(conn)
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	fmt.Println("Hourly Background Re-Pricer is active")

	updatePrices := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := inventoryClient.GetInventoryMetrics(ctx, &pb.GetInventoryMetricsRequest{})
		if err != nil {
			log.Printf("Metrics fetch failed: %v", err)
			return
		}

		for _, metric := range resp.GetMetrics() {
			newPrice := logic.CalculatePrice(metric.UnitCost, int(metric.Quantity))

			_, err := catalogStore.UpsertItem(ctx, store.Item{
				Sku:       metric.Sku,
				UnitPrice: newPrice,
			})
			if err != nil {
				log.Printf("Update failed for %s: %v", metric.Sku, err)
			} else {
				fmt.Printf("Re-Priced %s: %.2f (Cost: %.2f, Stock: %d)\n",
					metric.Sku, newPrice, metric.UnitCost, metric.Quantity)
			}
		}
	}

	updatePrices()

	for range ticker.C {
		updatePrices()
	}
}