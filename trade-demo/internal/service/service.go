package service

import (
	"context"
	v1 "trade-demo/api/trade/v1"
	"trade-demo/internal/biz"
	"trade-demo/internal/common"
	"trade-demo/internal/mapper"
)

// A process registers only its own service interface; these adapters contain no business rules.
type Service struct {
	v1.UnimplementedProductServiceServer
	v1.UnimplementedInventoryServiceServer
	v1.UnimplementedOrderServiceServer
	v1.UnimplementedPaymentServiceServer
	v1.UnimplementedSagaServiceServer
	uc *biz.Usecase
}

func New(uc *biz.Usecase) *Service { return &Service{uc: uc} }
func (s *Service) ListProducts(ctx context.Context, _ *v1.Empty) (*v1.ProductList, error) {
	out := &v1.ProductList{}
	for _, p := range s.uc.ListProducts() {
		out.Items = append(out.Items, mapper.Product(p))
	}
	return out, nil
}
func (s *Service) GetProduct(ctx context.Context, in *v1.IDRequest) (*v1.Product, error) {
	v, e := s.uc.GetProduct(in.Id)
	if e != nil {
		return nil, e
	}
	return mapper.Product(v), nil
}
func (s *Service) GetStock(ctx context.Context, _ *v1.Empty) (*v1.StockReply, error) {
	return &v1.StockReply{Stock: s.uc.GetStock()}, nil
}
func (s *Service) ReserveStock(ctx context.Context, in *v1.ReserveStockRequest) (*v1.Reservation, error) {
	v, e := s.uc.ReserveStock(common.Reservation{ID: in.Id, OrderID: in.OrderId, ProductID: in.ProductId, Quantity: in.Quantity})
	if e != nil {
		return nil, e
	}
	return mapper.Reservation(v), nil
}
func (s *Service) ConfirmReservation(ctx context.Context, in *v1.IDRequest) (*v1.Reservation, error) {
	v, e := s.uc.ChangeReservation(in.Id, "CONFIRMED")
	if e != nil {
		return nil, e
	}
	return mapper.Reservation(v), nil
}
func (s *Service) ReleaseReservation(ctx context.Context, in *v1.IDRequest) (*v1.Reservation, error) {
	v, e := s.uc.ChangeReservation(in.Id, "RELEASED")
	if e != nil {
		return nil, e
	}
	return mapper.Reservation(v), nil
}
func (s *Service) CreateOrder(ctx context.Context, in *v1.CreateOrderRequest) (*v1.CreateOrderReply, error) {
	o, v, e := s.uc.CreateOrder(ctx, in.UserId, in.ProductId, in.Quantity)
	if e != nil {
		return nil, e
	}
	return &v1.CreateOrderReply{Order: mapper.Order(o), Saga: mapper.Saga(v)}, nil
}
func (s *Service) GetOrder(ctx context.Context, in *v1.IDRequest) (*v1.Order, error) {
	v, e := s.uc.GetOrder(in.Id)
	if e != nil {
		return nil, e
	}
	return mapper.Order(v), nil
}
func (s *Service) MarkOrderPaid(ctx context.Context, in *v1.IDRequest) (*v1.Order, error) {
	v, e := s.uc.ChangeOrder(in.Id, "PAID")
	if e != nil {
		return nil, e
	}
	return mapper.Order(v), nil
}
func (s *Service) CancelOrder(ctx context.Context, in *v1.IDRequest) (*v1.Order, error) {
	v, e := s.uc.ChangeOrder(in.Id, "CANCELLED")
	if e != nil {
		return nil, e
	}
	return mapper.Order(v), nil
}
func (s *Service) CreatePayment(ctx context.Context, in *v1.CreatePaymentRequest) (*v1.Payment, error) {
	v, e := s.uc.CreatePayment(in.OrderId, in.AmountFen)
	if e != nil {
		return nil, e
	}
	return mapper.Payment(v), nil
}
func (s *Service) ClosePayment(ctx context.Context, in *v1.IDRequest) (*v1.Payment, error) {
	v, e := s.uc.ChangePayment(in.Id, "CLOSED")
	if e != nil {
		return nil, e
	}
	return mapper.Payment(v), nil
}
func (s *Service) GetPayment(ctx context.Context, in *v1.IDRequest) (*v1.Payment, error) {
	v, e := s.uc.GetPayment(in.Id)
	if e != nil {
		return nil, e
	}
	return mapper.Payment(v), nil
}
func (s *Service) SimulatePayment(ctx context.Context, in *v1.SimulatePaymentRequest) (*v1.Payment, error) {
	v, e := s.uc.SimulatePayment(ctx, in.Id, in.Result)
	if e != nil {
		return nil, e
	}
	return mapper.Payment(v), nil
}
func (s *Service) StartSaga(ctx context.Context, in *v1.StartSagaRequest) (*v1.Saga, error) {
	v, e := s.uc.StartSaga(ctx, in.OrderId)
	if e != nil {
		return nil, e
	}
	return mapper.Saga(v), nil
}
func (s *Service) NotifyPaymentResult(ctx context.Context, in *v1.PaymentResultRequest) (*v1.Saga, error) {
	v, e := s.uc.NotifyPayment(ctx, in.PaymentId, in.OrderId, in.Result)
	if e != nil {
		return nil, e
	}
	return mapper.Saga(v), nil
}
func (s *Service) GetSaga(ctx context.Context, in *v1.IDRequest) (*v1.Saga, error) {
	v, e := s.uc.GetSaga(in.Id)
	if e != nil {
		return nil, e
	}
	return mapper.Saga(v), nil
}
