package data

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"
	"time"
	v1 "trade-demo/api/trade/v1"
	"trade-demo/internal/common"
	"trade-demo/internal/mapper"
)

type Endpoints struct{ Product, Inventory, Order, Payment, Saga string }
type Remote struct {
	product   v1.ProductServiceClient
	inventory v1.InventoryServiceClient
	order     v1.OrderServiceClient
	payment   v1.PaymentServiceClient
	saga      v1.SagaServiceClient
	conns     []*grpc.ClientConn
}

// NewRemote dials only the dependencies of the selected process. Dialing is lazy,
// so mutually dependent services can start in any order.
func NewRemote(ctx context.Context, role string, e Endpoints) (*Remote, error) {
	r := &Remote{}
	dial := func(ep string) (*grpc.ClientConn, error) {
		c, err := kgrpc.DialInsecure(ctx, kgrpc.WithEndpoint(ep), kgrpc.WithTimeout(10*time.Second), kgrpc.WithMiddleware(tracing.Client()))
		if err == nil {
			r.conns = append(r.conns, c)
		}
		return c, err
	}
	targets := map[string][]string{"order": {"product", "saga"}, "payment": {"saga"}, "saga": {"product", "inventory", "order", "payment"}}
	for _, target := range targets[role] {
		var ep string
		switch target {
		case "product":
			ep = e.Product
		case "inventory":
			ep = e.Inventory
		case "order":
			ep = e.Order
		case "payment":
			ep = e.Payment
		case "saga":
			ep = e.Saga
		}
		c, err := dial(ep)
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("dial %s: %w", target, err)
		}
		switch target {
		case "product":
			r.product = v1.NewProductServiceClient(c)
		case "inventory":
			r.inventory = v1.NewInventoryServiceClient(c)
		case "order":
			r.order = v1.NewOrderServiceClient(c)
		case "payment":
			r.payment = v1.NewPaymentServiceClient(c)
		case "saga":
			r.saga = v1.NewSagaServiceClient(c)
		}
	}
	return r, nil
}
func (r *Remote) Close() {
	for _, c := range r.conns {
		_ = c.Close()
	}
}
func (r *Remote) GetProduct(ctx context.Context, id string) (common.Product, error) {
	v, err := r.product.GetProduct(ctx, &v1.IDRequest{Id: id})
	if err != nil {
		return common.Product{}, err
	}
	return mapper.ProductModel(v), nil
}
func (r *Remote) GetOrder(ctx context.Context, id string) (common.Order, error) {
	v, err := r.order.GetOrder(ctx, &v1.IDRequest{Id: id})
	if err != nil {
		return common.Order{}, err
	}
	return mapper.OrderModel(v), nil
}
func (r *Remote) ChangeOrder(ctx context.Context, id, target string) (common.Order, error) {
	var v *v1.Order
	var err error
	if target == "PAID" {
		v, err = r.order.MarkOrderPaid(ctx, &v1.IDRequest{Id: id})
	} else {
		v, err = r.order.CancelOrder(ctx, &v1.IDRequest{Id: id})
	}
	if err != nil {
		return common.Order{}, err
	}
	return mapper.OrderModel(v), nil
}
func (r *Remote) ReserveStock(ctx context.Context, in common.Reservation) (common.Reservation, error) {
	v, err := r.inventory.ReserveStock(ctx, &v1.ReserveStockRequest{Id: in.ID, OrderId: in.OrderID, ProductId: in.ProductID, Quantity: in.Quantity})
	if err != nil {
		return common.Reservation{}, err
	}
	return mapper.ReservationModel(v), nil
}
func (r *Remote) ChangeReservation(ctx context.Context, id, target string) (common.Reservation, error) {
	var v *v1.Reservation
	var err error
	if target == "CONFIRMED" {
		v, err = r.inventory.ConfirmReservation(ctx, &v1.IDRequest{Id: id})
	} else {
		v, err = r.inventory.ReleaseReservation(ctx, &v1.IDRequest{Id: id})
	}
	if err != nil {
		return common.Reservation{}, err
	}
	return mapper.ReservationModel(v), nil
}
func (r *Remote) CreatePayment(ctx context.Context, orderID string, amount int64) (common.Payment, error) {
	v, err := r.payment.CreatePayment(ctx, &v1.CreatePaymentRequest{OrderId: orderID, AmountFen: amount})
	if err != nil {
		return common.Payment{}, err
	}
	return mapper.PaymentModel(v), nil
}
func (r *Remote) GetPayment(ctx context.Context, id string) (common.Payment, error) {
	v, err := r.payment.GetPayment(ctx, &v1.IDRequest{Id: id})
	if err != nil {
		return common.Payment{}, err
	}
	return mapper.PaymentModel(v), nil
}
func (r *Remote) StartSaga(ctx context.Context, orderID string) (common.Saga, error) {
	v, err := r.saga.StartSaga(ctx, &v1.StartSagaRequest{OrderId: orderID})
	if err != nil {
		return common.Saga{}, err
	}
	return mapper.SagaModel(v), nil
}
func (r *Remote) NotifyPayment(ctx context.Context, p common.Payment) (common.Saga, error) {
	v, err := r.saga.NotifyPaymentResult(ctx, &v1.PaymentResultRequest{PaymentId: p.ID, OrderId: p.OrderID, Result: p.Status})
	if err != nil {
		return common.Saga{}, err
	}
	return mapper.SagaModel(v), nil
}
