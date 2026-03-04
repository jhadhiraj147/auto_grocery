package client

import (
	"auto_grocery/ordering/internal/mq"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
)

// Handler holds all dependencies for client-domain HTTP handlers.
// Each method only uses the fields it needs — unused fields are fine.
type Handler struct {
	clientStore     *store.ClientStore
	orderStore      *store.OrderStore
	inventoryClient pb.InventoryServiceClient
	analytics       *mq.AnalyticsPublisher
}

func NewHandler(
	clientStore *store.ClientStore,
	orderStore *store.OrderStore,
	inventoryClient pb.InventoryServiceClient,
	analytics *mq.AnalyticsPublisher,
) *Handler {
	return &Handler{
		clientStore:     clientStore,
		orderStore:      orderStore,
		inventoryClient: inventoryClient,
		analytics:       analytics,
	}
}
