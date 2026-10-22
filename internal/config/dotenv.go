package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

func LoadDotEnv() error {
	err := godotenv.Load(".env")
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return errors.New("could not load .env: check file permissions and syntax")
}
