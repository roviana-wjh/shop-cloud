package main

import (
	"context"
	"flag"
	"github.com/go-kratos/kratos/v2"
	"log"
	"path/filepath"
	"strings"
	"trade-demo/internal/biz"
	"trade-demo/internal/common"
	"trade-demo/internal/data"
	"trade-demo/internal/server"
	"trade-demo/internal/service"
)

func main() {
	role := flag.String("service", "", "product, inventory, order, payment, or saga")
	flag.Parse()
	ports := map[string][2]string{"product": {":8031", ":9031"}, "inventory": {":8032", ":9032"}, "order": {":8033", ":9033"}, "payment": {":8034", ":9034"}, "saga": {":8035", ":9035"}}
	pair, ok := ports[*role]
	if !ok {
		log.Fatal("select -service=product|inventory|order|payment|saga")
	}
	repo, err := data.OpenRepository(common.Env("TRADE_DATA_FILE", filepath.Join("data", *role+".json")), *role, strings.EqualFold(common.Env("TRADE_SEED", "true"), "true"))
	if err != nil {
		log.Fatal(err)
	}
	endpoints := data.Endpoints{Product: common.Env("PRODUCT_GRPC_ADDR", "127.0.0.1:9031"), Inventory: common.Env("INVENTORY_GRPC_ADDR", "127.0.0.1:9032"), Order: common.Env("ORDER_GRPC_ADDR", "127.0.0.1:9033"), Payment: common.Env("PAYMENT_GRPC_ADDR", "127.0.0.1:9034"), Saga: common.Env("SAGA_GRPC_ADDR", "127.0.0.1:9035")}
	remote, err := data.NewRemote(context.Background(), *role, endpoints)
	if err != nil {
		log.Fatal(err)
	}
	defer remote.Close()
	uc := biz.New(repo, remote)
	gs, hs, err := server.New(*role, common.Env("TRADE_GRPC_ADDR", pair[1]), common.Env("TRADE_ADDR", pair[0]), service.New(uc))
	if err != nil {
		log.Fatal(err)
	}
	app := kratos.New(kratos.Name("trade-"+*role), kratos.Server(gs, hs))
	if err = app.Run(); err != nil {
		log.Fatal(err)
	}
}
