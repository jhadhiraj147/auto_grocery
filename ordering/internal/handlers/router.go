package handlers

import (
	"net/http"

	"auto_grocery/ordering/internal/auth"
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


	mux.Handle("POST /api/client/register", &client.RegisterHandler{Store: clientStore})
	mux.Handle("POST /api/client/login",    &client.LoginHandler{Store: clientStore})
	
	
	mux.Handle("POST /api/client/refresh",  &client.RefreshHandler{Store: clientStore})

	
	mux.Handle("POST /api/truck/register", &truck.RegisterHandler{TruckStore: truckStore})
	mux.Handle("POST /api/truck/restock",  &truck.RestockHandler{
		TruckStore:      truckStore,
		RestockStore:    restockStore,
		InventoryClient: inventoryClient,
	})

	
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