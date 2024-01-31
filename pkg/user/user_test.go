package user

import (
	"log"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/navikt/knorten/local/dbsetup"
	"github.com/navikt/knorten/pkg/database"
	"github.com/sirupsen/logrus"
)

var repo *database.Repo

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "../..")
	err := os.Chdir(dir)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	dbConn, err := dbsetup.SetupDBForTests()
	if err != nil {
		log.Fatal(err)
	}
	repo, err = database.New(dbConn, "", logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	os.Exit(code)
}
