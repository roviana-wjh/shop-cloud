package biz

import "trade-demo/internal/common"

func (u *Usecase) GetStock() map[string]int64 {
	out := map[string]int64{}
	u.repo.Read(func(s common.State) {
		for k, v := range s.Stock {
			out[k] = v
		}
	})
	return out
}
func (u *Usecase) ReserveStock(in common.Reservation) (common.Reservation, error) {
	var result common.Reservation
	if in.ID == "" || in.OrderID == "" || in.ProductID == "" || in.Quantity < 1 {
		return result, invalid("invalid reservation")
	}
	err := u.repo.Update(func(s *common.State) error {
		if old, ok := s.Reservations[in.ID]; ok {
			if old.OrderID != in.OrderID || old.ProductID != in.ProductID || old.Quantity != in.Quantity {
				return conflict("reservation idempotency key conflict")
			}
			if old.Status != "RESERVED" && old.Status != "CONFIRMED" {
				return conflict("reservation already released")
			}
			result = old
			return nil
		}
		if s.Stock[in.ProductID] < in.Quantity {
			return conflict("insufficient stock")
		}
		s.Stock[in.ProductID] -= in.Quantity
		in.Status = "RESERVED"
		s.Reservations[in.ID] = in
		result = in
		return nil
	})
	return result, storage(err)
}
func (u *Usecase) ChangeReservation(id, target string) (common.Reservation, error) {
	var result common.Reservation
	err := u.repo.Update(func(s *common.State) error {
		v, ok := s.Reservations[id]
		if !ok {
			return missing("reservation not found")
		}
		if v.Status == target {
			result = v
			return nil
		}
		if v.Status != "RESERVED" {
			return conflict("reservation already " + v.Status)
		}
		if target == "RELEASED" {
			s.Stock[v.ProductID] += v.Quantity
		}
		v.Status = target
		s.Reservations[id] = v
		result = v
		return nil
	})
	return result, storage(err)
}
