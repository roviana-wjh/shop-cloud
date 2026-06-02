package server

import (
	"strconv"

	"review-service/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

func registerPublicReviewRoutes(srv *khttp.Server, reviewer *service.ReviewService, logger log.Logger) {
	lg := log.NewHelper(logger)
	r := srv.Route("/")
	r.GET("/v1/review/public/store/{storeID}", func(ctx khttp.Context) error {
		storeID, ok := pathID(ctx, "storeID")
		if !ok {
			return ctx.Result(400, map[string]string{"message": "storeID must be greater than 0"})
		}
		page, pageSize := pagination(ctx)
		out, err := reviewer.ListPublicReviewsByStoreId(ctx, storeID, page, pageSize)
		if err != nil {
			lg.WithContext(ctx).Errorf("list public store reviews failed: %v", err)
			return ctx.Result(500, map[string]string{"message": err.Error()})
		}
		return ctx.Result(200, out)
	})
	r.GET("/v1/review/public/goods/{spuID}", func(ctx khttp.Context) error {
		spuID, ok := pathID(ctx, "spuID")
		if !ok {
			return ctx.Result(400, map[string]string{"message": "spuID must be greater than 0"})
		}
		page, pageSize := pagination(ctx)
		out, err := reviewer.ListPublicReviewsBySpuId(ctx, spuID, page, pageSize)
		if err != nil {
			lg.WithContext(ctx).Errorf("list public goods reviews failed: %v", err)
			return ctx.Result(500, map[string]string{"message": err.Error()})
		}
		return ctx.Result(200, out)
	})
}

func pathID(ctx khttp.Context, name string) (int64, bool) {
	var vars struct {
		StoreID int64 `json:"storeID"`
		SpuID   int64 `json:"spuID"`
	}
	if err := ctx.BindVars(&vars); err != nil {
		return 0, false
	}
	if name == "spuID" {
		return vars.SpuID, vars.SpuID > 0
	}
	return vars.StoreID, vars.StoreID > 0
}

func pagination(ctx khttp.Context) (int32, int32) {
	query := ctx.Request().URL.Query()
	page := int32(1)
	pageSize := int32(10)
	if s := query.Get("page"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 32); err == nil && v > 0 {
			page = int32(v)
		}
	}
	if s := query.Get("pageSize"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 32); err == nil && v > 0 && v <= 100 {
			pageSize = int32(v)
		}
	}
	return page, pageSize
}
