package user

import (
	"log"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/nais/knorten/local/dbsetup"
	"github.com/nais/knorten/pkg/database"
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
	var err error
	repo, err = dbsetup.SetupDBForTests()
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()
	os.Exit(code)
}
