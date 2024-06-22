// internal/database/database.go
package database

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

var DB *gorm.DB

func Connect(url string) error {
	var err error
	DB, err = gorm.Open("postgres", url)
	if err != nil {
		return err
	}
	return nil
}

func Migrate(models ...interface{}) error {
	return DB.AutoMigrate(models...).Error
}
