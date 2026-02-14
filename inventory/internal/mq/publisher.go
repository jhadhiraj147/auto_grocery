package mq

import (
	"auto_grocery/inventory/fbs/RobotMessages"
	"log"

	flatbuffers "github.com/google/flatbuffers/go"
	zmq "github.com/pebbe/zmq4"
)

type ItemDetails struct {
	Quantity int32
	Aisle    string
}

type Publisher struct {
	socket *zmq.Socket
}

// NewPublisher creates and binds a ZeroMQ PUB socket on the requested port.
func NewPublisher(bindAddr string) (*Publisher, error) {
	sock, err := zmq.NewSocket(zmq.Type(zmq.PUB))
	if err != nil {
		return nil, err
	}
	if err := sock.Bind(bindAddr); err != nil {
		return nil, err
	}
	log.Printf("[inventory-pub] bound zmq publisher at %s", bindAddr)
	return &Publisher{socket: sock}, nil
}

// SendRobotCommand serializes and broadcasts robot commands for a specific order type.
func (p *Publisher) SendRobotCommand(orderID string, orderType string, items map[string]ItemDetails) error {
	log.Printf("[inventory-pub] building broadcast order=%s type=%s items=%d", orderID, orderType, len(items))
	builder := flatbuffers.NewBuilder(1024)

	var itemOffsets []flatbuffers.UOffsetT
	for sku, detail := range items {
		log.Printf("[inventory-pub] item order=%s sku=%s qty=%d aisle=%s", orderID, sku, detail.Quantity, detail.Aisle)
		s := builder.CreateString(sku)
		a := builder.CreateString(detail.Aisle)

		RobotMessages.ItemStart(builder)
		RobotMessages.ItemAddSku(builder, s)
		RobotMessages.ItemAddQuantity(builder, detail.Quantity)
		RobotMessages.ItemAddAisle(builder, a)
		itemOffsets = append(itemOffsets, RobotMessages.ItemEnd(builder))
	}

	RobotMessages.OrderBroadcastStartItemsVector(builder, len(itemOffsets))
	for i := len(itemOffsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(itemOffsets[i])
	}
	itemsVec := builder.EndVector(len(itemOffsets))

	oid := builder.CreateString(orderID)
	otype := builder.CreateString(orderType) // Create string for orderType

	RobotMessages.OrderBroadcastStart(builder)
	RobotMessages.OrderBroadcastAddOrderId(builder, oid)
	RobotMessages.OrderBroadcastAddOrderType(builder, otype) // Add orderType to builder
	RobotMessages.OrderBroadcastAddItems(builder, itemsVec)
	order := RobotMessages.OrderBroadcastEnd(builder)

	builder.Finish(order)
	payload := builder.FinishedBytes()

	_, err := p.socket.SendBytes(payload, 0)
	if err != nil {
		log.Printf("[inventory-pub] send failed order=%s err=%v", orderID, err)
		return err
	}
	log.Printf("[inventory-pub] broadcast sent order=%s bytes=%d", orderID, len(payload))
	return err
}

// Close releases publisher socket resources.
func (p *Publisher) Close() {
	p.socket.Close()
}
