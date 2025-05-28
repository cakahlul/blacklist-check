package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

// BlacklistRecord represents a blacklist record in the database
type BlacklistRecord struct {
	ID          int64     `db:"id"`
	NIK         string    `db:"nik"`
	Name        string    `db:"name"`
	BirthPlace  string    `db:"birth_place"`
	BirthDate   time.Time `db:"birth_date"`
	Reason      string    `db:"reason"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	Similarity  float64   `db:"similarity"`
}

// BlacklistStore defines the interface for blacklist data access
type BlacklistStore interface {
	GetByNIK(ctx context.Context, nik string) (*BlacklistRecord, error)
	GetByFuzzyMatch(ctx context.Context, name string, birthPlace *string, birthDate *time.Time) ([]*BlacklistRecord, error)
	SearchByName(ctx context.Context, name string) ([]*BlacklistRecord, error)
	Ping(ctx context.Context) error
}

// blacklistStore implements BlacklistStore
type blacklistStore struct {
	db *sqlx.DB
}

// NewBlacklistStore creates a new blacklist store
func NewBlacklistStore(db *sqlx.DB) BlacklistStore {
	return &blacklistStore{db: db}
}

// GetByNIK retrieves a blacklist record by NIK
func (s *blacklistStore) GetByNIK(ctx context.Context, nik string) (*BlacklistRecord, error) {
	var record BlacklistRecord
	err := s.db.GetContext(ctx, &record, `
		SELECT id, nik, name, birth_place, birth_date, reason, created_at, updated_at
		FROM blacklist
		WHERE nik = $1
	`, nik)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// GetByFuzzyMatch performs an efficient fuzzy match using PostgreSQL's trigram similarity
func (s *blacklistStore) GetByFuzzyMatch(ctx context.Context, name string, birthPlace *string, birthDate *time.Time) ([]*BlacklistRecord, error) {
	var records []*BlacklistRecord
	var err error

	// Minimum similarity threshold (0.3 is a good balance between accuracy and performance)
	const minSimilarity = 0.3

	if birthDate != nil && birthPlace != nil {
		// Full match with name similarity, exact birth date, and birth place similarity
		err = s.db.SelectContext(ctx, &records, `
			WITH name_matches AS (
				SELECT 
					id, nik, name, birth_place, birth_date, reason, created_at, updated_at,
					similarity(name, $1) as similarity
				FROM blacklist
				WHERE similarity(name, $1) > $4
					AND birth_date = $2
					AND similarity(birth_place, $3) > $4
				ORDER BY similarity DESC
				LIMIT 5
			)
			SELECT * FROM name_matches
			WHERE similarity > $4
		`, name, birthDate, *birthPlace, minSimilarity)
	} else if birthDate != nil {
		// Match with name similarity and exact birth date
		err = s.db.SelectContext(ctx, &records, `
			WITH name_matches AS (
				SELECT 
					id, nik, name, birth_place, birth_date, reason, created_at, updated_at,
					similarity(name, $1) as similarity
				FROM blacklist
				WHERE similarity(name, $1) > $3
					AND birth_date = $2
				ORDER BY similarity DESC
				LIMIT 5
			)
			SELECT * FROM name_matches
			WHERE similarity > $3
		`, name, birthDate, minSimilarity)
	} else if birthPlace != nil {
		// Match with name and birth place similarity
		err = s.db.SelectContext(ctx, &records, `
			WITH name_matches AS (
				SELECT 
					id, nik, name, birth_place, birth_date, reason, created_at, updated_at,
					similarity(name, $1) as similarity
				FROM blacklist
				WHERE similarity(name, $1) > $3
					AND similarity(birth_place, $2) > $3
				ORDER BY similarity DESC
				LIMIT 5
			)
			SELECT * FROM name_matches
			WHERE similarity > $3
		`, name, *birthPlace, minSimilarity)
	} else {
		// Name-only match with similarity
		err = s.db.SelectContext(ctx, &records, `
			WITH name_matches AS (
				SELECT 
					id, nik, name, birth_place, birth_date, reason, created_at, updated_at,
					similarity(name, $1) as similarity
				FROM blacklist
				WHERE similarity(name, $1) > $2
				ORDER BY similarity DESC
				LIMIT 5
			)
			SELECT * FROM name_matches
			WHERE similarity > $2
		`, name, minSimilarity)
	}

	if err != nil {
		return nil, err
	}

	return records, nil
}

// SearchByName searches for blacklist records by name using fuzzy matching
func (s *blacklistStore) SearchByName(ctx context.Context, name string) ([]*BlacklistRecord, error) {
	var records []*BlacklistRecord
	const minSimilarity = 0.3

	err := s.db.SelectContext(ctx, &records, `
		WITH name_matches AS (
			SELECT 
				id, nik, name, birth_place, birth_date, reason, created_at, updated_at,
				similarity(name, $1) as similarity
			FROM blacklist
			WHERE similarity(name, $1) > $2
			ORDER BY similarity DESC
			LIMIT 5
		)
		SELECT * FROM name_matches
		WHERE similarity > $2
	`, name, minSimilarity)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (s *blacklistStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
} 