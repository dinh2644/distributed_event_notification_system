package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	gomail "gopkg.in/mail.v2"

	"github.com/dinh2644/distributed_event_notification/pkg/env"
	"github.com/dinh2644/distributed_event_notification/pkg/event"
	"github.com/dinh2644/distributed_event_notification/pkg/kafka"
)

var (
	LISTEN_ADDR        string
	KAFKA_BROKERS      []string
	KAFKA_TOPIC        string
	kafkaProducer      *kafka.KafkaProducer
	CONSUMER_GROUP_ID  string
	PERSONAL_GMAIL     string
	GMAIL_APP_PASSWORD string
)

type ServiceEventHandler struct{}

func main() {
	BROKER_LIST := env.GetEnv("KAFKA_BROKERS", "kafka:9092")
	KAFKA_BROKERS = strings.Split(BROKER_LIST, ",")
	KAFKA_TOPIC = env.GetEnv("KAFKA_ORDERS_TOPIC", "order-events")
	LISTEN_ADDR = env.GetEnv("NOTIFICATION_SERVICE_LISTEN_ADDR", ":8084")
	CONSUMER_GROUP_ID = env.GetEnv("NOTIFICATION_SERVICE_GROUP", "notification-service-group")
	PERSONAL_GMAIL = env.GetEnv("PERSONAL_GMAIL", "")
	GMAIL_APP_PASSWORD = env.GetEnv("GMAIL_APP_PASSWORD", "")

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
	notificationEventHandler := &ServiceEventHandler{}
	consumerGroup, err := kafka.NewConsumerGroup(KAFKA_BROKERS, KAFKA_TOPIC, CONSUMER_GROUP_ID, notificationEventHandler)
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
	case "com.ecommerce.ShippingConfirmed":
		handleShippingConfirmed(ev)

		// default:
		// log.Printf("  -> Skipping event type '%s' by OrderServiceEventHandler", ev.Type)
	}
	return nil
}

func handleShippingConfirmed(ev event.CloudEvent) {
	// Acknowledge interested event(s)
	log.Printf("(NotificationService) Acknowledged event: com.ecommerce.PaymentConfirmed\n[LOG] Which OrderId: %s\n[LOG] From topic: %s", ev.ID, KAFKA_TOPIC)

	// Send email to user
	log.Println("Email sent")

	// Source: https://mailtrap.io/blog/golang-send-email-gmail/

	// Create a new message
	message := gomail.NewMessage()

	// Set email headers
	message.SetHeader("From", "ecommerce@gmail.com")
	message.SetHeader("To", ev.Data.UserEmail)
	subject := "Order '" + ev.ID + "' has shipped"
	message.SetHeader("Subject", subject)

	// Set email body
	body := `
		<h2>Your Order Details</h2>
		<table style="border-collapse: collapse; width: 100%;">
			<tr>
				<td style="padding: 8px; border: 1px solid #ddd;"><strong>Item</strong></td>
				<td style="padding: 8px; border: 1px solid #ddd;">` + ev.Data.Item + `</td>
			</tr>
			<tr>
				<td style="padding: 8px; border: 1px solid #ddd;"><strong>Price</strong></td>
				<td style="padding: 8px; border: 1px solid #ddd;">$` + strconv.FormatFloat(ev.Data.Amount, 'f', 2, 64) + `</td>
			</tr>
			<tr>
				<td style="padding: 8px; border: 1px solid #ddd;"><strong>Order ID</strong></td>
				<td style="padding: 8px; border: 1px solid #ddd;">` + ev.ID + `</td>
			</tr>
		</table>
	`
	message.SetBody("text/html", body)

	// Set up the SMTP dialer
	dialer := gomail.NewDialer("smtp.gmail.com", 587, PERSONAL_GMAIL, GMAIL_APP_PASSWORD)

	// Send the email
	if err := dialer.DialAndSend(message); err != nil {
		fmt.Println("Error:", err)
		panic(err)
	} else {
		fmt.Println("Email sent successfully!")
	}

}
