package db_test

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/FlyTOmeLight/server-go/db"
)

func ExampleTransaction() {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	connection, err := database.DB()
	if err != nil {
		panic(err)
	}
	defer connection.Close()
	if err := database.Exec(
		"CREATE TABLE accounts (id INTEGER PRIMARY KEY, balance INTEGER NOT NULL)",
	).Error; err != nil {
		panic(err)
	}
	err = db.Transaction(
		context.Background(),
		database,
		db.TransactionOptions{Isolation: sql.LevelSerializable},
		func(tx *db.DB) error {
			return tx.Exec("INSERT INTO accounts (id, balance) VALUES (?, ?)", 1, 100).Error
		},
	)
	if err != nil {
		panic(err)
	}
	var balance int
	if err := database.Raw("SELECT balance FROM accounts WHERE id = ?", 1).Scan(&balance).Error; err != nil {
		panic(err)
	}
	fmt.Println(balance)
	// Output: 100
}
