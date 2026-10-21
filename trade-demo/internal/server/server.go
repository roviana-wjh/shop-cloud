package server

import (
	"fmt"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"time"
	v1 "trade-demo/api/trade/v1"
	"trade-demo/internal/service"
)

func New(role, grpcAddr, httpAddr string, s *service.Service) (*kgrpc.Server, *khttp.Server, error) {
	gs := kgrpc.NewServer(kgrpc.Address(grpcAddr), kgrpc.Timeout(30*time.Second), kgrpc.Middleware(recovery.Recovery(), tracing.Server()))
	hs := khttp.NewServer(khttp.Address(httpAddr), khttp.Timeout(30*time.Second), khttp.Middleware(recovery.Recovery(), tracing.Server()))
	switch role {
	case "product":
		v1.RegisterProductServiceServer(gs, s)
		v1.RegisterProductServiceHTTPServer(hs, s)
	case "inventory":
		v1.RegisterInventoryServiceServer(gs, s)
		v1.RegisterInventoryServiceHTTPServer(hs, s)
	case "order":
		v1.RegisterOrderServiceServer(gs, s)
		v1.RegisterOrderServiceHTTPServer(hs, s)
	case "payment":
		v1.RegisterPaymentServiceServer(gs, s)
		v1.RegisterPaymentServiceHTTPServer(hs, s)
	case "saga":
		v1.RegisterSagaServiceServer(gs, s)
		v1.RegisterSagaServiceHTTPServer(hs, s)
	default:
		return nil, nil, fmt.Errorf("unknown service %s", role)
	}
	hs.Route("/").GET("/healthz", func(ctx khttp.Context) error {
		return ctx.Result(200, map[string]string{"status": "ok", "service": role})
	})
	return gs, hs, nil
}
