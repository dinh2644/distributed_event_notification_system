package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dinh2644/distributed_event_notification/pkg/env"
	"github.com/dinh2644/distributed_event_notification/pkg/event"
	"github.com/dinh2644/distributed_event_notification/pkg/kafka"
	"github.com/dinh2644/distributed_event_notification/pkg/redisclient"
)

var (
	LISTEN_ADDR       string
	KAFKA_BROKERS     []string
	KAFKA_TOPIC       string
	kafkaProducer     *kafka.KafkaProducer
	CONSUMER_GROUP_ID string
	redisClient       *redisclient.Client
)

type ServiceEventHandler struct {
	// dependencies needed by handler
	Redis        *redisclient.Client
	processedIDs *sync.Map
}

func main() {
	BROKER_LIST := env.GetEnv("KAFKA_BROKERS", "kafka:9092")
	KAFKA_BROKERS = strings.Split(BROKER_LIST, ",")
	KAFKA_TOPIC = env.GetEnv("KAFKA_ORDERS_TOPIC", "order-events")
	LISTEN_ADDR = env.GetEnv("INVENTORY_SERVICE_LISTEN_ADDR", ":8082")
	CONSUMER_GROUP_ID = env.GetEnv("INVENTORY_SERVICE_GROUP", "inventory-service-group")

	// Init Redis Client
	ctxStartup, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()
	var err error
	redisClient, err = redisclient.New(ctxStartup)
	if err != nil {
		log.Fatalf("Failed to initialize Redis client: %v", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("Error closing Redis client: %v", err)
		}
	}()

	// Init Producer
	kafkaProducer, err = kafka.NewProducer(KAFKA_BROKERS)
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			log.Printf("Error closing Kafka producer: %v", err)
		}
	}()

	// Init Consumer Group
	inventoryEventHandler := &ServiceEventHandler{
		Redis:        redisClient,
		processedIDs: &sync.Map{}}
	consumerGroup, err := kafka.NewConsumerGroup(KAFKA_BROKERS, KAFKA_TOPIC, CONSUMER_GROUP_ID, inventoryEventHandler)
	if err != nil {
		log.Fatalf("Failed to create Kafka consumer group: %v", err)
	}
	defer func() {
		if err := consumerGroup.Close(); err != nil {
			log.Printf("Error closing Kafka consumer group: %v", err)
		}
	}()

	// Start Consumer Loop
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go consumerGroup.Run(ctx, &wg)
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigterm
		log.Println("Termination signal received. Initiating shutdown...")
		cancel()
	}()

	// Start HTTP server
	if err := http.ListenAndServe(LISTEN_ADDR, nil); err != nil {
		log.Fatalf("Failed to start Order Service HTTP server: %v", err)
	}

	wg.Wait()
}

func (h *ServiceEventHandler) Handle(ev event.CloudEvent) error {
	switch ev.Type {
	case event.PaymentConfirmedType:
		h.handlePaymentConfirmed(ev)

		// default:
		// log.Printf("  -> Skipping event type '%s' by OrderServiceEventHandler", ev.Type)
	}
	return nil
}

func (h *ServiceEventHandler) handlePaymentConfirmed(ev event.CloudEvent) error {
	orderID := ev.ID
	ctx := context.Background()

	// Check if orderId exists
	if _, exists := h.processedIDs.Load(orderID); exists {
		log.Printf("(PaymentService) SKIP: OrderID %s already processed (in-memory cache)", orderID)
		return nil
	}

	log.Printf("(InventoryService) Received PaymentConfirmed event (OrderID: %s)", orderID)

	// Get current state of orderId
	currentState, errRedisGet := h.Redis.GetOrderState(ctx, orderID)
	if errRedisGet != nil && errRedisGet != redisclient.ErrStateNotFound { // not found = havent processed it yet, also checks additionally for redis comms errors
		log.Printf("(InventoryService) ERROR: Failed to get state for OrderID %s from Redis: %v", orderID, errRedisGet)
		return errRedisGet
	}

	// Check if prerequisite state is met
	if currentState != redisclient.StatePaymentConfirmed {
		log.Printf("(InventoryService) SKIP: Skipping PaymentConfirmed for OrderID %s because prerequisite state '%s' is not met (Current State: '%s')...",
			orderID, redisclient.StatePaymentConfirmed, currentState)
		return nil
	}

	// Idempotency check
	if errRedisGet == nil && (currentState == redisclient.StateInventoryUpdated) {
		log.Printf("(InventoryService) SKIP: OrderID %s already processed (State: %s). Skipping duplicate PaymentConfirmed.", orderID, currentState)
		return nil
	}

	// Acknowledge interested event(s)
	log.Printf("(InventoryService) Acknowledged event: com.ecommerce.PaymentConfirmed\n[LOG] Which OrderId: %s\n[LOG] From topic: %s", ev.ID, KAFKA_TOPIC)

	// Handle broadcast and state update
	orderData := event.OrderData{
		UserId:    ev.Data.UserId,
		UserEmail: ev.Data.UserEmail,
		Item:      ev.Data.Item,
		Amount:    ev.Data.Amount,
	}
	newEv := event.CloudEvent{
		ID:              ev.ID,
		Source:          "InventoryService",
		Type:            event.InventoryUpdatedType,
		DataContentType: "application/json",
		Time:            time.Now().UTC(),
		Data:            orderData,
	}
	b, errMarshal := json.Marshal(newEv)

	var publishErr error
	newState := redisclient.StateInventoryUpdated

	if errMarshal != nil {
		log.Printf("(InventoryService) Error marshal-ing event %s: %v", orderID, errMarshal)
		newState = redisclient.StatePaymentConfirmed
		publishErr = errMarshal
	} else {
		var partition int32
		var offset int64
		partition, offset, publishErr = kafkaProducer.Publish(KAFKA_TOPIC, orderID, b)
		if publishErr != nil {
			log.Printf("(InventoryService) Error publishing event %s to topic %s: %v", orderID, KAFKA_TOPIC, publishErr)
			newState = redisclient.StatePaymentConfirmed
		} else {
			log.Printf("(InventoryService) has successfully broadcasted event: %s, orderId: %s successfully sent to topic(%s)/partition(%d)/offset(%d)",
				event.InventoryUpdatedType, orderID, KAFKA_TOPIC, partition, offset)
		}
	}

	// Update State
	h.processedIDs.Store(orderID, struct{}{}) // and cache of processed orderIds
	errRedisSet := h.Redis.SetOrderState(ctx, orderID, newState, 24*time.Hour)
	if errRedisSet != nil {
		log.Printf("(InventoryService) CRITICAL ERROR: Failed to set state to %s for OrderID %s AFTER processing/publishing: %v", newState, orderID, errRedisSet)
		return errRedisSet
	}
	log.Printf("(InventoryService) State set to %s for OrderID %s", newState, orderID)

	return publishErr
}
