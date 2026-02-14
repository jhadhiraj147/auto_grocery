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

	mux := http.NewServeMux()

	// --- Client API ---
	mux.Handle("POST /api/client/register", &client.RegisterHandler{Store: clientStore})
	mux.Handle("POST /api/client/login", &client.LoginHandler{Store: clientStore})
	mux.Handle("POST /api/client/refresh", &client.RefreshHandler{Store: clientStore})

	// --- Truck API ---
	mux.Handle("POST /api/truck/restock", &truck.RestockHandler{
		RestockStore:    restockStore,
		InventoryClient: inventoryClient,
	})
	mux.Handle("GET /api/truck/restock/status", &truck.RestockStatusHandler{
		RestockStore: restockStore,
	})

	// --- Internal Webhooks (Protected by X-Internal-Secret) ---
	clientWebhook := &client.WebhookHandler{
		OrderStore: orderStore,
		Analytics:  analyticsPub,
	}
	mux.Handle("POST /internal/webhook/update-order", auth.InternalMiddleware(clientWebhook))

	truckWebhook := &truck.WebhookHandler{
		RestockStore: restockStore,
		Analytics:    analyticsPub,
	}
	mux.Handle("POST /internal/webhook/update-restock", auth.InternalMiddleware(truckWebhook))

	// --- Protected Client Routes (Requires JWT) ---
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
