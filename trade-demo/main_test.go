package main

import (
	"bytes"
	"context"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"
	v1 "trade-demo/api/trade/v1"
	"trade-demo/internal/biz"
	"trade-demo/internal/common"
	"trade-demo/internal/data"
	"trade-demo/internal/server"
	"trade-demo/internal/service"
)

func TestKratosGRPCTrade(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	roles := []string{"product", "inventory", "order", "payment", "saga"}
	addresses := map[string]string{}
	httpAddresses := map[string]string{}
	remotes := map[string]*data.Remote{}
	var starters []func()
	for _, role := range roles {
		repo, err := data.OpenRepository(filepath.Join(dir, role+".json"), role, true)
		if err != nil {
			t.Fatal(err)
		}
		remote := &data.Remote{}
		remotes[role] = remote
		uc := biz.New(repo, remote)
		gs, hs, err := server.New(role, "127.0.0.1:0", "127.0.0.1:0", service.New(uc))
		if err != nil {
			t.Fatal(err)
		}
		ge, err := gs.Endpoint()
		if err != nil {
			t.Fatal(err)
		}
		he, err := hs.Endpoint()
		if err != nil {
			t.Fatal(err)
		}
		addresses[role] = ge.Host
		httpAddresses[role] = "http://" + he.Host
		starts := func() {
			go func() { _ = gs.Start(context.Background()) }()
			go func() { _ = hs.Start(context.Background()) }()
		}
		starters = append(starters, starts)
		t.Cleanup(func() {
			stopCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
			defer c()
			_ = hs.Stop(stopCtx)
			_ = gs.Stop(stopCtx)
			remote.Close()
		})
	}
	e := data.Endpoints{Product: addresses["product"], Inventory: addresses["inventory"], Order: addresses["order"], Payment: addresses["payment"], Saga: addresses["saga"]}
	for _, role := range roles {
		r, err := data.NewRemote(ctx, role, e)
		if err != nil {
			t.Fatal(err)
		}
		*remotes[role] = *r
	}
	for _, start := range starters {
		start()
	}
	oc, err := kgrpc.DialInsecure(ctx, kgrpc.WithEndpoint(e.Order), kgrpc.WithTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer oc.Close()
	pc, err := kgrpc.DialInsecure(ctx, kgrpc.WithEndpoint(e.Payment), kgrpc.WithTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	ic, err := kgrpc.DialInsecure(ctx, kgrpc.WithEndpoint(e.Inventory))
	if err != nil {
		t.Fatal(err)
	}
	defer ic.Close()
	sc, err := kgrpc.DialInsecure(ctx, kgrpc.WithEndpoint(e.Saga))
	if err != nil {
		t.Fatal(err)
	}
	defer sc.Close()
	orders, payments, inventory, sagas := v1.NewOrderServiceClient(oc), v1.NewPaymentServiceClient(pc), v1.NewInventoryServiceClient(ic), v1.NewSagaServiceClient(sc)
	var completedSaga string
	for _, result := range []string{"success", "fail"} {
		t.Run(result, func(t *testing.T) {
			created, err := orders.CreateOrder(ctx, &v1.CreateOrderRequest{UserId: "test", ProductId: "tea", Quantity: 2})
			if err != nil {
				t.Fatal(err)
			}
			if created.Saga.Status != "WAITING_FOR_PAYMENT" || created.Order.PaymentId != created.Saga.PaymentId || created.Order.AmountFen != 2580 {
				t.Fatalf("unexpected created order: %v", created)
			}
			// Duplicate Saga start returns the same ID without reserving stock twice.
			duplicate, err := sagas.StartSaga(ctx, &v1.StartSagaRequest{OrderId: created.Order.Id})
			if err != nil || duplicate.Id != created.Saga.Id {
				t.Fatalf("duplicate saga: %v %v", duplicate, err)
			}
			for i := 0; i < 2; i++ {
				if _, err = payments.SimulatePayment(ctx, &v1.SimulatePaymentRequest{Id: created.Saga.PaymentId, Result: result}); err != nil {
					t.Fatal(err)
				}
			}
			o, err := orders.GetOrder(ctx, &v1.IDRequest{Id: created.Order.Id})
			if err != nil {
				t.Fatal(err)
			}
			s, err := sagas.GetSaga(ctx, &v1.IDRequest{Id: created.Saga.Id})
			if err != nil {
				t.Fatal(err)
			}
			wantOrder, wantSaga := "PAID", "COMPLETED"
			if result == "fail" {
				wantOrder, wantSaga = "CANCELLED", "FAILED"
			} else {
				completedSaga = s.Id
			}
			if o.Status != wantOrder || s.Status != wantSaga {
				t.Fatalf("order=%s saga=%s", o.Status, s.Status)
			}
			opposite := "success"
			if result == "success" {
				opposite = "fail"
			}
			_, err = payments.SimulatePayment(ctx, &v1.SimulatePaymentRequest{Id: created.Saga.PaymentId, Result: opposite})
			if kerrors.Code(err) != 409 {
				t.Fatalf("conflicting payment result should be 409: %v", err)
			}
		})
	}
	stock, err := inventory.GetStock(ctx, &v1.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if stock.Stock["tea"] != 18 {
		t.Fatalf("stock after compensation=%d", stock.Stock["tea"])
	}
	t.Run("out of stock cancels order", func(t *testing.T) {
		v, err := orders.CreateOrder(ctx, &v1.CreateOrderRequest{UserId: "test", ProductId: "mug", Quantity: 99})
		if err != nil {
			t.Fatal(err)
		}
		if v.Saga.Status != "FAILED" || v.Order.Status != "CANCELLED" || v.Order.PaymentId != "" {
			t.Fatalf("unexpected failed order %v", v)
		}
	})
	t.Run("HTTP uses generated Kratos binding", func(t *testing.T) {
		resp, err := http.Post(httpAddresses["order"]+"/orders", "application/json", bytes.NewBufferString(`{"userId":"http","productId":"mug","quantity":1}`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			t.Fatalf("HTTP %d: %s", resp.StatusCode, b)
		}
		var v v1.CreateOrderReply
		if err = protojson.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		if v.Order.AmountFen != 3990 {
			t.Fatalf("unexpected HTTP reply %v", &v)
		}
		// Internal mutation operations are available over gRPC only.
		resp2, err := http.Post(httpAddresses["order"]+"/internal/orders/"+v.Order.Id+"/paid", "application/json", bytes.NewBufferString(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		resp2.Body.Close()
		if resp2.StatusCode != 404 {
			t.Fatalf("internal HTTP route must be removed, got %d", resp2.StatusCode)
		}
	})
	reopened, err := data.OpenRepository(filepath.Join(dir, "saga.json"), "saga", true)
	if err != nil {
		t.Fatal(err)
	}
	reopened.Read(func(st common.State) {
		if st.Sagas[completedSaga].Status != "COMPLETED" {
			t.Fatal("Saga completion was not persisted")
		}
	})
}
