package sheetstocsv

import (
	"log"
	"testing"
	"time"
)

func TestStartDoFile(t *testing.T) {
	srv := GetService("./testdata/credentials.json", "./testdata")
	lastExecTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")

	log.Println("res: ", <-StartDoFile(srv, "1izKuUhRe9sSsLO6NP4ZoCNak1mkHOR1p", "./testdata/out/", lastExecTime, func(path string) {
	}))
}
