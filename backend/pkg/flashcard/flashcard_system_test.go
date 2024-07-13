package flashcard

import (
	"backend/pkg/model"
	"backend/pkg/repository"
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "test",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
		),
	}
	postgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("could not start container: %v", err)
	}
	host, err := postgresContainer.Host(ctx)
	if err != nil {
		t.Fatalf("could not get container host: %v", err)
	}
	port, err := postgresContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("could not get container port: %v", err)
	}
	dbConfig := repository.DBConfig{
		Host:     host,
		Port:     port.Port(),
		User:     "test",
		Password: "test",
		DBName:   "test",
		SSLMode:  "disable",
	}
	pg := repository.NewPostgres(dbConfig)
	if err := pg.Open(); err != nil {
		t.Fatalf("could not connect to database: %v", err)
	}
	if err := MigrateDB(pg.DB); err != nil {
		t.Fatalf("could not migrate database: %v", err)
	}
	return pg.DB, func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate postgres container: %s", err)
		}
	}
}

func createTestCardGroup(t *testing.T, db *gorm.DB, name string) model.Cardgroup {
	t.Helper()
	cardGroup := model.Cardgroup{Name: name}
	if err := db.Create(&cardGroup).Error; err != nil {
		t.Fatalf("could not create test card group: %v", err)
	}
	return cardGroup
}

func createTestCard(t *testing.T, db *gorm.DB, front, back string, reviewDate time.Time, intervalDays int, cardGroupID int64) model.Card {
	t.Helper()
	card := model.Card{
		Front:        front,
		Back:         back,
		ReviewDate:   reviewDate,
		IntervalDays: intervalDays,
		CardGroupID:  cardGroupID,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("could not create test card: %v", err)
	}
	return card
}

func createTestUser(t *testing.T, db *gorm.DB, id, name string) model.User {
	t.Helper()
	parsedId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		fmt.Println(err)
	}

	user := model.User{ID: parsedId, Name: name}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("could not create test user: %v", err)
	}
	return user
}

func createTestRole(t *testing.T, db *gorm.DB, name string) model.Role {
	t.Helper()
	role := model.Role{Name: name}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("could not create test role: %v", err)
	}
	return role
}

func TestFlashcardFunctions(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	cardGroup := createTestCardGroup(t, db, "Test Group")

	t.Run("Test GetDueCards", func(t *testing.T) {
		pastDate := time.Now().AddDate(0, 0, -1)
		futureDate := time.Now().AddDate(0, 0, 1)
		createTestCard(t, db, "Past Due", "Test", pastDate, 1, cardGroup.ID)
		createTestCard(t, db, "Future Due", "Test", futureDate, 1, cardGroup.ID)

		dueCards, err := GetDueCards(db)
		assert.NoError(t, err)
		assert.Len(t, dueCards, 1)
		assert.Equal(t, "Past Due", dueCards[0].Front)
	})

	t.Run("Test UpdateCardReview", func(t *testing.T) {
		card := createTestCard(t, db, "Update Test", "Test", time.Now(), 1, cardGroup.ID)

		err := UpdateCardReview(&card, db)
		assert.NoError(t, err)

		var updatedCard model.Card
		db.First(&updatedCard, card.ID)
		assert.Equal(t, 2, updatedCard.IntervalDays)
		assert.True(t, updatedCard.ReviewDate.After(time.Now()))
	})

	t.Run("Test CreateCardReview", func(t *testing.T) {
		newCard := model.Card{
			Front:        "New Card",
			Back:         "Test",
			ReviewDate:   time.Now(),
			IntervalDays: 1,
			CardGroupID:  cardGroup.ID,
		}

		err := CreateCardReview(&newCard, db)
		assert.NoError(t, err)

		var createdCard model.Card
		result := db.First(&createdCard, newCard.ID)
		assert.NoError(t, result.Error)
		assert.Equal(t, "New Card", createdCard.Front)

		// If the above assertion fails, let's print more information
		if createdCard.Front != "New Card" {
			t.Logf("Failed to retrieve the created card. ID: %d, Front: %s", newCard.ID, createdCard.Front)

			// Let's try to fetch all cards to see what's in the database
			var allCards []model.Card
			db.Find(&allCards)
			t.Logf("All cards in database: %+v", allCards)
		}
	})

	t.Run("Test MigrateDB", func(t *testing.T) {
		err := MigrateDB(db)
		assert.NoError(t, err)
	})

	t.Run("Test Card Model", func(t *testing.T) {
		card := createTestCard(t, db, "Front", "Back", time.Now(), 1, cardGroup.ID)

		assert.NotZero(t, card.ID)
		assert.Equal(t, "Front", card.Front)
		assert.Equal(t, "Back", card.Back)
		assert.NotZero(t, card.Created)
		assert.NotZero(t, card.Updated)
		assert.Equal(t, cardGroup.ID, card.CardGroupID)

		// Test autoCreateTime and autoUpdateTime
		createdTime := card.Created
		time.Sleep(time.Second)
		db.Model(&card).Update("Front", "Updated Front")
		db.First(&card, card.ID)
		assert.WithinDuration(t, createdTime, card.Created, time.Second)
		assert.True(t, card.Updated.After(createdTime))
	})

	t.Run("Test Cardgroup Model", func(t *testing.T) {
		assert.NotZero(t, cardGroup.ID)
		assert.Equal(t, "Test Group", cardGroup.Name)
		assert.NotZero(t, cardGroup.Created)
		assert.NotZero(t, cardGroup.Updated)
	})

	t.Run("Test User Model", func(t *testing.T) {
		user := createTestUser(t, db, "user1", "Test User")
		assert.Equal(t, "user1", user.ID)
		assert.Equal(t, "Test User", user.Name)
		assert.NotZero(t, user.Created)
		assert.NotZero(t, user.Updated)
	})

	t.Run("Test Role Model", func(t *testing.T) {
		role := createTestRole(t, db, "Admin")
		assert.NotZero(t, role.ID)
		assert.Equal(t, "Admin", role.Name)
	})

	t.Run("Test Relationships", func(t *testing.T) {
		cardGroup := createTestCardGroup(t, db, "Relationship Group")
		card1 := createTestCard(t, db, "Card 1", "Back 1", time.Now(), 1, cardGroup.ID)
		card2 := createTestCard(t, db, "Card 2", "Back 2", time.Now(), 1, cardGroup.ID)
		user := createTestUser(t, db, "user2", "Relationship User")
		role := createTestRole(t, db, "User")

		// Test Cardgroup - Card relationship
		var fetchedCardGroup model.Cardgroup
		db.Preload("Cards").First(&fetchedCardGroup, cardGroup.ID)
		assert.Len(t, fetchedCardGroup.Cards, 2)
		assert.Equal(t, card1.ID, fetchedCardGroup.Cards[0].ID)
		assert.Equal(t, card2.ID, fetchedCardGroup.Cards[1].ID)

		// Test User - Cardgroup relationship
		db.Model(&user).Association("CardGroups").Append(&cardGroup)
		var fetchedUser model.User
		db.Preload("CardGroups").First(&fetchedUser, "id = ?", user.ID)
		assert.Len(t, fetchedUser.CardGroups, 1)
		assert.Equal(t, cardGroup.ID, fetchedUser.CardGroups[0].ID)

		// Test User - Role relationship
		db.Model(&user).Association("Roles").Append(&role)
		db.Preload("Roles").First(&fetchedUser, "id = ?", user.ID)
		assert.Len(t, fetchedUser.Roles, 1)
		assert.Equal(t, role.ID, fetchedUser.Roles[0].ID)

		// Clean up user-cardgroup relationships to allow deletion of cardgroup
		db.Model(&user).Association("CardGroups").Delete(&cardGroup)

		// Test deletion cascade
		db.Delete(&cardGroup)
		var deletedCards []model.Card
		db.Where("cardgroup_id = ?", cardGroup.ID).Find(&deletedCards)
		assert.Len(t, deletedCards, 0)
	})
}
