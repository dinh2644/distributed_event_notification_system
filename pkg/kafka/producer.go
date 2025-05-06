package kafka

import (
	"log"

	"github.com/IBM/sarama"
)

// Manages the Sarama SyncProducer connection
type KafkaProducer struct {
	producer sarama.SyncProducer
}

// NewProducer creates and initializes a new Kafka SyncProducer (it connects to the specified brokers)
func NewProducer(brokers []string) (*KafkaProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Idempotent = true
	config.Net.MaxOpenRequests = 1

	// Use sarama.NewSyncProducer for synchronous production (waits for ack)
	syncProducer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Printf("Failed to create Sarama sync producer: %v", err)
		return nil, err
	}

	log.Println("Kafka sync producer created successfully")
	kp := &KafkaProducer{
		producer: syncProducer,
	}
	return kp, nil
}

// Sends a message to the specified topic (along w/ given key)
func (kp *KafkaProducer) Publish(topic string, key string, message []byte) (partition int32, offset int64, err error) {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.StringEncoder(message),
	}

	partition, offset, err = kp.producer.SendMessage(msg)
	if err != nil {
		log.Printf("Failed to send message to topic %s: %v", topic, err)
	}
	return partition, offset, err
}

func (kp *KafkaProducer) Close() error {
	log.Println("Closing Kafka producer...")
	if kp.producer != nil {
		return kp.producer.Close()
	}
	return nil
}
