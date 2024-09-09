package db

import (
	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewWithMock() (*DB, sqlmock.Sqlmock, error) {
	mockDb, mock, _ := sqlmock.New()
	dialector := postgres.New(postgres.Config{
		Conn:       mockDb,
		DriverName: "postgres",
	})

	g, _ := gorm.Open(dialector, &gorm.Config{})
	return &DB{g: g}, mock, nil
}
