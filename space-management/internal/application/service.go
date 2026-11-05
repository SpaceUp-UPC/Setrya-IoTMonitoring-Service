package application

import (
	"context"
	"errors"

	"github.com/SpaceUp-UPC/Setrya-IoTMonitoring-Service/space-management/internal/domain"
)

type Repository interface {
	Save(context.Context, domain.Space) error
	Find(context.Context, string) (domain.Space, error)
	ByOwner(context.Context, string) ([]domain.Space, error)
}

var ErrNotFound = errors.New("space not found")

type Service struct {
	Repo Repository
}

func (s Service) Create(ctx context.Context, space domain.Space) error {
	return s.Repo.Save(ctx, space)
}

func (s Service) Get(ctx context.Context, id string) (domain.Space, error) {
	return s.Repo.Find(ctx, id)
}

func (s Service) List(ctx context.Context, ownerID string) ([]domain.Space, error) {
	return s.Repo.ByOwner(ctx, ownerID)
}

func (s Service) Update(
	ctx context.Context,
	id, name, description string,
	location domain.Location,
	dimensions domain.Dimensions,
) error {
	space, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := space.Update(name, description, location, dimensions); err != nil {
		return err
	}

	return s.Repo.Save(ctx, space)
}

func (s Service) SetStatus(ctx context.Context, id string, active bool) error {
	space, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if active {
		space.Publish()
	} else {
		space.Pause()
	}

	return s.Repo.Save(ctx, space)
}
