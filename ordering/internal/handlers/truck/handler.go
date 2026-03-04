package truck

import (
	"auto_grocery/ordering/internal/mq"
	"auto_grocery/ordering/internal/store"
	pb "auto_grocery/ordering/proto"
)

// Handler holds all dependencies for truck-domain HTTP handlers.
// Each method only uses the fields it needs — unused fields are fine.
type Handler struct {
	restockStore    *store.RestockStore
	inventoryClient pb.InventoryServiceClient
	analytics       *mq.AnalyticsPublisher
}

func NewHandler(
	restockStore *store.RestockStore,
	inventoryClient pb.InventoryServiceClient,
	analytics *mq.AnalyticsPublisher,
) *Handler {
	return &Handler{
		restockStore:    restockStore,
		inventoryClient: inventoryClient,
		analytics:       analytics,
	}
}
