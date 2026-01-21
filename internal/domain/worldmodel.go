package domain

import (
	"net/url"
	"strings"
	"time"
)

// Fact - факт извлеченный из исследования
type Fact struct {
	ID          string
	UserID      int64
	Content     string
	SourceURL   string // может быть пустым
	Confidence  float64
	ExtractedAt time.Time
}

func (f *Fact) Validate() error {
	if strings.TrimSpace(f.Content) == "" {
		return ErrEmptyContent
	}
	if f.SourceURL != "" {
		if _, err := url.ParseRequestURI(f.SourceURL); err != nil {
			return ErrInvalidURL
		}
	}
	return nil
}

// Entity - сущность (компания, человек, концепт и т.д.)
type Entity struct {
	ID          string
	UserID      int64
	Name        string
	Type        EntityType
	Attributes  map[string]string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

func (e *Entity) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return ErrEmptyEntityName
	}
	if !e.Type.IsValid() {
		return ErrInvalidEntityType
	}
	return nil
}

type EntityType string

const (
	EntityCompany EntityType = "company"
	EntityPerson  EntityType = "person"
	EntityConcept EntityType = "concept"
	EntityProduct EntityType = "product"
	EntityMarket  EntityType = "market"
)

func (t EntityType) IsValid() bool {
	switch t {
	case EntityCompany, EntityPerson, EntityConcept, EntityProduct, EntityMarket:
		return true
	}
	return false
}

func (t EntityType) String() string { return string(t) }

// ResearchSession - сессия исследования
type ResearchSession struct {
	ID        string
	UserID    int64
	Question  string
	Strategy  string
	CreatedAt time.Time
}

func (s *ResearchSession) Validate() error {
	if strings.TrimSpace(s.Question) == "" {
		return ErrEmptyQuestion
	}
	return nil
}
