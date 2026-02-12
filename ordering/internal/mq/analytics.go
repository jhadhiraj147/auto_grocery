package mq

import (
	"time"

	"auto_grocery/ordering/fbs/AnalyticsMessages"

	flatbuffers "github.com/google/flatbuffers/go"
	zmq "github.com/pebbe/zmq4"
)

type AnalyticsPublisher struct {
	socket *zmq.Socket
}

func NewAnalyticsPublisher(port string) (*AnalyticsPublisher, error) {
	sock, err := zmq.NewSocket(zmq.Type(zmq.PUB))
	if err != nil {
		return nil, err
	}
	addr := "tcp://*:" + port
	if err := sock.Bind(addr); err != nil {
		return nil, err
	}
	return &AnalyticsPublisher{socket: sock}, nil
}

func (p *AnalyticsPublisher) Publish(orderID string, status string, duration float64) error {
	builder := flatbuffers.NewBuilder(1024)

	oID := builder.CreateString(orderID)
	stat := builder.CreateString(status)

	AnalyticsMessages.OrderMetricStart(builder)
	AnalyticsMessages.OrderMetricAddOrderId(builder, oID)
	AnalyticsMessages.OrderMetricAddStatus(builder, stat)
	AnalyticsMessages.OrderMetricAddDurationSeconds(builder, duration)
	AnalyticsMessages.OrderMetricAddTimestamp(builder, time.Now().Unix())
	metric := AnalyticsMessages.OrderMetricEnd(builder)

	builder.Finish(metric)
	payload := builder.FinishedBytes()

	_, err := p.socket.SendBytes(payload, 0)
	return err
}

func (p *AnalyticsPublisher) Close() {
	p.socket.Close()
}
