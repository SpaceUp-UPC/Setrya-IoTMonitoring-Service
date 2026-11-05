package domain

import (
	"errors"
	"strings"
	"time"
)

type Status string

const (
	StatusActive Status = "active"
	StatusPaused Status = "paused"
)

type Location struct {
	Address  string `json:"address"`
	District string `json:"district"`
	City     string `json:"city"`
}

type Dimensions struct {
	Width  float64 `json:"width"`
	Length float64 `json:"length"`
}

func (d Dimensions) Area() float64 {
	return d.Width * d.Length
}

type Space struct {
	ID          string     `json:"id"`
	OwnerID     string     `json:"ownerId"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Location    Location   `json:"location"`
	Dimensions  Dimensions `json:"dimensions"`
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func NewSpace(
	id, ownerID, name, description string,
	loc Location,
	dim Dimensions,
) (Space, error) {
	if strings.TrimSpace(id) == "" ||
		strings.TrimSpace(ownerID) == "" ||
		strings.TrimSpace(name) == "" {
		return Space{}, errors.New("id, ownerId y name son requeridos")
	}

	if strings.TrimSpace(loc.Address) == "" ||
		strings.TrimSpace(loc.District) == "" ||
		strings.TrimSpace(loc.City) == "" {
		return Space{}, errors.New("address, district y city son requeridos")
	}

	if dim.Width <= 0 || dim.Length <= 0 {
		return Space{}, errors.New("width y length deben ser mayores que cero")
	}

	now := time.Now().UTC()
	return Space{
		ID:          id,
		OwnerID:     ownerID,
		Name:        name,
		Description: description,
		Location:    loc,
		Dimensions:  dim,
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *Space) Update(
	name, description string,
	loc Location,
	dim Dimensions,
) error {
	if strings.TrimSpace(name) == "" || dim.Width <= 0 || dim.Length <= 0 {
		return errors.New("nombre y dimensiones válidos son requeridos")
	}

	s.Name = name
	s.Description = description
	s.Location = loc
	s.Dimensions = dim
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *Space) Pause() {
	s.Status = StatusPaused
	s.UpdatedAt = time.Now().UTC()
}

func (s *Space) Publish() {
	s.Status = StatusActive
	s.UpdatedAt = time.Now().UTC()
}
