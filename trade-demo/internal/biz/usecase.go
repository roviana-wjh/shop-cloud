package biz

import (
	"context"
	"github.com/go-kratos/kratos/v2/errors"
	"sync"
	"trade-demo/internal/common"
)

// Repository is the local transaction boundary; every service has its own repository.
type Repository interface {
	Read(func(common.State))
	Update(func(*common.State) error) error
}

// Remote describes outbound domain operations. The data layer implements it with generated gRPC clients.
type Remote interface {
	GetProduct(context.Context, string) (common.Product, error)
	GetOrder(context.Context, string) (common.Order, error)
	ChangeOrder(context.Context, string, string) (common.Order, error)
	ReserveStock(context.Context, common.Reservation) (common.Reservation, error)
	ChangeReservation(context.Context, string, string) (common.Reservation, error)
	CreatePayment(context.Context, string, int64) (common.Payment, error)
	GetPayment(context.Context, string) (common.Payment, error)
	StartSaga(context.Context, string) (common.Saga, error)
	NotifyPayment(context.Context, common.Payment) (common.Saga, error)
}

type Usecase struct {
	repo   Repository
	remote Remote
	sagaMu sync.Mutex
}

func New(repo Repository, remote Remote) *Usecase { return &Usecase{repo: repo, remote: remote} }
func invalid(message string) error                { return errors.BadRequest("INVALID_ARGUMENT", message) }
func conflict(message string) error               { return errors.Conflict("STATE_CONFLICT", message) }
func missing(message string) error                { return errors.NotFound("NOT_FOUND", message) }
func storage(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*errors.Error); ok {
		return err
	}
	return errors.InternalServer("STORAGE_ERROR", err.Error())
}
