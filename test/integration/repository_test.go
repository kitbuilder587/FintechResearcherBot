package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	pgRepo "github.com/kitbuilder587/fintech-bot/internal/repository/postgres"
)

var testDB *pgRepo.DB

func TestMain(m *testing.M) {
	if os.Getenv("SHORT_TESTS") == "1" {
		os.Exit(0)
	}

	ctx := context.Background()

	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("test_db"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		panic(err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(err)
	}

	testDB, err = pgRepo.New(ctx, connStr)
	if err != nil {
		panic(err)
	}

	_, err = testDB.Pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS users (
            id BIGINT PRIMARY KEY,
            username TEXT,
            created_at TIMESTAMPTZ DEFAULT NOW()
        );
        CREATE TABLE IF NOT EXISTS sources (
            id BIGSERIAL PRIMARY KEY,
            user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            url TEXT NOT NULL,
            name TEXT NOT NULL,
            trust_level TEXT NOT NULL DEFAULT 'medium',
            is_user_added BOOLEAN NOT NULL DEFAULT false,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE(user_id, url)
        );
    `)
	if err != nil {
		panic(err)
	}

	code := m.Run()

	testDB.Close()
	pgContainer.Terminate(ctx)

	os.Exit(code)
}

func TestUserRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	repo := pgRepo.NewUserRepo(testDB)

	user, err := repo.GetOrCreate(ctx, 12345, "testuser")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if user.ID != 12345 {
		t.Errorf("user.ID = %v, want %v", user.ID, 12345)
	}

	user2, err := repo.GetOrCreate(ctx, 12345, "updatedname")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}
	if user2.Username != "updatedname" {
		t.Errorf("user.Username = %v, want %v", user2.Username, "updatedname")
	}

	found, err := repo.GetByID(ctx, 12345)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if found.ID != 12345 {
		t.Errorf("GetByID() user.ID = %v, want %v", found.ID, 12345)
	}

	_, err = repo.GetByID(ctx, 99999)
	if err != domain.ErrUserNotFound {
		t.Errorf("GetByID() error = %v, want ErrUserNotFound", err)
	}
}

func TestSourceRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	userRepo := pgRepo.NewUserRepo(testDB)
	sourceRepo := pgRepo.NewSourceRepo(testDB)

	user, _ := userRepo.GetOrCreate(ctx, 54321, "sourcetest")

	source := &domain.Source{
		UserID:      user.ID,
		URL:         "https://example.com",
		Name:        "Example",
		TrustLevel:  domain.TrustMedium,
		IsUserAdded: true,
	}
	err := sourceRepo.Create(ctx, source)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if source.ID == 0 {
		t.Error("Create() did not set source ID")
	}

	duplicate := &domain.Source{
		UserID:      user.ID,
		URL:         "https://example.com",
		Name:        "Example Duplicate",
		TrustLevel:  domain.TrustMedium,
		IsUserAdded: true,
	}
	err = sourceRepo.Create(ctx, duplicate)
	if err != domain.ErrDuplicateSource {
		t.Errorf("Create() duplicate error = %v, want ErrDuplicateSource", err)
	}

	sources, err := sourceRepo.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("ListByUser() got %d sources, want 1", len(sources))
	}

	count, err := sourceRepo.CountByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("CountByUser() error = %v", err)
	}
	if count != 1 {
		t.Errorf("CountByUser() = %d, want 1", count)
	}

	exists, err := sourceRepo.ExistsByURL(ctx, user.ID, "https://example.com")
	if err != nil {
		t.Fatalf("ExistsByURL() error = %v", err)
	}
	if !exists {
		t.Error("ExistsByURL() = false, want true")
	}

	found, err := sourceRepo.GetByID(ctx, source.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if found.URL != source.URL {
		t.Errorf("GetByID() URL = %v, want %v", found.URL, source.URL)
	}

	err = sourceRepo.Delete(ctx, user.ID, source.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	err = sourceRepo.Delete(ctx, user.ID, 99999)
	if err != domain.ErrSourceNotFound {
		t.Errorf("Delete() error = %v, want ErrSourceNotFound", err)
	}
}

func TestSourceRepository_GetDomainsByUserID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	userRepo := pgRepo.NewUserRepo(testDB)
	sourceRepo := pgRepo.NewSourceRepo(testDB)

	user, _ := userRepo.GetOrCreate(ctx, 99999, "domaintest")

	existingSources, _ := sourceRepo.ListByUser(ctx, user.ID)
	for _, s := range existingSources {
		sourceRepo.Delete(ctx, user.ID, s.ID)
	}

	domains, err := sourceRepo.GetDomainsByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetDomainsByUserID() error = %v", err)
	}
	if len(domains) != 0 {
		t.Errorf("GetDomainsByUserID() got %d domains for empty sources, want 0", len(domains))
	}

	testSources := []*domain.Source{
		{UserID: user.ID, URL: "https://example.com/page1", Name: "Example 1", TrustLevel: domain.TrustMedium},
		{UserID: user.ID, URL: "https://example.com/page2", Name: "Example 2", TrustLevel: domain.TrustMedium},
		{UserID: user.ID, URL: "http://test.org/article", Name: "Test", TrustLevel: domain.TrustMedium},
		{UserID: user.ID, URL: "https://sub.domain.ru/path", Name: "Subdomain", TrustLevel: domain.TrustMedium},
	}

	for _, s := range testSources {
		if err := sourceRepo.Create(ctx, s); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	domains, err = sourceRepo.GetDomainsByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetDomainsByUserID() error = %v", err)
	}

	if len(domains) != 3 {
		t.Errorf("GetDomainsByUserID() got %d domains, want 3; domains: %v", len(domains), domains)
	}

	domainSet := make(map[string]bool)
	for _, d := range domains {
		domainSet[d] = true
	}

	expectedDomains := []string{"example.com", "test.org", "sub.domain.ru"}
	for _, expected := range expectedDomains {
		if !domainSet[expected] {
			t.Errorf("GetDomainsByUserID() missing expected domain %q; got: %v", expected, domains)
		}
	}

	for _, s := range testSources {
		sourceRepo.Delete(ctx, user.ID, s.ID)
	}
}
