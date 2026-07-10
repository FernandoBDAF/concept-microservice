package profile

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type Cache interface {
	GetProfile(ctx context.Context, id uuid.UUID) (*Profile, error)
	SetProfile(ctx context.Context, profile *Profile) error
	InvalidateProfile(ctx context.Context, id uuid.UUID) error
	GetProfileList(ctx context.Context, page int) ([]*Profile, error)
	SetProfileList(ctx context.Context, page int, profiles []*Profile) error
}

type Service struct {
	repo      Repository
	cache     Cache
	cacheMiss error
}

func NewService(repo Repository, cache Cache, cacheMiss error) *Service {
	return &Service{repo: repo, cache: cache, cacheMiss: cacheMiss}
}

func (s *Service) Create(ctx context.Context, req *ProfileRequest) (*Profile, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	p := &Profile{
		ID:        uuid.New(),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
	}

	for _, addr := range req.Addresses {
		p.Addresses = append(p.Addresses, Address{
			ID:         uuid.New(),
			Street:     addr.Street,
			City:       addr.City,
			State:      addr.State,
			Country:    addr.Country,
			PostalCode: addr.PostalCode,
			IsPrimary:  addr.IsPrimary,
		})
	}
	for _, contact := range req.Contacts {
		p.Contacts = append(p.Contacts, Contact{
			ID:        uuid.New(),
			Type:      contact.Type,
			Value:     contact.Value,
			IsPrimary: contact.IsPrimary,
		})
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.SetProfile(ctx, p)
	}

	return p, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Profile, error) {
	if s.cache != nil {
		if cached, err := s.cache.GetProfile(ctx, id); err == nil {
			return cached, nil
		} else if !errors.Is(err, s.cacheMiss) {
			// ignore cache errors
		}
	}

	p, err := s.repo.GetByID(ctx, id)
	if err != nil || p == nil {
		return p, err
	}

	if s.cache != nil {
		_ = s.cache.SetProfile(ctx, p)
	}

	return p, nil
}

func (s *Service) List(ctx context.Context, page, pageSize int) ([]*Profile, error) {
	if s.cache != nil {
		if cached, err := s.cache.GetProfileList(ctx, page); err == nil {
			return cached, nil
		} else if !errors.Is(err, s.cacheMiss) {
			// ignore cache errors
		}
	}

	profiles, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.SetProfileList(ctx, page, profiles)
	}

	return profiles, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req *ProfileRequest) (*Profile, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	p := &Profile{
		ID:        id,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
	}

	for _, addr := range req.Addresses {
		p.Addresses = append(p.Addresses, Address{
			ID:         uuid.New(),
			Street:     addr.Street,
			City:       addr.City,
			State:      addr.State,
			Country:    addr.Country,
			PostalCode: addr.PostalCode,
			IsPrimary:  addr.IsPrimary,
		})
	}
	for _, contact := range req.Contacts {
		p.Contacts = append(p.Contacts, Contact{
			ID:        uuid.New(),
			Type:      contact.Type,
			Value:     contact.Value,
			IsPrimary: contact.IsPrimary,
		})
	}

	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.SetProfile(ctx, p)
	}

	return p, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if s.cache != nil {
		_ = s.cache.InvalidateProfile(ctx, id)
	}
	return nil
}

