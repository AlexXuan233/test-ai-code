package store

import (
	"context"
	"time"

	"fraud-scorer/internal/models"

	"gorm.io/gorm"
)

// TransactionStore defines the interface for transaction persistence and queries.
type TransactionStore interface {
	Create(ctx context.Context, tx *models.Transaction) error
	CountByCardHash(ctx context.Context, cardHash string, since time.Time) (int64, error)
	CountByEmail(ctx context.Context, email string, since time.Time) (int64, error)
	CountByIP(ctx context.Context, ip string, since time.Time) (int64, error)
	CountDistinctCardsByShipping(ctx context.Context, shipping string, since time.Time) (int64, error)
	AvgAmountByCustomer(ctx context.Context, customerID string) (float64, error)
	List(ctx context.Context, limit, offset int) ([]models.Transaction, error)
	UpdateConfirmedStatus(ctx context.Context, transactionID, status string) error
	CountHighRiskByIP(ctx context.Context, ip string, since time.Time) (int64, error)
}

// GORMStore implements TransactionStore using GORM.
type GORMStore struct {
	db *gorm.DB
}

// NewGORMStore creates a new GORM-backed store.
func NewGORMStore(db *gorm.DB) TransactionStore {
	return &GORMStore{db: db}
}

// Create persists a transaction.
func (s *GORMStore) Create(ctx context.Context, tx *models.Transaction) error {
	return s.db.WithContext(ctx).Create(tx).Error
}

// CountByCardHash counts transactions for a card hash since a given time.
func (s *GORMStore) CountByCardHash(ctx context.Context, cardHash string, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.Transaction{}).
		Where("card_hash = ? AND timestamp >= ?", cardHash, since).
		Count(&count).Error
	return count, err
}

// CountByEmail counts transactions for an email since a given time.
func (s *GORMStore) CountByEmail(ctx context.Context, email string, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.Transaction{}).
		Where("customer_email = ? AND timestamp >= ?", email, since).
		Count(&count).Error
	return count, err
}

// CountByIP counts transactions for an IP since a given time.
func (s *GORMStore) CountByIP(ctx context.Context, ip string, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.Transaction{}).
		Where("ip_address = ? AND timestamp >= ?", ip, since).
		Count(&count).Error
	return count, err
}

// CountDistinctCardsByShipping counts distinct cards used for a shipping address since a given time.
func (s *GORMStore) CountDistinctCardsByShipping(ctx context.Context, shipping string, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.Transaction{}).
		Select("COUNT(DISTINCT card_hash)").
		Where("shipping_address = ? AND timestamp >= ?", shipping, since).
		Scan(&count).Error
	return count, err
}

// AvgAmountByCustomer returns the average transaction amount for a customer.
func (s *GORMStore) AvgAmountByCustomer(ctx context.Context, customerID string) (float64, error) {
	var avg float64
	err := s.db.WithContext(ctx).Model(&models.Transaction{}).
		Select("COALESCE(AVG(amount), 0)").
		Where("customer_id = ?", customerID).
		Scan(&avg).Error
	return avg, err
}

// List returns recent transactions.
func (s *GORMStore) List(ctx context.Context, limit, offset int) ([]models.Transaction, error) {
	var txs []models.Transaction
	err := s.db.WithContext(ctx).Order("timestamp DESC").Limit(limit).Offset(offset).Find(&txs).Error
	return txs, err
}

// UpdateConfirmedStatus updates the confirmed fraud/legitimate status.
func (s *GORMStore) UpdateConfirmedStatus(ctx context.Context, transactionID, status string) error {
	return s.db.WithContext(ctx).Model(&models.Transaction{}).
		Where("transaction_id = ?", transactionID).
		Update("confirmed_status", status).Error
}

// CountHighRiskByIP counts high-risk transactions (score >= 70) from an IP since a given time.
func (s *GORMStore) CountHighRiskByIP(ctx context.Context, ip string, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.Transaction{}).
		Where("ip_address = ? AND score >= 70 AND timestamp >= ?", ip, since).
		Count(&count).Error
	return count, err
}
