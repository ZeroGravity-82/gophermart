package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"zerogravity-82/gophermart/internal/accrual"
	"zerogravity-82/gophermart/internal/logging"
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
	workers       int
}

// NewAccrualWorker создает AccrualWorker.
//
// pollInterval задает период опроса внешнего сервиса расчета начислений баллов лояльности.
//
// batchSize задает максимальное количество заказов, обрабатываемых за одну итерацию.
//
// workers задает количество воркеров (максимум параллельных запросов) для опроса внешнего сервиса расчета начислений
// баллов лояльности.
func NewAccrualWorker(
	orderRepo orderAccrualRepository,
	accrualClient accrualClient,
	pollInterval time.Duration,
	batchSize int,
	workers int,
) (*AccrualWorker, error) {
	if pollInterval <= 0 {
		return nil, errors.New("poll interval must be positive")
	}
	if batchSize <= 0 {
		return nil, errors.New("batch size must be positive")
	}
	if workers <= 0 {
		return nil, errors.New("workers must be positive")
	}
	return &AccrualWorker{
		orderRepo:     orderRepo,
		accrualClient: accrualClient,
		pollInterval:  pollInterval,
		batchSize:     batchSize,
		workers:       workers,
	}, nil
}

type orderAccrualRepository interface {
	ListForAccrualProcessing(ctx context.Context, limit int) ([]model.Order, error)
	ApplyAccrualResult(ctx context.Context, orderNumber string, status model.OrderStatus, accrualAmount *uint64) error
}

// Run запускает цикл обработки и работает до отмены контекста.
func (w *AccrualWorker) Run(ctx context.Context) {
	logger := logging.FromContext(ctx)
	logger.Info(
		"starting accrual worker",
		slog.String("poll_interval", w.pollInterval.String()),
		slog.Int("batch_size", w.batchSize),
		slog.Int("workers", w.workers),
	)

	orderNumberCh := make(chan string, w.batchSize)
	defer close(orderNumberCh)

	var wg sync.WaitGroup
	w.startWorkers(ctx, logger, orderNumberCh, &wg)

	timer := time.NewTimer(w.pollInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-timer.C:
			if err := w.processOrdersWithAccrual(ctx, orderNumberCh); err != nil {
				logger.Error("failed to process orders with accrual", slog.Any("err", err))
			}
			timer.Reset(w.pollInterval)
		}
	}
}

func (w *AccrualWorker) startWorkers(
	ctx context.Context,
	logger *slog.Logger,
	orderNumberCh <-chan string,
	wg *sync.WaitGroup,
) {
	wg.Add(w.workers)
	for i := 0; i < w.workers; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case orderNumber, ok := <-orderNumberCh:
					if !ok {
						return
					}
					if err := w.processOrder(ctx, orderNumber); err != nil {
						logger.Error(
							"failed to process the order with accrual",
							slog.String("order", orderNumber),
							slog.Any("err", err),
						)
					}
				}
			}
		}()
	}
}

func (w *AccrualWorker) processOrdersWithAccrual(ctx context.Context, orderNumberCh chan<- string) error {
	orders, err := w.orderRepo.ListForAccrualProcessing(ctx, w.batchSize)
	if err != nil {
		return fmt.Errorf("failed to list orders for accrual: %w", err)
	}
	for _, o := range orders {
		select {
		case <-ctx.Done():
			return nil
		case orderNumberCh <- o.Number:
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
			var err error
			minorUnits, err = model.AccrualMajorToMinor(*resp.Accrual)
			if err != nil {
				return "", nil, fmt.Errorf("invalid accrual amount: %w", err)
			}
		}
		return model.OrderStatusProcessed, &minorUnits, nil
	default:
		return "", nil, fmt.Errorf("unknown accrual status: %s", resp.Status)
	}
}
