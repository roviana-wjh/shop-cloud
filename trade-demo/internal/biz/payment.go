package biz

import (
	"context"
	"github.com/go-kratos/kratos/v2/errors"
	"time"
	"trade-demo/internal/common"
)

func (u *Usecase) CreatePayment(orderID string, amount int64) (common.Payment, error) {
	var out common.Payment
	if orderID == "" || amount < 1 {
		return out, invalid("invalid payment request")
	}
	err := u.repo.Update(func(s *common.State) error {
		if id, ok := s.ByOrder[orderID]; ok {
			out = s.Payments[id]
			if out.AmountFen != amount {
				return conflict("payment amount conflict")
			}
			return nil
		}
		out = common.Payment{ID: common.ID("pay"), OrderID: orderID, AmountFen: amount, Status: "CREATED", CreatedAt: time.Now().UTC()}
		s.Payments[out.ID] = out
		s.ByOrder[orderID] = out.ID
		return nil
	})
	return out, storage(err)
}
func (u *Usecase) GetPayment(id string) (common.Payment, error) {
	var out common.Payment
	ok := false
	u.repo.Read(func(s common.State) { out, ok = s.Payments[id] })
	if !ok {
		return out, missing("支付单不存在")
	}
	return out, nil
}
func (u *Usecase) ChangePayment(id, target string) (common.Payment, error) {
	var out common.Payment
	err := u.repo.Update(func(s *common.State) error {
		p, ok := s.Payments[id]
		if !ok {
			return missing("支付单不存在")
		}
		if p.Status == target {
			out = p
			return nil
		}
		if p.Status != "CREATED" {
			return conflict("payment already " + p.Status)
		}
		p.Status = target
		s.Payments[id] = p
		out = p
		return nil
	})
	return out, storage(err)
}
func (u *Usecase) SimulatePayment(ctx context.Context, id, result string) (common.Payment, error) {
	if result != "success" && result != "fail" {
		return common.Payment{}, invalid("result 必须是 success 或 fail")
	}
	target := "FAILED"
	if result == "success" {
		target = "SUCCEEDED"
	}
	p, err := u.ChangePayment(id, target)
	if err != nil {
		return p, err
	}
	if _, err = u.remote.NotifyPayment(ctx, p); err != nil {
		return p, errors.ServiceUnavailable("PAYMENT_NOTIFICATION_FAILED", "支付结果已保存；重试相同结果继续编排: "+err.Error())
	}
	return p, nil
}
