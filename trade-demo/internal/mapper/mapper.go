// Package mapper translates transport messages and domain models explicitly.
package mapper

import (
	"time"
	v1 "trade-demo/api/trade/v1"
	"trade-demo/internal/common"
)

func Product(v common.Product) *v1.Product {
	return &v1.Product{Id: v.ID, Name: v.Name, PriceFen: v.PriceFen, Active: v.Active}
}
func ProductModel(v *v1.Product) common.Product {
	return common.Product{ID: v.Id, Name: v.Name, PriceFen: v.PriceFen, Active: v.Active}
}
func Order(v common.Order) *v1.Order {
	items := make([]*v1.OrderItem, 0, len(v.Items))
	for _, i := range v.Items {
		items = append(items, &v1.OrderItem{ProductId: i.ProductID, ProductName: i.ProductName, UnitPriceFen: i.UnitPrice, Quantity: i.Quantity})
	}
	return &v1.Order{Id: v.ID, UserId: v.UserID, Items: items, AmountFen: v.AmountFen, Status: v.Status, PaymentId: v.PaymentID, SagaId: v.SagaID, CreatedAt: v.CreatedAt.Format(time.RFC3339Nano)}
}
func OrderModel(v *v1.Order) common.Order {
	t, _ := time.Parse(time.RFC3339Nano, v.CreatedAt)
	out := common.Order{ID: v.Id, UserID: v.UserId, AmountFen: v.AmountFen, Status: v.Status, PaymentID: v.PaymentId, SagaID: v.SagaId, CreatedAt: t}
	for _, i := range v.Items {
		out.Items = append(out.Items, common.OrderItem{ProductID: i.ProductId, ProductName: i.ProductName, UnitPrice: i.UnitPriceFen, Quantity: i.Quantity})
	}
	return out
}
func Payment(v common.Payment) *v1.Payment {
	return &v1.Payment{Id: v.ID, OrderId: v.OrderID, AmountFen: v.AmountFen, Status: v.Status, CreatedAt: v.CreatedAt.Format(time.RFC3339Nano)}
}
func PaymentModel(v *v1.Payment) common.Payment {
	t, _ := time.Parse(time.RFC3339Nano, v.CreatedAt)
	return common.Payment{ID: v.Id, OrderID: v.OrderId, AmountFen: v.AmountFen, Status: v.Status, CreatedAt: t}
}
func Reservation(v common.Reservation) *v1.Reservation {
	return &v1.Reservation{Id: v.ID, OrderId: v.OrderID, ProductId: v.ProductID, Quantity: v.Quantity, Status: v.Status}
}
func ReservationModel(v *v1.Reservation) common.Reservation {
	return common.Reservation{ID: v.Id, OrderID: v.OrderId, ProductID: v.ProductId, Quantity: v.Quantity, Status: v.Status}
}
func Saga(v common.Saga) *v1.Saga {
	out := &v1.Saga{Id: v.ID, OrderId: v.OrderID, PaymentId: v.PaymentID, Status: v.Status, CurrentStep: v.CurrentStep, UpdatedAt: v.UpdatedAt.Format(time.RFC3339Nano)}
	for _, s := range v.Steps {
		out.Steps = append(out.Steps, &v1.Step{Name: s.Name, Status: s.Status, Detail: s.Detail})
	}
	return out
}
func SagaModel(v *v1.Saga) common.Saga {
	t, _ := time.Parse(time.RFC3339Nano, v.UpdatedAt)
	out := common.Saga{ID: v.Id, OrderID: v.OrderId, PaymentID: v.PaymentId, Status: v.Status, CurrentStep: v.CurrentStep, UpdatedAt: t}
	for _, s := range v.Steps {
		out.Steps = append(out.Steps, common.SagaStep{Name: s.Name, Status: s.Status, Detail: s.Detail})
	}
	return out
}
