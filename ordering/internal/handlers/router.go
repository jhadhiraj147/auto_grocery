package handlers

import (
	"net/http"

	"auto_grocery/ordering/internal/auth" // <--- Import Auth
	"auto_grocery/ordering/internal/handlers/client"
	"auto_grocery/ordering/internal/handlers/truck"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
)

func NewRouter(
	clientStore *store.ClientStore,
	truckStore *store.TruckStore,
	orderStore *store.OrderStore,
	restockStore *store.RestockStore,
	inventoryClient pb.InventoryServiceClient,
) *http.ServeMux {
	
	mux := http.NewServeMux()

	// --- PUBLIC ROUTES (No Auth Needed) ---
	clientAuth := client.NewAuthHandler(clientStore)
	mux.HandleFunc("POST /api/client/register", clientAuth.Register)
	mux.HandleFunc("POST /api/client/login", clientAuth.Login)

	// --- TRUCK ROUTES (Currently No Auth / Public) ---
	truckHandler := truck.NewTruckHandler(truckStore, restockStore, inventoryClient)
	mux.HandleFunc("POST /api/truck/register", truckHandler.Register)
	mux.HandleFunc("POST /api/truck/restock", truckHandler.Restock)

	// --- PROTECTED ROUTES (Require Login) ---
	clientOrders := client.NewOrderHandler(orderStore, inventoryClient)

	// We create a helper to wrap handlers easily
	protected := func(handlerFunc http.HandlerFunc) http.HandlerFunc {
		// Convert HandlerFunc -> Handler -> Wrap with Middleware -> Convert back
		return auth.AuthMiddleware(http.HandlerFunc(handlerFunc)).ServeHTTP
	}

	// Now we wrap the order endpoints!
	mux.HandleFunc("POST /api/client/order/preview", protected(clientOrders.PreviewOrder))
	mux.HandleFunc("POST /api/client/order/confirm", protected(clientOrders.ConfirmOrder))
	mux.HandleFunc("POST /api/client/order/cancel", protected(clientOrders.CancelOrder))
	mux.HandleFunc("GET /api/client/orders", protected(clientOrders.GetHistory))
	mux.HandleFunc("GET /api/client/orders/last", protected(clientOrders.GetLastOrder))

	return mux
}