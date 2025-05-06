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
	"github.com/google/uuid"
)

var (
	LISTEN_ADDR       string
	KAFKA_BROKERS     []string
	KAFKA_TOPIC       string
	kafkaProducer     *kafka.KafkaProducer
	CONSUMER_GROUP_ID string
)

type ServiceEventHandler struct{}

func main() {
	BROKER_LIST := env.GetEnv("KAFKA_BROKERS", "kafka:9092")
	KAFKA_BROKERS = strings.Split(BROKER_LIST, ",")
	KAFKA_TOPIC = env.GetEnv("KAFKA_ORDERS_TOPIC", "order-events")
	LISTEN_ADDR = env.GetEnv("ORDER_SERVICE_LISTEN_ADDR", ":8080")
	CONSUMER_GROUP_ID = env.GetEnv("ORDER_SERVICE_GROUP", "order-service-group")

	// Init Producer
	var err error
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
	orderEventHandler := &ServiceEventHandler{}
	consumerGroup, err := kafka.NewConsumerGroup(KAFKA_BROKERS, KAFKA_TOPIC, CONSUMER_GROUP_ID, orderEventHandler)
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
	http.HandleFunc("/create-order", handleCreateOrder)
	if err := http.ListenAndServe(LISTEN_ADDR, nil); err != nil {
		log.Fatalf("Failed to start Order Service HTTP server: %v", err)
	}

	wg.Wait()
}

func handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" || !strings.HasPrefix(contentType, "application/json") {
		http.Error(w, "Content-Type header must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	defer r.Body.Close()
	var req event.OrderData
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // ignore other fields not in our struct
	err := decoder.Decode(&req)
	if err != nil {
		log.Println(err)
		http.Error(w, "Error decoding order request", http.StatusBadRequest)
		return
	}

	// Normally you'd parse order details from the req body
	orderID := uuid.New().String()
	// orderID := "abc123"
	orderData := event.OrderData{
		UserId:    req.UserId,
		UserEmail: req.UserEmail,
		Item:      req.Item,
		Amount:    req.Amount,
	}
	ev := event.CloudEvent{
		ID:              orderID,
		Source:          "OrderService",
		Type:            event.OrderPlacedType,
		DataContentType: "application/json",
		Time:            time.Now().UTC(),
		Data:            orderData,
	}
	b, err := json.Marshal(ev)

	if err != nil {
		log.Printf("(OrderService) Error marshal-ing event %s: %v", orderID, err)
		http.Error(w, "Error placing order", http.StatusInternalServerError)
		return
	}

	partition, offset, err := kafkaProducer.Publish(KAFKA_TOPIC, orderID, b)

	if err != nil {
		log.Printf("(OrderService) Error publishing event %s to topic %s: %v", orderID, KAFKA_TOPIC, err)
		http.Error(w, "Failed to broadcast order event", http.StatusInternalServerError)
		return
	}

	log.Printf("(OrderService) has successfully broadcasted event: %s, orderId: %s successfully sent to topic(%s)/partition(%d)/offset(%d)",
		event.OrderPlacedType, orderID, KAFKA_TOPIC, partition, offset)

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Order received: " + orderID))
}

func (h *ServiceEventHandler) Handle(ev event.CloudEvent) error {
	switch ev.Type {
	case "com.ecommerce.OrderCancelled":
		// handleOrderCancelled(ev)

		// default:
		// log.Printf("  -> Skipping event type '%s' by ServiceEventHandler", ev.Type)
	}
	return nil
}
