package data

import "trade-demo/internal/common"

func OpenRepository(path, role string, seed bool) (*common.Store[common.State], error) {
	initial := common.State{}
	switch role {
	case "product":
		initial.Products = map[string]common.Product{}
		if seed {
			initial.Products["tea"] = common.Product{ID: "tea", Name: "清香绿茶", PriceFen: 1290, Active: true}
			initial.Products["mug"] = common.Product{ID: "mug", Name: "陶瓷马克杯", PriceFen: 3990, Active: true}
		}
	case "inventory":
		initial.Stock = map[string]int64{}
		initial.Reservations = map[string]common.Reservation{}
		if seed {
			initial.Stock["tea"] = 20
			initial.Stock["mug"] = 8
		}
	case "order":
		initial.Orders = map[string]common.Order{}
	case "payment":
		initial.Payments = map[string]common.Payment{}
		initial.ByOrder = map[string]string{}
	case "saga":
		initial.Sagas = map[string]common.Saga{}
		initial.ByOrder = map[string]string{}
		initial.ReservationByOrder = map[string]string{}
	}
	repo, err := common.OpenStore(path, initial)
	if err != nil {
		return nil, err
	}
	// Persist seeds too; reopen existing JSON without resetting inventory or history.
	if err = repo.Update(func(*common.State) error { return nil }); err != nil {
		return nil, err
	}
	return repo, nil
}
