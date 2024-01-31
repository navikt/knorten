package main

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/local/dbsetup"
)

const (
	dbname = "knorten"
)

func main() {
	ctx := context.Background()
	dbURL := "postgres://postgres:postgres@localhost:5432"

	err := dbsetup.SetupDB(ctx, dbURL, dbname)
	if err != nil {
		panic(err)
	}

	fmt.Println("All good! Run `make local` to start testing.")
}
