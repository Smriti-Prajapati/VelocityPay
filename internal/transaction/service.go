package transaction

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/fraud"
	"github.com/velocitypay/velocitypay/internal/metrics"
	"github.com/velocitypay/velocitypay/internal/rabbitmq"
	redisc "github.com/velocitypay/velocitypay/internal/redis"
	"github.com/velocitypay/velocitypay/internal/users"
	"github.com/velocitypay/velocitypay/internal/wallet"
	"go.uber.org/zap"
)

const (
	workerCount        = 10
	jobQueueSize       = 500
	idempotencyTTL     = 24 * time.Hour
	idempotencyPrefix  = "idempotency:"
	transferTimeout    = 30 * time.Second
)

// Service orchestrates transaction processing.
type Service struct {
	repo        Repository
	walletRepo  wallet.Repository
	userRepo    users.Repository
	fraudSvc    *fraud.Service
	publisher   *rabbitmq.Publisher
	cache       *redisc.Client
	metrics     *metrics.Metrics
	pool        *WorkerPool
	log         *zap.Logger
}

// NewService wires up the transaction service and starts the worker pool.
func NewService(
	repo Repository,
	walletRepo wallet.Repository,
	userRepo users.Repository,
	fraudSvc *fraud.Service,
	publisher *rabbitmq.Publisher,
	cache *redisc.Client,
	m *metrics.Metrics,
	log *zap.Logger,
) *Service {
	svc := &Service{
		repo:       repo,
		walletRepo: walletRepo,
		userRepo:   userRepo,
		fraudSvc:   fraudSvc,
		publisher:  publisher,
		cache:      cache,
		metrics:    m,
		log:        log,
	}

	svc.pool = NewWorkerPool(workerCount, jobQueueSize, svc.processTransfer, log)
	return svc
}

// StartWorkers launches the background worker pool. Call this after wiring.
func (s *Service) StartWorkers(ctx context.Context) {
	s.pool.Start(ctx, workerCount)
	s.log.Info("transaction worker pool started", zap.Int("workers", workerCount))
}

// Shutdown gracefully stops the worker pool.
func (s *Service) Shutdown() {
	s.pool.Shutdown()
}

// Transfer initiates a fund transfer between two wallets.
// It enforces idempotency, validates the receiver, then enqueues a job.
func (s *Service) Transfer(ctx context.Context, senderID uuid.UUID, req *TransferRequest) (*Transaction, error) {
	// 1. Resolve idempotency key
	idempKey := req.IdempotencyKey
	if idempKey == "" {
		idempKey = uuid.New().String()
	}

	// 2. Check idempotency cache — if the key was recently used, return the existing transaction
	cacheKey := idempotencyPrefix + idempKey
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != "" {
		existing, err := s.repo.FindByIdempotencyKey(ctx, idempKey)
		if err == nil {
			s.log.Info("idempotent request — returning existing transaction",
				zap.String("idempotency_key", idempKey),
			)
			return existing, nil
		}
	}

	// 3. Resolve sender wallet
	senderWallet, err := s.walletRepo.FindByUserID(ctx, senderID)
	if err != nil {
		if errors.Is(err, wallet.ErrNotFound) {
			return nil, errors.New("sender wallet not found")
		}
		return nil, fmt.Errorf("find sender wallet: %w", err)
	}

	// 4. Resolve receiver wallet
	receiverWallet, err := s.walletRepo.FindByWalletNumber(ctx, req.ReceiverWalletNumber)
	if err != nil {
		if errors.Is(err, wallet.ErrNotFound) {
			return nil, errors.New("receiver wallet not found")
		}
		return nil, fmt.Errorf("find receiver wallet: %w", err)
	}

	if senderWallet.ID == receiverWallet.ID {
		return nil, errors.New("cannot transfer to your own wallet")
	}

	// 5. Quick balance pre-check (non-authoritative — the DB constraint is authoritative)
	if senderWallet.Balance < req.Amount {
		return nil, wallet.ErrInsufficientBalance
	}

	// 6. Fraud check — load sender account age then evaluate all rules
	sender, err := s.userRepo.FindByID(ctx, senderID)
	if err != nil {
		return nil, fmt.Errorf("load sender for fraud check: %w", err)
	}

	fraudReq := fraud.BuildCheckRequest(senderID, uuid.New(), req.Amount, senderWallet.Balance, sender.CreatedAt)
	fraudResult := s.fraudSvc.Check(ctx, fraudReq)
	if !fraudResult.Allowed {
		return nil, errors.New("transaction blocked by fraud detection")
	}

	// 6. Create a pending transaction record
	now := time.Now().UTC()
	txn := &Transaction{
		ID:              uuid.New(),
		SenderID:        senderID,
		ReceiverID:      receiverWallet.UserID,
		Amount:          roundAmount(req.Amount),
		TransactionType: TypeTransfer,
		Status:          StatusPending,
		Notes:           req.Notes,
		IdempotencyKey:  idempKey,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	if err := s.repo.Create(ctx, dbTx, txn); err != nil {
		_ = dbTx.Rollback()
		return nil, fmt.Errorf("persist pending transaction: %w", err)
	}
	if err := dbTx.Commit(); err != nil {
		return nil, fmt.Errorf("commit pending transaction: %w", err)
	}

	// 7. Cache idempotency key
	_ = s.cache.Set(ctx, cacheKey, txn.ID.String(), idempotencyTTL)

	// 8. Enqueue for worker pool processing
	resultCh := make(chan TransferResult, 1)
	job := TransferJob{
		SenderID:         senderID,
		ReceiverID:       receiverWallet.UserID,
		SenderWalletID:   senderWallet.ID,
		ReceiverWalletID: receiverWallet.ID,
		Amount:           txn.Amount,
		Notes:            req.Notes,
		Transaction:      txn,
		ResultCh:         resultCh,
	}

	if !s.pool.Submit(job) {
		// Queue full — mark transaction as failed
		_ = s.repo.UpdateStatus(ctx, txn.ID, StatusFailed, "system overloaded, please retry")
		return nil, errors.New("transaction queue full, please retry shortly")
	}

	// 9. Wait for processing result with a timeout
	select {
	case result := <-resultCh:
		return result.Transaction, result.Err
	case <-time.After(transferTimeout):
		return nil, errors.New("transaction processing timed out")
	}
}

// processTransfer is the worker function — runs inside the pool goroutines.
// It executes the atomic PostgreSQL transaction:
//   BEGIN → debit sender → credit receiver → update status → COMMIT
func (s *Service) processTransfer(ctx context.Context, job TransferJob) {
	txCtx, cancel := context.WithTimeout(ctx, transferTimeout)
	defer cancel()

	result := TransferResult{}

	dbTx, err := s.walletRepo.BeginTx(txCtx)
	if err != nil {
		result.Err = fmt.Errorf("begin db tx: %w", err)
		job.ResultCh <- result
		return
	}

	// Debit sender
	if err := s.walletRepo.UpdateBalance(txCtx, dbTx, job.SenderWalletID, -job.Amount); err != nil {
		_ = dbTx.Rollback()
		_ = s.repo.UpdateStatus(ctx, job.Transaction.ID, StatusFailed, err.Error())
		s.publishFailed(job, err.Error())
		result.Err = err
		job.ResultCh <- result
		return
	}

	// Credit receiver
	if err := s.walletRepo.UpdateBalance(txCtx, dbTx, job.ReceiverWalletID, job.Amount); err != nil {
		_ = dbTx.Rollback()
		_ = s.repo.UpdateStatus(ctx, job.Transaction.ID, StatusFailed, err.Error())
		s.publishFailed(job, err.Error())
		result.Err = err
		job.ResultCh <- result
		return
	}

	if err := dbTx.Commit(); err != nil {
		_ = s.repo.UpdateStatus(ctx, job.Transaction.ID, StatusFailed, "commit failed")
		s.publishFailed(job, "commit failed")
		result.Err = fmt.Errorf("commit transfer: %w", err)
		job.ResultCh <- result
		return
	}

	// Mark completed
	if err := s.repo.UpdateStatus(ctx, job.Transaction.ID, StatusCompleted, ""); err != nil {
		s.log.Error("failed to mark transaction completed", zap.Error(err), zap.String("txn_id", job.Transaction.ID.String()))
	}

	job.Transaction.Status = StatusCompleted
	job.Transaction.UpdatedAt = time.Now().UTC()

	// Publish completion event
	_ = s.publisher.Publish(ctx, rabbitmq.EventTransactionCompleted, map[string]interface{}{
		"transaction_id": job.Transaction.ID,
		"sender_id":      job.SenderID,
		"receiver_id":    job.ReceiverID,
		"amount":         job.Amount,
		"status":         StatusCompleted,
	})

	s.metrics.TransactionsTotal.WithLabelValues("completed").Inc()
	s.metrics.TransactionAmount.WithLabelValues("transfer").Observe(job.Amount)

	s.log.Info("transfer completed",
		zap.String("txn_id", job.Transaction.ID.String()),
		zap.Float64("amount", job.Amount),
		zap.String("sender_id", job.SenderID.String()),
		zap.String("receiver_id", job.ReceiverID.String()),
	)

	result.Transaction = job.Transaction
	job.ResultCh <- result
}

func (s *Service) publishFailed(job TransferJob, reason string) {
	_ = s.publisher.Publish(context.Background(), rabbitmq.EventTransactionFailed, map[string]interface{}{
		"transaction_id": job.Transaction.ID,
		"sender_id":      job.SenderID,
		"receiver_id":    job.ReceiverID,
		"amount":         job.Amount,
		"reason":         reason,
	})
	s.metrics.TransactionsTotal.WithLabelValues("failed").Inc()
}

// GetByID retrieves a single transaction, enforcing that the requester is a party to it.
func (s *Service) GetByID(ctx context.Context, requesterID uuid.UUID, txnID uuid.UUID) (*Transaction, error) {
	txn, err := s.repo.FindByID(ctx, txnID)
	if err != nil {
		return nil, err
	}

	if txn.SenderID != requesterID && txn.ReceiverID != requesterID {
		return nil, errors.New("access denied")
	}

	return txn, nil
}

// GetHistory returns paginated transaction history for the authenticated user.
func (s *Service) GetHistory(ctx context.Context, userID uuid.UUID, filter HistoryFilter) (*HistoryResponse, error) {
	txns, total, err := s.repo.ListByUserID(ctx, userID, filter)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}

	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.Page < 1 {
		filter.Page = 1
	}

	totalPages := int(math.Ceil(float64(total) / float64(filter.PageSize)))

	return &HistoryResponse{
		Transactions: txns,
		Total:        total,
		Page:         filter.Page,
		PageSize:     filter.PageSize,
		TotalPages:   totalPages,
	}, nil
}

// roundAmount truncates to 2 decimal places to avoid floating-point drift.
func roundAmount(amount float64) float64 {
	return math.Round(amount*100) / 100
}
