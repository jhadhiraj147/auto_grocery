package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"auto_grocery/ordering/internal/auth"
	handlers "auto_grocery/ordering/internal/handlers"
	"auto_grocery/ordering/internal/mq"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"

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

// main initializes infrastructure dependencies and starts the ordering HTTP server.
func main() {
	// 1. Load Environment Variables
	if err := godotenv.Load("ordering/.env"); err != nil {
		log.Println("[ordering] WARN no .env file found, relying on system environment variables")
	}

	// 2. Initialize JWT secret.
	if err := auth.InitJWTKey(); err != nil {
		log.Fatalf("[ordering] ERROR failed to initialize jwt secret: %v", err)
	}

	// 3. Database Connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("[ordering] ERROR DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 4. gRPC Connection
	inventoryGRPCAddr := getenv("INVENTORY_GRPC_ADDR", "localhost:50051")
	invConn, err := grpc.Dial(inventoryGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer invConn.Close()
	inventoryClient := pb.NewInventoryServiceClient(invConn)

	// 5. Analytics Publisher
	analyticsZMQBindAddr := getenv("ANALYTICS_ZMQ_BIND_ADDR", "tcp://*:5557")
	analyticsPub, err := mq.NewAnalyticsPublisher(analyticsZMQBindAddr)
	if err != nil {
		log.Printf("[ordering] WARN failed to connect to analytics zmq: %v", err)
	} else {
		defer analyticsPub.Close()
	}

	// 6. Stores & Router
	clientStore := store.NewClientStore(db)
	orderStore := store.NewOrderStore(db)
	restockStore := store.NewRestockStore(db)

	mux := handlers.NewRouter(clientStore, orderStore, restockStore, inventoryClient, analyticsPub)

	orderingHTTPAddr := getenv("ORDERING_HTTP_ADDR", ":5050")
	log.Printf("[ordering] INFO service listening on %s (inventory_grpc=%s analytics_bind=%s)", orderingHTTPAddr, inventoryGRPCAddr, analyticsZMQBindAddr)
	if err := http.ListenAndServe(orderingHTTPAddr, mux); err != nil {
		log.Fatal(err)
	}
}
