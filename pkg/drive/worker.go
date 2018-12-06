package drive

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type FileWorker struct {
	job chan interface{}
	res chan error
}

func FileHandleWork(resp *http.Response, path string, afterCreate func(string), worker FileWorker) {
	<-worker.job
	defer resp.Body.Close()
	dirPath := filepath.Dir(path)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		e := os.MkdirAll(dirPath, 0777)
		if e != nil {
			log.Fatalln("mkDirErr : ", e)
		}
	}
	out, err := os.Create(path)
	if err != nil {
		log.Fatalln(err)
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	afterCreate(path)
	worker.res <- err
}

func NewWorkerPerAll(jobCount int, resCount int) FileWorker {
	return FileWorker{
		job: make(chan interface{}, jobCount),
		res: make(chan error, resCount),
	}
}
func NewWorker(jobCount int) FileWorker {
	return FileWorker{
		job: make(chan interface{}, jobCount),
		res: make(chan error, jobCount),
	}
}

func (w *FileWorker) WaitAll(fn func(error)) {
	for res := range w.res {
		fn(res)
	}
}
