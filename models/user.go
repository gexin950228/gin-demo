package models

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `gorm:"uniqueIndex;size:64;not null"`
	Email    string `gorm:"uniqueIndex;size:128;not null"`
	Password string `gorm:"column:password;not null"`
}

var (
	DB               *gorm.DB
	ErrUserExists    = errors.New("user already exists")
	ErrUserNotFound  = errors.New("user not found")
	ErrInvalidPasswd = errors.New("invalid password")
)

func InitDB(db *gorm.DB) {
	DB = db
}

func CreateUser(username, email, password string) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	// check existing by username or email
	var u User
	if err := DB.Where("username = ? OR email = ?", username, email).First(&u).Error; err == nil {
		return ErrUserExists
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u = User{Username: username, Email: email, Password: string(hash)}
	if err := DB.Create(&u).Error; err != nil {
		return err
	}
	return nil
}

func Authenticate(username, password string) error {
	if DB == nil {
		return errors.New("database not initialized")
	}
	var u User
	if err := DB.Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidPasswd
	}
	return nil
}

// TableName returns the database table name for the User model.
func (User) TableName() string {
	return "users"
}
