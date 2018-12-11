package sheetstocsv

import (
	"log"
	"testing"
	"time"
)

func TestStartDoFile(t *testing.T) {
	srv := GetService("./testdata/credentials.json", "./testdata")
	lastExecTime := time.Now().AddDate(0, 0, -1)

	log.Println("res: ", <-StartDoFile(srv, "1_8S1qcl_o1fAz3tBbK1t_T2tCRdFzyXA", "./testdata/out", lastExecTime, func(path string) {
	}))
}
