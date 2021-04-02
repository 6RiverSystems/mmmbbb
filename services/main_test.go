package services

import (
	"os"
	"testing"

	"go.6river.tech/gosix/db"
	"go.6river.tech/mmmbbb/version"
)

func TestMain(m *testing.M) {
	db.SetDefaultDbName(version.AppName)
	os.Exit(m.Run())
}
