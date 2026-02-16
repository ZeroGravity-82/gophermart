package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"zerogravity-82/gophermart/internal/accrual"
	"zerogravity-82/gophermart/internal/model"
)

// AccrualWorker периодически опрашивает внешний сервис расчета начислений баллов лояльности и обновляет заказы в
// соответствии с полученными данными.
type accrualClient interface {
	GetAccrual(ctx context.Context, orderNumber string) (accrual.GetAccrualAPIResponse, error)
}

type AccrualWorker struct {
	orderRepo     orderAccrualRepository
	accrualClient accrualClient
	pollInterval  time.Duration
	batchSize     int
}

// NewAccrualWorker создает AccrualWorker.
//
// pollInterval задает период опроса внешнего сервиса расчета начислений баллов лояльности.
// batchSize задает максимальное количество заказов, обрабатываемых за одну итерацию.
func NewAccrualWorker(
	orderRepo orderAccrualRepository,
	accrualClient accrualClient,
	pollInterval time.Duration,
	batchSize int,
) (*AccrualWorker, error) {
	if pollInterval <= 0 {
		return nil, errors.New("poll interval must be positive")
	}
	if batchSize <= 0 {
		return nil, errors.New("batch size must be positive")
	}
	return &AccrualWorker{
		orderRepo:     orderRepo,
		accrualClient: accrualClient,
		pollInterval:  pollInterval,
		batchSize:     batchSize,
	}, nil
}

type orderAccrualRepository interface {
	ListForAccrualProcessing(ctx context.Context, limit int) ([]model.Order, error)
	ApplyAccrualResult(ctx context.Context, orderNumber string, status model.OrderStatus, accrualAmount *uint64) error
}

// Run запускает цикл обработки и работает до отмены контекста или возникновения ошибки.
func (w *AccrualWorker) Run(ctx context.Context) error {
	timer := time.NewTimer(w.pollInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		case <-timer.C:
			if err := w.processOrdersWithAccrual(ctx); err != nil {
				slog.Error("failed to process orders with accrual", slog.Any("err", err))
			}
			timer.Reset(w.pollInterval)
		}
	}
}

func (w *AccrualWorker) processOrdersWithAccrual(ctx context.Context) error {
	orders, err := w.orderRepo.ListForAccrualProcessing(ctx, w.batchSize)
	if err != nil {
		return fmt.Errorf("failed to list orders for accrual: %w", err)
	}
	for _, o := range orders {
		if err := w.processOrder(ctx, o.Number); err != nil {
			// При первой ошибке выходим, чтобы не создавать лишнюю нагрузку.
			return err
		}
	}
	return nil
}

func (w *AccrualWorker) processOrder(ctx context.Context, orderNumber string) error {
	resp, err := w.accrualClient.GetAccrual(ctx, orderNumber)
	if err != nil {
		if errors.Is(err, accrual.ErrOrderNotProcessed) {
			// Заказ еще не зарегистрирован в сервисе расчета начислений баллов лояльности.
			return nil
		}
		return fmt.Errorf("failed to get accrual for order %s: %w", orderNumber, err)
	}

	status, accrualAmount, err := mapAccrualResponse(resp)
	if err != nil {
		return err
	}

	if err := w.orderRepo.ApplyAccrualResult(ctx, orderNumber, status, accrualAmount); err != nil {
		return fmt.Errorf("failed to apply accrual result: %w", err)
	}
	return nil
}

func mapAccrualResponse(resp accrual.GetAccrualAPIResponse) (model.OrderStatus, *uint64, error) {
	switch resp.Status {
	case "REGISTERED":
		return model.OrderStatusProcessing, nil, nil
	case "PROCESSING":
		return model.OrderStatusProcessing, nil, nil
	case "INVALID":
		minorUnits := uint64(0)
		return model.OrderStatusInvalid, &minorUnits, nil
	case "PROCESSED":
		var minorUnits uint64
		if resp.Accrual == nil {
			minorUnits = 0
		} else {
			var ok bool
			minorUnits, ok = model.AccrualMajorToMinor(*resp.Accrual)
			if !ok {
				return "", nil, errors.New("invalid accrual amount")
			}
		}
		return model.OrderStatusProcessed, &minorUnits, nil
	default:
		return "", nil, fmt.Errorf("unknown accrual status: %s", resp.Status)
	}
}
