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

// NewRouter wires all HTTP routes, middleware, and handler dependencies for ordering service APIs.
func NewRouter(
	clientStore *store.ClientStore,
	orderStore *store.OrderStore,
	restockStore *store.RestockStore,
	inventoryClient pb.InventoryServiceClient,
	analyticsPub *mq.AnalyticsPublisher,
) *http.ServeMux {

	clientH := client.NewHandler(clientStore, orderStore, inventoryClient, analyticsPub)
	truckH := truck.NewHandler(restockStore, inventoryClient, analyticsPub)

	mux := http.NewServeMux()

	// --- Client API ---
	mux.HandleFunc("POST /api/client/register", clientH.Register)
	mux.HandleFunc("POST /api/client/login", clientH.Login)
	mux.HandleFunc("POST /api/client/refresh", clientH.Refresh)

	// --- Truck API ---
	mux.HandleFunc("POST /api/truck/restock", truckH.Restock)
	mux.HandleFunc("GET /api/truck/restock/status", truckH.RestockStatus)

	// --- Internal Webhooks (Protected by X-Internal-Secret) ---
	mux.Handle("POST /internal/webhook/update-order", auth.InternalMiddleware(http.HandlerFunc(clientH.Webhook)))
	mux.Handle("POST /internal/webhook/update-restock", auth.InternalMiddleware(http.HandlerFunc(truckH.Webhook)))

	// --- Protected Client Routes (Requires JWT) ---
	protected := func(h http.Handler) http.Handler {
		return auth.AuthMiddleware(h)
	}

	mux.Handle("POST /api/client/order/preview", protected(http.HandlerFunc(clientH.PreviewOrder)))
	mux.Handle("POST /api/client/order/confirm", protected(http.HandlerFunc(clientH.ConfirmOrder)))
	mux.Handle("POST /api/client/order/cancel", protected(http.HandlerFunc(clientH.CancelOrder)))
	mux.Handle("GET /api/client/orders", protected(http.HandlerFunc(clientH.History)))
	mux.Handle("GET /api/client/orders/last", protected(http.HandlerFunc(clientH.LastOrder)))

	return mux
}
