package redisclient

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/dinh2644/distributed_event_notification/pkg/env"
	"github.com/redis/go-redis/v9"
)

const (
	StateOrderPlaced       = "ORDER_PLACED"
	StatePaymentConfirmed  = "PAYMENT_CONFIRMED"
	StateInventoryUpdated  = "INVENTORY_UPDATED"
	StateShippingConfirmed = "SHIPPING_CONFIRMED"
	StateNotificationSent  = "NOTIFICATION_SENT"
	StateOrderCancelled    = "ORDER_CANCELLED"
	StatePaymentFailed     = "PAYMENT_FAILED"
)

var (
	// is returned when a state for an order ID is not found in Redis.
	ErrStateNotFound = errors.New("order state not found")
)

// Client wraps the go-redis client
type Client struct {
	rdb *redis.Client
}

// New initializes and returns a new Redis client connection (it reads configuration from environment variables)
func New(ctx context.Context) (*Client, error) {
	redisAddr := env.GetEnv("REDIS_ADDR", "redisdb:6379")
	redisPassword := env.GetEnv("REDIS_PASSWORD", "ahundredmenvsonegorilla123")

	log.Printf("Connecting to Redis at %s", redisAddr)

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})

	// Ping Redis to check conn
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
		return nil, err
	}

	log.Println("Successfully connected to Redis")
	return &Client{rdb: rdb}, nil
}

// GetOrderState retrieves the current state for a given orderID (returns the state string or ErrStateNotFound if the key doesn't exist)
func (c *Client) GetOrderState(ctx context.Context, orderID string) (string, error) {
	key := "saga:state:" + orderID // future note: adding prefix is good practice
	status, err := c.rdb.Get(ctx, key).Result()

	if err == redis.Nil {
		// Key does not exist (represents state not yet set)
		return "", ErrStateNotFound
	} else if err != nil {
		log.Printf("Redis GET error for key %s: %v", key, err)
		return "", err
	}
	return status, nil
}

// SetOrderState sets the state for a given orderID
func (c *Client) SetOrderState(ctx context.Context, orderID string, status string, expiration time.Duration) error {
	key := "saga:state:" + orderID
	err := c.rdb.Set(ctx, key, status, expiration).Err()
	if err != nil {
		log.Printf("Redis SET error for key %s: %v", key, err)
	}
	return err
}

// Helper function(s) for data extraction
// func ExtractOrderData(dataMap map[string]interface{}) (userID, item string, amount float64) {
// 	var ok bool
// 	userIDVal, ok1 := dataMap["userId"]
// 	itemVal, ok2 := dataMap["item"]
// 	amountVal, ok3 := dataMap["amount"]

// 	if !ok1 || !ok2 || !ok3 {
// 		return "", "", 0 // failure
// 	}

// 	userID, ok = userIDVal.(string)
// 	if !ok {
// 		return "", "", 0
// 	}
// 	item, ok = itemVal.(string)
// 	if !ok {
// 		return "", "", 0
// 	}
// 	amount, ok = amountVal.(float64)
// 	if !ok {
// 		return "", "", 0
// 	}
// 	return userID, item, amount
// }

func (c *Client) Close() error {
	if c.rdb != nil {
		return c.rdb.Close()
	}
	return nil
}
