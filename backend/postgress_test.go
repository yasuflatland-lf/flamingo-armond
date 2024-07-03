package backend

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type User struct {
	ID    uint
	Name  string
	Email string
}

func setupTestContainer() (testcontainers.Container, *DBConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpassword",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp"),
		),
	}

	postgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, err
	}

	host, err := postgresContainer.Host(ctx)
	if err != nil {
		return nil, nil, err
	}

	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, nil, err
	}

	config := &DBConfig{
		Host:     host,
		User:     "testuser",
		Password: "testpassword",
		DBName:   "testdb",
		Port:     port.Port(),
		SSLMode:  "disable",
	}

	return postgresContainer, config, nil
}

func TestPostgress(t *testing.T) {
	ctx := context.Background()

	container, config, err := setupTestContainer()
	if err != nil {
		t.Fatalf("Failed to setup test container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate container: %v", err)
		}
	}()

	pg := NewPostgress(*config)
	if err := pg.Open(); err != nil {
		t.Fatalf("Could not open database: %v", err)
	}

	if err := pg.runGooseMigrations(); err != nil {
		t.Fatalf("Goose migration failed: %v", err)
	}

	t.Run("TestCountUsersInitiallyZero", func(t *testing.T) {
		var userCount int64
		if err := pg.DB.Model(&User{}).Count(&userCount).Error; err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}

		if userCount != 0 {
			t.Fatalf("Expected 0 users, got %d", userCount)
		}
	})

	t.Run("TestCreateUser", func(t *testing.T) {
		newUser := User{Name: "John Doe", Email: "john.doe@example.com"}
		if err := pg.DB.Create(&newUser).Error; err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		var userCount int64
		if err := pg.DB.Model(&User{}).Count(&userCount).Error; err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}

		if userCount != 1 {
			t.Fatalf("Expected 1 user, got %d", userCount)
		}
	})

	t.Run("TestUpdateUser", func(t *testing.T) {
		var user User
		if err := pg.DB.First(&user, "email = ?", "john.doe@example.com").Error; err != nil {
			t.Fatalf("Failed to find user: %v", err)
		}

		user.Name = "John Doe Updated"
		if err := pg.DB.Save(&user).Error; err != nil {
			t.Fatalf("Failed to update user: %v", err)
		}

		var updatedUser User
		if err := pg.DB.First(&updatedUser, user.ID).Error; err != nil {
			t.Fatalf("Failed to find updated user: %v", err)
		}

		if updatedUser.Name != "John Doe Updated" {
			t.Fatalf("Expected user name to be 'John Doe Updated', got '%s'", updatedUser.Name)
		}
	})

	t.Run("TestDeleteUser", func(t *testing.T) {
		var user User
		if err := pg.DB.First(&user, "email = ?", "john.doe@example.com").Error; err != nil {
			t.Fatalf("Failed to find user: %v", err)
		}

		if err := pg.DB.Delete(&user).Error; err != nil {
			t.Fatalf("Failed to delete user: %v", err)
		}

		var userCount int64
		if err := pg.DB.Model(&User{}).Count(&userCount).Error; err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}

		if userCount != 0 {
			t.Fatalf("Expected 0 users, got %d", userCount)
		}
	})
}
