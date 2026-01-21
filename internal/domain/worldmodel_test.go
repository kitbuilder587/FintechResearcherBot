package domain

import (
	"errors"
	"testing"
	"time"
)

func TestFact_Validate(t *testing.T) {
	tests := []struct {
		name    string
		fact    Fact
		wantErr error
	}{
		{
			name: "valid fact with URL",
			fact: Fact{
				ID:          "123",
				UserID:      1,
				Content:     "Market is $300B",
				SourceURL:   "https://example.com/report",
				Confidence:  0.9,
				ExtractedAt: time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "empty content",
			fact: Fact{
				ID:          "123",
				UserID:      1,
				Content:     "",
				SourceURL:   "https://example.com/report",
				Confidence:  0.9,
				ExtractedAt: time.Now(),
			},
			wantErr: ErrEmptyContent,
		},
		{
			name: "invalid URL",
			fact: Fact{
				ID:          "123",
				UserID:      1,
				Content:     "Some fact",
				SourceURL:   "not-url",
				Confidence:  0.9,
				ExtractedAt: time.Now(),
			},
			wantErr: ErrInvalidURL,
		},
		{
			name: "empty URL is valid (optional)",
			fact: Fact{
				ID:          "123",
				UserID:      1,
				Content:     "Some fact",
				SourceURL:   "",
				Confidence:  0.9,
				ExtractedAt: time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "valid http URL",
			fact: Fact{
				ID:          "123",
				UserID:      1,
				Content:     "Another fact",
				SourceURL:   "http://example.com",
				Confidence:  0.5,
				ExtractedAt: time.Now(),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fact.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Fact.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEntity_Validate(t *testing.T) {
	tests := []struct {
		name    string
		entity  Entity
		wantErr error
	}{
		{
			name: "valid company entity",
			entity: Entity{
				ID:          "123",
				UserID:      1,
				Name:        "Klarna",
				Type:        EntityCompany,
				Attributes:  map[string]string{"industry": "fintech"},
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "valid person entity",
			entity: Entity{
				ID:          "123",
				UserID:      1,
				Name:        "John Doe",
				Type:        EntityPerson,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "empty name",
			entity: Entity{
				ID:          "123",
				UserID:      1,
				Name:        "",
				Type:        EntityCompany,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
			wantErr: ErrEmptyEntityName,
		},
		{
			name: "invalid entity type",
			entity: Entity{
				ID:          "123",
				UserID:      1,
				Name:        "Klarna",
				Type:        EntityType("invalid"),
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
			wantErr: ErrInvalidEntityType,
		},
		{
			name: "valid concept entity",
			entity: Entity{
				ID:          "123",
				UserID:      1,
				Name:        "BNPL",
				Type:        EntityConcept,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "valid product entity",
			entity: Entity{
				ID:          "123",
				UserID:      1,
				Name:        "Klarna App",
				Type:        EntityProduct,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "valid market entity",
			entity: Entity{
				ID:          "123",
				UserID:      1,
				Name:        "BNPL Market",
				Type:        EntityMarket,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entity.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Entity.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEntityType_IsValid(t *testing.T) {
	tests := []struct {
		name       string
		entityType EntityType
		want       bool
	}{
		{
			name:       "company is valid",
			entityType: EntityCompany,
			want:       true,
		},
		{
			name:       "person is valid",
			entityType: EntityPerson,
			want:       true,
		},
		{
			name:       "concept is valid",
			entityType: EntityConcept,
			want:       true,
		},
		{
			name:       "product is valid",
			entityType: EntityProduct,
			want:       true,
		},
		{
			name:       "market is valid",
			entityType: EntityMarket,
			want:       true,
		},
		{
			name:       "unknown is invalid",
			entityType: EntityType("unknown"),
			want:       false,
		},
		{
			name:       "empty is invalid",
			entityType: EntityType(""),
			want:       false,
		},
		{
			name:       "random string is invalid",
			entityType: EntityType("invalid"),
			want:       false,
		},
		{
			name:       "uppercase COMPANY is invalid",
			entityType: EntityType("COMPANY"),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entityType.IsValid(); got != tt.want {
				t.Errorf("EntityType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntityType_String(t *testing.T) {
	tests := []struct {
		name       string
		entityType EntityType
		want       string
	}{
		{
			name:       "company",
			entityType: EntityCompany,
			want:       "company",
		},
		{
			name:       "person",
			entityType: EntityPerson,
			want:       "person",
		},
		{
			name:       "concept",
			entityType: EntityConcept,
			want:       "concept",
		},
		{
			name:       "product",
			entityType: EntityProduct,
			want:       "product",
		},
		{
			name:       "market",
			entityType: EntityMarket,
			want:       "market",
		},
		{
			name:       "empty",
			entityType: EntityType(""),
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entityType.String(); got != tt.want {
				t.Errorf("EntityType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResearchSession_Validate(t *testing.T) {
	tests := []struct {
		name    string
		session ResearchSession
		wantErr error
	}{
		{
			name: "valid session",
			session: ResearchSession{
				ID:        "123",
				UserID:    1,
				Question:  "What is BNPL?",
				Strategy:  "deep_research",
				CreatedAt: time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "empty question",
			session: ResearchSession{
				ID:        "123",
				UserID:    1,
				Question:  "",
				Strategy:  "deep_research",
				CreatedAt: time.Now(),
			},
			wantErr: ErrEmptyQuestion,
		},
		{
			name: "whitespace only question",
			session: ResearchSession{
				ID:        "123",
				UserID:    1,
				Question:  "   ",
				Strategy:  "deep_research",
				CreatedAt: time.Now(),
			},
			wantErr: ErrEmptyQuestion,
		},
		{
			name: "valid session without strategy",
			session: ResearchSession{
				ID:        "123",
				UserID:    1,
				Question:  "How does Klarna work?",
				Strategy:  "",
				CreatedAt: time.Now(),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ResearchSession.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
