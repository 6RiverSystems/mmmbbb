package services

import (
	"os"
	"testing"

	"go.6river.tech/gosix/db"
)

func TestMain(m *testing.M) {
	db.SetDefaultDbName("mmmbbb")
	os.Exit(m.Run())
}
