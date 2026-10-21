package biz

import (
	"context"
	"github.com/go-kratos/kratos/v2/errors"
	"math"
	"time"
	"trade-demo/internal/common"
)

func (u *Usecase) CreateOrder(ctx context.Context, userID, productID string, quantity int64) (common.Order, common.Saga, error) {
	var o common.Order
	var saga common.Saga
	if userID == "" || productID == "" || quantity < 1 || quantity > 99 {
		return o, saga, invalid("userId、productId 和 1..99 的 quantity 为必填")
	}
	p, err := u.remote.GetProduct(ctx, productID)
	if err != nil {
		return o, saga, err
	}
	if !p.Active || p.PriceFen <= 0 || p.PriceFen > math.MaxInt64/quantity {
		return o, saga, conflict("商品不可售或金额无效")
	}
	id := common.ID("ord")
	o = common.Order{ID: id, UserID: userID, Items: []common.OrderItem{{ProductID: p.ID, ProductName: p.Name, UnitPrice: p.PriceFen, Quantity: quantity}}, AmountFen: p.PriceFen * quantity, Status: "PENDING_PAYMENT", CreatedAt: time.Now().UTC()}
	if err = u.repo.Update(func(s *common.State) error { s.Orders[id] = o; return nil }); err != nil {
		return o, saga, storage(err)
	}
	saga, err = u.remote.StartSaga(ctx, id)
	if err != nil {
		return o, saga, errors.ServiceUnavailable("SAGA_START_FAILED", "订单已保存，Saga 启动失败；orderId="+id+": "+err.Error())
	}
	err = u.repo.Update(func(s *common.State) error {
		v := s.Orders[id]
		v.SagaID = saga.ID
		v.PaymentID = saga.PaymentID
		s.Orders[id] = v
		o = v
		return nil
	})
	return o, saga, storage(err)
}
func (u *Usecase) GetOrder(id string) (common.Order, error) {
	var o common.Order
	ok := false
	u.repo.Read(func(s common.State) { o, ok = s.Orders[id]; o.Items = append([]common.OrderItem(nil), o.Items...) })
	if !ok {
		return o, missing("订单不存在")
	}
	return o, nil
}
func (u *Usecase) ChangeOrder(id, target string) (common.Order, error) {
	var out common.Order
	err := u.repo.Update(func(s *common.State) error {
		o, ok := s.Orders[id]
		if !ok {
			return missing("订单不存在")
		}
		if o.Status == target {
			out = o
			return nil
		}
		if o.Status != "PENDING_PAYMENT" {
			return conflict("invalid order transition " + o.Status + " to " + target)
		}
		o.Status = target
		s.Orders[id] = o
		out = o
		return nil
	})
	return out, storage(err)
}
