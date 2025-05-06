// your-project-root/kafka/consumer.go
package kafka

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/IBM/sarama"
	"github.com/dinh2644/distributed_event_notification/pkg/event"
)

// KafkaConsumer manages the Sarama consumer connection and channels.
type KafkaConsumer struct {
	// consumer sarama.PartitionConsumer
	// worker   sarama.Consumer

	Errors   <-chan *sarama.ConsumerError
	Messages <-chan *sarama.ConsumerMessage
	// future note: internal channel to signal shutdown to any running loops (if needed)
	// done chan struct{} // May not be needed if main controls the loop
}

type ConsumerGroup struct {
	cg      sarama.ConsumerGroup
	brokers []string
	topic   string
	groupID string
	handler EventHandler
}

func NewConsumerGroup(brokers []string, topic string, groupID string, handler EventHandler) (*ConsumerGroup, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_1_0
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRange()

	cg, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		log.Printf("Error creating consumer group client (groupID: %s): %v", groupID, err)
		return nil, err
	}
	// log.Printf("Consumer Group client created (groupID: %s)", groupID)

	return &ConsumerGroup{
		cg:      cg,
		brokers: brokers,
		topic:   topic,
		groupID: groupID,
		handler: handler,
	}, nil
}

type EventHandler interface {
	Handle(ev event.CloudEvent) error
}

// Run starts the consumer group processing loop (it takes a context for cancellation)
func (c *ConsumerGroup) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done() // signal WaitGroup when loop exits

	// Create an instance of the handler for this consumption session
	handlerInstance := &consumerGroupHandler{
		eventHandler: c.handler, // pass the specific event handler logic
	}

	log.Printf("Run(): Consumer group '%s' starting consumption loop for topic '%s'", c.groupID, c.topic)
	for {
		// Consume waits for rebalances and assigns partitions
		// The handler's ConsumeClaim will be called for assigned partitions
		err := c.cg.Consume(ctx, []string{c.topic}, handlerInstance)
		if err != nil {
			// Errors like ErrClosedConsumerGroup happen on shutdown, others might be recoverable
			log.Printf("Error from consumer group '%s': %v", c.groupID, err)
		}

		// Check if context was cancelled, signaling shutdown
		if ctx.Err() != nil {
			log.Printf("Context cancelled for consumer group '%s'. Exiting loop.", c.groupID)
			return // context is cancelled (so exit loop)
		}

		// If Consume returns without context cancellation, it might mean a recoverable error
		// or a rebalance happened
		// The loop will retry consumption
		log.Printf("Consumer group '%s' encountered an issue, will re-attempt consumption...", c.groupID)
	}
}

func (c *ConsumerGroup) Close() error {
	log.Printf("Close(): Closing consumer group '%s'", c.groupID)
	if c.cg != nil {
		return c.cg.Close()
	}
	return nil
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	eventHandler EventHandler // the logic to apply to filtered events
	msgCnt       int          // track message count per session/rebalance
	processedCnt int          // track processed count per session/rebalance
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	log.Printf("Consumer group handler SETUP for generation %d. Claims: %v", session.GenerationID(), session.Claims())
	h.msgCnt = 0 // reset counters for the new session if needed
	h.processedCnt = 0
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *consumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	log.Printf("Cleanup(): Consumer group handler CLEANUP for generation %d.", session.GenerationID())
	log.Printf("   Session stats: Received: %d, Processed: %d", h.msgCnt, h.processedCnt)
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages() (once the Messages() channel is closed, the handler must finish its processing loop)
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// Single log at the beginning of consumption
	log.Printf("Starting consumer for topic '%s', partition %d", claim.Topic(), claim.Partition())

	for message := range claim.Messages() {
		h.msgCnt++

		var ev event.CloudEvent
		err := json.Unmarshal(message.Value, &ev)
		if err != nil {
			log.Printf("ERROR: Failed to unmarshal message from offset %d: %v", message.Offset, err)
			session.MarkMessage(message, "")
			continue
		}

		// Let the event handler do the filtering and processing
		err = h.eventHandler.Handle(ev)
		if err != nil {
			log.Printf("ERROR: Handler failed for event ID %s: %v", ev.ID, err)
		} else {
			h.processedCnt++
		}

		// Mark message as consumed
		session.MarkMessage(message, "")
	}

	log.Printf("Consumer stopped for topic '%s', partition %d", claim.Topic(), claim.Partition())
	return nil
}
