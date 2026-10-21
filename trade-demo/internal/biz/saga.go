package biz

import (
	"context"
	"github.com/go-kratos/kratos/v2/errors"
	"time"
	"trade-demo/internal/common"
)

func (u *Usecase) GetSaga(id string) (common.Saga, error) {
	var out common.Saga
	ok := false
	u.repo.Read(func(s common.State) { out, ok = s.Sagas[id]; out.Steps = append([]common.SagaStep(nil), out.Steps...) })
	if !ok {
		return out, missing("Saga 不存在")
	}
	return out, nil
}
func (u *Usecase) persist(v *common.Saga) error {
	v.UpdatedAt = time.Now().UTC()
	return storage(u.repo.Update(func(s *common.State) error {
		copy := *v
		copy.Steps = append([]common.SagaStep(nil), v.Steps...)
		s.Sagas[v.ID] = copy
		return nil
	}))
}
func step(v *common.Saga, name, status, detail string) {
	v.CurrentStep = name
	for i := range v.Steps {
		if v.Steps[i].Name == name {
			v.Steps[i].Status = status
			v.Steps[i].Detail = detail
			return
		}
	}
	v.Steps = append(v.Steps, common.SagaStep{Name: name, Status: status, Detail: detail})
}

func (u *Usecase) StartSaga(ctx context.Context, orderID string) (common.Saga, error) {
	u.sagaMu.Lock()
	defer u.sagaMu.Unlock()
	if orderID == "" {
		return common.Saga{}, invalid("orderId required")
	}
	var existing string
	u.repo.Read(func(s common.State) { existing = s.ByOrder[orderID] })
	if existing != "" {
		return u.GetSaga(existing)
	}
	o, err := u.remote.GetOrder(ctx, orderID)
	if err != nil {
		return common.Saga{}, err
	}
	if o.Status != "PENDING_PAYMENT" || len(o.Items) != 1 {
		return common.Saga{}, conflict("only a pending single-product order can start a Saga")
	}
	v := common.Saga{ID: common.ID("saga"), OrderID: orderID, Status: "STARTED", Steps: []common.SagaStep{}}
	if err = u.repo.Update(func(s *common.State) error { s.ByOrder[orderID] = v.ID; s.Sagas[v.ID] = v; return nil }); err != nil {
		return v, storage(err)
	}
	item := o.Items[0]
	p, err := u.remote.GetProduct(ctx, item.ProductID)
	if err != nil || !p.Active || p.PriceFen != item.UnitPrice {
		detail := "商品不可用或价格变化"
		if err != nil {
			detail = err.Error()
		}
		step(&v, "ValidateProduct", "FAILED", detail)
		err = u.compensate(ctx, &v, "")
		return v, err
	}
	step(&v, "ValidateProduct", "COMPLETED", "商品在售且价格与快照一致")
	if err = u.persist(&v); err != nil {
		return v, err
	}
	reservationID := "res-" + orderID
	// Persist intent before invoking a remote side effect. Idempotency key is stable across retries.
	if err = u.repo.Update(func(s *common.State) error { s.ReservationByOrder[orderID] = reservationID; return nil }); err != nil {
		return v, storage(err)
	}
	step(&v, "ReserveStock", "RUNNING", "")
	if err = u.persist(&v); err != nil {
		return v, err
	}
	_, err = u.remote.ReserveStock(ctx, common.Reservation{ID: reservationID, OrderID: orderID, ProductID: item.ProductID, Quantity: item.Quantity})
	if err != nil {
		step(&v, "ReserveStock", "FAILED", err.Error())
		if errors.Code(err) == 409 {
			err = u.compensate(ctx, &v, "")
			return v, err
		}
		v.Status = "RECOVERY_REQUIRED"
		if saveErr := u.persist(&v); saveErr != nil {
			return v, saveErr
		}
		return v, err
	}
	step(&v, "ReserveStock", "COMPLETED", "库存已预留")
	if err = u.persist(&v); err != nil {
		return v, err
	}
	step(&v, "CreatePayment", "RUNNING", "")
	if err = u.persist(&v); err != nil {
		return v, err
	}
	payment, err := u.remote.CreatePayment(ctx, orderID, o.AmountFen)
	if err != nil {
		step(&v, "CreatePayment", "RETRY_REQUIRED", err.Error())
		v.Status = "RECOVERY_REQUIRED"
		if saveErr := u.persist(&v); saveErr != nil {
			return v, saveErr
		}
		return v, err
	}
	v.PaymentID = payment.ID
	step(&v, "CreatePayment", "COMPLETED", "模拟支付单已创建")
	v.Status = "WAITING_FOR_PAYMENT"
	step(&v, "AwaitPayment", "WAITING", "等待模拟支付结果")
	return v, u.persist(&v)
}

func (u *Usecase) NotifyPayment(ctx context.Context, paymentID, orderID, result string) (common.Saga, error) {
	u.sagaMu.Lock()
	defer u.sagaMu.Unlock()
	if paymentID == "" || orderID == "" || (result != "SUCCEEDED" && result != "FAILED") {
		return common.Saga{}, invalid("invalid payment result")
	}
	var id, reservationID string
	u.repo.Read(func(s common.State) { id = s.ByOrder[orderID]; reservationID = s.ReservationByOrder[orderID] })
	v, err := u.GetSaga(id)
	if err != nil {
		return v, err
	}
	if v.PaymentID != paymentID {
		return v, conflict("payment does not belong to saga")
	}
	// The payment service is the source of truth; do not trust a caller-supplied result.
	p, err := u.remote.GetPayment(ctx, paymentID)
	if err != nil {
		return v, err
	}
	if p.OrderID != orderID || p.Status != result {
		return v, conflict("payment result does not match payment service")
	}
	if v.Status == "COMPLETED" || v.Status == "FAILED" {
		return v, nil
	}
	if result == "FAILED" {
		step(&v, "AwaitPayment", "FAILED", "支付失败")
		err = u.compensate(ctx, &v, reservationID)
		return v, err
	}
	step(&v, "AwaitPayment", "COMPLETED", "收到支付成功结果")
	v.Status = "CONFIRMING"
	if err = u.persist(&v); err != nil {
		return v, err
	}
	step(&v, "ConfirmReservation", "RUNNING", "")
	if err = u.persist(&v); err != nil {
		return v, err
	}
	if _, err = u.remote.ChangeReservation(ctx, reservationID, "CONFIRMED"); err != nil {
		step(&v, "ConfirmReservation", "RETRY_REQUIRED", err.Error())
		if saveErr := u.persist(&v); saveErr != nil {
			return v, saveErr
		}
		return v, err
	}
	step(&v, "ConfirmReservation", "COMPLETED", "预留库存已确认")
	if err = u.persist(&v); err != nil {
		return v, err
	}
	step(&v, "MarkOrderPaid", "RUNNING", "")
	if err = u.persist(&v); err != nil {
		return v, err
	}
	if _, err = u.remote.ChangeOrder(ctx, orderID, "PAID"); err != nil {
		step(&v, "MarkOrderPaid", "RETRY_REQUIRED", err.Error())
		if saveErr := u.persist(&v); saveErr != nil {
			return v, saveErr
		}
		return v, err
	}
	step(&v, "MarkOrderPaid", "COMPLETED", "订单已支付")
	v.Status = "COMPLETED"
	v.CurrentStep = ""
	return v, u.persist(&v)
}

func (u *Usecase) compensate(ctx context.Context, v *common.Saga, reservationID string) error {
	v.Status = "COMPENSATING"
	if err := u.persist(v); err != nil {
		return err
	}
	if reservationID != "" {
		step(v, "ReleaseStock", "RUNNING", "")
		if err := u.persist(v); err != nil {
			return err
		}
		if _, err := u.remote.ChangeReservation(ctx, reservationID, "RELEASED"); err != nil {
			step(v, "ReleaseStock", "RETRY_REQUIRED", err.Error())
			v.Status = "COMPENSATION_REQUIRED"
			if saveErr := u.persist(v); saveErr != nil {
				return saveErr
			}
			return err
		}
		step(v, "ReleaseStock", "COMPLETED", "释放库存预留")
		if err := u.persist(v); err != nil {
			return err
		}
	}
	step(v, "CancelOrder", "RUNNING", "")
	if err := u.persist(v); err != nil {
		return err
	}
	if _, err := u.remote.ChangeOrder(ctx, v.OrderID, "CANCELLED"); err != nil {
		step(v, "CancelOrder", "RETRY_REQUIRED", err.Error())
		v.Status = "COMPENSATION_REQUIRED"
		if saveErr := u.persist(v); saveErr != nil {
			return saveErr
		}
		return err
	}
	step(v, "CancelOrder", "COMPLETED", "订单已取消")
	v.Status = "FAILED"
	v.CurrentStep = ""
	return u.persist(v)
}
