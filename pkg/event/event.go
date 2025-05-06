package event

import "time"

// Services indices in vector clock
// const (
// 	OrderServiceIndex     = 0
// 	InventoryServiceIndex = 1
// 	PaymentServiceIndex   = 2
// 	ShippingServiceIndex  = 3
// 	ClockSize             = 4
// )

// Event Types
const (
	OrderPlacedType       = "com.ecommerce.OrderPlaced"
	OrderCancelledType    = "com.ecommerce.OrderCancelled"
	InventoryUpdatedType  = "com.ecommerce.InventoryUpdated"
	PaymentConfirmedType  = "com.ecommerce.PaymentConfirmed"
	ShippingConfirmedType = "com.ecommerce.ShippingConfirmed"
)

type CloudEvent struct {
	ID              string    `json:"id"`
	Source          string    `json:"source"`
	Type            string    `json:"type"`
	DataContentType string    `json:"datacontenttype"`
	Time            time.Time `json:"time"`
	Data            OrderData `json:"data"`
}

type OrderData struct {
	UserId    string  `json:"userId"`
	UserEmail string  `json:"userEmail"`
	Item      string  `json:"item"`
	Amount    float64 `json:"amount"`
}

// type PaymentMethod struct {
// 	Network string `json:"network"`
// 	CardNo  string `json:"cardNo"`
// 	Exp     string `json:"exp"`
// 	CVV     string `json:"cvv"`
// }
