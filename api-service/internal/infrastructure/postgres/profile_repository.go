package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/fernandobarroso/microservices/api-service/internal/domain/profile"
	"github.com/fernandobarroso/microservices/api-service/internal/pkg/logger"
)

// ProfileRepository handles database operations for profiles
type ProfileRepository struct {
	db  *sqlx.DB
	log *zap.Logger
}

func NewProfileRepository(db *sqlx.DB, log *zap.Logger) *ProfileRepository {
	return &ProfileRepository{
		db:  db,
		log: log.Named("profile_repository"),
	}
}

func (r *ProfileRepository) checkConnectionHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result int
	if err := r.db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		r.log.Error("Database health check failed", logger.ErrorField(err))
		return fmt.Errorf("database health check failed: %w", err)
	}
	return nil
}

func (r *ProfileRepository) Create(ctx context.Context, p *profile.Profile) error {
	correlationID := getCorrelationID(ctx)
	r.log.Info("Creating new profile",
		logger.String("profile_id", p.ID.String()),
		logger.String("email", p.Email),
		logger.String("correlation_id", correlationID),
	)

	if err := r.checkConnectionHealth(ctx); err != nil {
		return fmt.Errorf("connection health check failed: %w", err)
	}

	tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			r.log.Error("Transaction panic",
				zap.Any("panic", p),
				logger.String("correlation_id", correlationID),
			)
			panic(p)
		}
	}()

	query := `
		INSERT INTO profiles (id, first_name, last_name, email, phone)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`

	if err := tx.QueryRowxContext(ctx, query,
		p.ID, p.FirstName, p.LastName, p.Email, p.Phone,
	).Scan(&p.CreatedAt, &p.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("error creating profile: %w", err)
	}

	for i := range p.Addresses {
		addr := &p.Addresses[i]
		addr.ProfileID = p.ID
		if err := r.createAddressTx(ctx, tx, addr); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	for i := range p.Contacts {
		contact := &p.Contacts[i]
		contact.ProfileID = p.ID
		if err := r.createContactTx(ctx, tx, contact); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	r.log.Info("Successfully created profile",
		logger.String("profile_id", p.ID.String()),
		logger.String("correlation_id", correlationID),
	)
	return nil
}

func (r *ProfileRepository) GetByID(ctx context.Context, id uuid.UUID) (*profile.Profile, error) {
	query := `
		SELECT id, first_name, last_name, email, phone, created_at, updated_at
		FROM profiles
		WHERE id = $1`

	var p profile.Profile
	if err := r.db.GetContext(ctx, &p, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting profile: %w", err)
	}

	addresses, err := r.getAddresses(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Addresses = addresses

	contacts, err := r.getContacts(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Contacts = contacts

	return &p, nil
}

func (r *ProfileRepository) GetByEmail(ctx context.Context, email string) (*profile.Profile, error) {
	query := `
		SELECT id, first_name, last_name, email, phone, created_at, updated_at
		FROM profiles
		WHERE email = $1`

	var p profile.Profile
	if err := r.db.GetContext(ctx, &p, query, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting profile by email: %w", err)
	}

	addresses, err := r.getAddresses(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.Addresses = addresses

	contacts, err := r.getContacts(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.Contacts = contacts

	return &p, nil
}

func (r *ProfileRepository) Update(ctx context.Context, p *profile.Profile) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	query := `
		UPDATE profiles
		SET first_name = $1, last_name = $2, email = $3, phone = $4
		WHERE id = $5
		RETURNING updated_at`

	if err := tx.QueryRowxContext(ctx, query,
		p.FirstName, p.LastName, p.Email, p.Phone, p.ID,
	).Scan(&p.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("error updating profile: %w", err)
	}

	if err := r.updateAddressesTx(ctx, tx, p.ID, p.Addresses); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := r.updateContactsTx(ctx, tx, p.ID, p.Contacts); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (r *ProfileRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	result, err := tx.ExecContext(ctx, `DELETE FROM profiles WHERE id = $1`, id)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("error deleting profile: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("error getting rows affected: %w", err)
	}
	if rows == 0 {
		_ = tx.Rollback()
		return ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (r *ProfileRepository) List(ctx context.Context, page, pageSize int) ([]*profile.Profile, error) {
	offset := (page - 1) * pageSize
	query := `
		SELECT id, first_name, last_name, email, phone, created_at, updated_at
		FROM profiles
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	var profiles []*profile.Profile
	if err := r.db.SelectContext(ctx, &profiles, query, pageSize, offset); err != nil {
		return nil, fmt.Errorf("error listing profiles: %w", err)
	}

	for _, p := range profiles {
		addresses, err := r.getAddresses(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		p.Addresses = addresses

		contacts, err := r.getContacts(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		p.Contacts = contacts
	}

	return profiles, nil
}

func (r *ProfileRepository) createAddressTx(ctx context.Context, tx *sqlx.Tx, addr *profile.Address) error {
	query := `
		INSERT INTO addresses (id, profile_id, street, city, state, country, postal_code, is_primary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`

	return tx.QueryRowxContext(ctx, query,
		addr.ID, addr.ProfileID, addr.Street, addr.City, addr.State,
		addr.Country, addr.PostalCode, addr.IsPrimary,
	).Scan(&addr.CreatedAt, &addr.UpdatedAt)
}

func (r *ProfileRepository) createContactTx(ctx context.Context, tx *sqlx.Tx, contact *profile.Contact) error {
	query := `
		INSERT INTO contacts (id, profile_id, type, value, is_primary)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`

	return tx.QueryRowxContext(ctx, query,
		contact.ID, contact.ProfileID, contact.Type, contact.Value, contact.IsPrimary,
	).Scan(&contact.CreatedAt, &contact.UpdatedAt)
}

func (r *ProfileRepository) getAddresses(ctx context.Context, profileID uuid.UUID) ([]profile.Address, error) {
	query := `
		SELECT id, profile_id, street, city, state, country, postal_code, is_primary, created_at, updated_at
		FROM addresses
		WHERE profile_id = $1`

	var addresses []profile.Address
	if err := r.db.SelectContext(ctx, &addresses, query, profileID); err != nil {
		return nil, fmt.Errorf("error getting addresses: %w", err)
	}
	return addresses, nil
}

func (r *ProfileRepository) getContacts(ctx context.Context, profileID uuid.UUID) ([]profile.Contact, error) {
	query := `
		SELECT id, profile_id, type, value, is_primary, created_at, updated_at
		FROM contacts
		WHERE profile_id = $1`

	var contacts []profile.Contact
	if err := r.db.SelectContext(ctx, &contacts, query, profileID); err != nil {
		return nil, fmt.Errorf("error getting contacts: %w", err)
	}
	return contacts, nil
}

func (r *ProfileRepository) updateAddressesTx(ctx context.Context, tx *sqlx.Tx, profileID uuid.UUID, addresses []profile.Address) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM addresses WHERE profile_id = $1`, profileID); err != nil {
		return fmt.Errorf("error deleting addresses: %w", err)
	}
	for i := range addresses {
		addr := &addresses[i]
		addr.ProfileID = profileID
		if err := r.createAddressTx(ctx, tx, addr); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProfileRepository) updateContactsTx(ctx context.Context, tx *sqlx.Tx, profileID uuid.UUID, contacts []profile.Contact) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM contacts WHERE profile_id = $1`, profileID); err != nil {
		return fmt.Errorf("error deleting contacts: %w", err)
	}
	for i := range contacts {
		contact := &contacts[i]
		contact.ProfileID = profileID
		if err := r.createContactTx(ctx, tx, contact); err != nil {
			return err
		}
	}
	return nil
}

func getCorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value := ctx.Value("correlation_id")
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

// ErrNotFound is returned when a profile is not found.
var ErrNotFound = errors.New("profile not found")


