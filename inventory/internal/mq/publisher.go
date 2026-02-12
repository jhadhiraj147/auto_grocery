package mq

import (
	"auto_grocery/inventory/fbs/RobotMessages"

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

func NewPublisher(port string) (*Publisher, error) {
	sock, err := zmq.NewSocket(zmq.Type(zmq.PUB))
	if err != nil {
		return nil, err
	}
	addr := "tcp://*:" + port
	if err := sock.Bind(addr); err != nil {
		return nil, err
	}
	return &Publisher{socket: sock}, nil
}

func (p *Publisher) SendRobotCommand(orderID string, items map[string]ItemDetails) error {
	builder := flatbuffers.NewBuilder(1024)

	var itemOffsets []flatbuffers.UOffsetT
	for sku, detail := range items {
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
	RobotMessages.OrderBroadcastStart(builder)
	RobotMessages.OrderBroadcastAddOrderId(builder, oid)
	RobotMessages.OrderBroadcastAddItems(builder, itemsVec)
	order := RobotMessages.OrderBroadcastEnd(builder)

	builder.Finish(order)
	payload := builder.FinishedBytes()

	_, err := p.socket.SendBytes(payload, 0)
	return err
}

func (p *Publisher) Close() {
	p.socket.Close()
}
