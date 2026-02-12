package handlers

import (
	"net/http"

	"auto_grocery/ordering/internal/auth"
	"auto_grocery/ordering/internal/handlers/client"
	"auto_grocery/ordering/internal/handlers/truck"
	"auto_grocery/ordering/internal/mq"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
)

func NewRouter(
	clientStore *store.ClientStore,
	truckStore *store.TruckStore,
	orderStore *store.OrderStore,
	restockStore *store.RestockStore,
	inventoryClient pb.InventoryServiceClient,
	analyticsPub *mq.AnalyticsPublisher,
) *http.ServeMux {

	mux := http.NewServeMux()

	// --- 1. Client Auth ---
	mux.Handle("POST /api/client/register", &client.RegisterHandler{Store: clientStore})
	mux.Handle("POST /api/client/login",    &client.LoginHandler{Store: clientStore})
	mux.Handle("POST /api/client/refresh",  &client.RefreshHandler{Store: clientStore})

	// --- 2. Trucks ---
	mux.Handle("POST /api/truck/register", &truck.RegisterHandler{TruckStore: truckStore})
	mux.Handle("POST /api/truck/restock",  &truck.RestockHandler{
		TruckStore:      truckStore,
		RestockStore:    restockStore,
		InventoryClient: inventoryClient,
	})

	// --- 3. INTERNAL WEBHOOK (Protected by Secret Key) ---
	// UPDATED: We wrap the handler with InternalMiddleware
	webhookHandler := &client.WebhookHandler{
		OrderStore: orderStore,
		Analytics:  analyticsPub,
	}
	mux.Handle("POST /internal/webhook/update-order", auth.InternalMiddleware(webhookHandler))

	// --- 4. Protected Client Routes ---
	protected := func(h http.Handler) http.Handler {
		return auth.AuthMiddleware(h)
	}

	mux.Handle("POST /api/client/order/preview", protected(&client.PreviewOrderHandler{
		OrderStore:      orderStore,
		InventoryClient: inventoryClient,
	}))

	mux.Handle("POST /api/client/order/confirm", protected(&client.ConfirmOrderHandler{
		OrderStore:      orderStore,
		InventoryClient: inventoryClient,
	}))

	mux.Handle("POST /api/client/order/cancel", protected(&client.CancelOrderHandler{
		OrderStore:      orderStore,
		InventoryClient: inventoryClient,
	}))

	mux.Handle("GET /api/client/orders", protected(&client.HistoryHandler{
		OrderStore: orderStore,
	}))

	mux.Handle("GET /api/client/orders/last", protected(&client.LastOrderHandler{
		OrderStore: orderStore,
	}))

	return mux
}