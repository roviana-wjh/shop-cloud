package biz

import "trade-demo/internal/common"

func (u *Usecase) ListProducts() []common.Product {
	list := []common.Product{}
	u.repo.Read(func(s common.State) {
		for _, p := range s.Products {
			list = append(list, p)
		}
	})
	return list
}
func (u *Usecase) GetProduct(id string) (common.Product, error) {
	var p common.Product
	ok := false
	u.repo.Read(func(s common.State) { p, ok = s.Products[id] })
	if !ok {
		return p, missing("商品不存在")
	}
	return p, nil
}
