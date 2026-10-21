package common

import "time"

type Product struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	PriceFen int64  `json:"priceFen"`
	Active   bool   `json:"active"`
}
type OrderItem struct {
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	UnitPrice   int64  `json:"unitPriceFen"`
	Quantity    int64  `json:"quantity"`
}
type Order struct {
	ID        string      `json:"id"`
	UserID    string      `json:"userId"`
	Items     []OrderItem `json:"items"`
	AmountFen int64       `json:"amountFen"`
	Status    string      `json:"status"`
	PaymentID string      `json:"paymentId,omitempty"`
	SagaID    string      `json:"sagaId,omitempty"`
	CreatedAt time.Time   `json:"createdAt"`
}
type Payment struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"orderId"`
	AmountFen int64     `json:"amountFen"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
type Reservation struct {
	ID        string `json:"id"`
	OrderID   string `json:"orderId"`
	ProductID string `json:"productId"`
	Quantity  int64  `json:"quantity"`
	Status    string `json:"status"`
}
type SagaStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}
type Saga struct {
	ID          string     `json:"id"`
	OrderID     string     `json:"orderId"`
	PaymentID   string     `json:"paymentId,omitempty"`
	Status      string     `json:"status"`
	CurrentStep string     `json:"currentStep,omitempty"`
	Steps       []SagaStep `json:"steps"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}
