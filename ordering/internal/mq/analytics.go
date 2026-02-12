package mq

import (
	"time"

	// Import the generated FlatBuffers package
	"auto_grocery/ordering/fbs/AnalyticsMessages"

	flatbuffers "github.com/google/flatbuffers/go"
	zmq "github.com/pebbe/zmq4"
)

type AnalyticsPublisher struct {
	socket *zmq.Socket
}

// NewAnalyticsPublisher creates a PUB socket on the specified port (e.g., 5557)
func NewAnalyticsPublisher(port string) (*AnalyticsPublisher, error) {
	sock, err := zmq.NewSocket(zmq.Type(zmq.PUB))
	if err != nil {
		return nil, err
	}
	// Bind to the port so others (like Saugat's module) can SUBSCRIBE
	addr := "tcp://*:" + port
	if err := sock.Bind(addr); err != nil {
		return nil, err
	}
	return &AnalyticsPublisher{socket: sock}, nil
}

// Publish sends the OrderMetric FlatBuffer to any listeners
func (p *AnalyticsPublisher) Publish(orderID string, status string, duration float64) error {
	builder := flatbuffers.NewBuilder(1024)

	// 1. Create Strings first (FlatBuffers rule: strings must be created before the table starts)
	oID := builder.CreateString(orderID)
	stat := builder.CreateString(status)

	// 2. Start the Object
	AnalyticsMessages.OrderMetricStart(builder)
	AnalyticsMessages.OrderMetricAddOrderId(builder, oID)
	AnalyticsMessages.OrderMetricAddStatus(builder, stat)
	AnalyticsMessages.OrderMetricAddDurationSeconds(builder, duration)
	AnalyticsMessages.OrderMetricAddTimestamp(builder, time.Now().Unix()) // Add current timestamp
	metric := AnalyticsMessages.OrderMetricEnd(builder)

	// 3. Finish the Buffer
	builder.Finish(metric)
	payload := builder.FinishedBytes()

	// 4. Send via ZeroMQ
	// We don't use a topic here (sending empty string as topic), 
	// but you could add "metrics" as a prefix if you wanted.
	_, err := p.socket.SendBytes(payload, 0)
	return err
}

func (p *AnalyticsPublisher) Close() {
	p.socket.Close()
}