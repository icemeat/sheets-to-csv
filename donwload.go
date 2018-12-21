package sheetstocsv

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/api/drive/v3"
)

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

func perFile(srv *drive.Service, i *drive.File, path string, dateOffset time.Time, afterCreate func(string), worker FileWorker) {
	<-worker.job
	startTime := time.Now()
	var err error
	if i.MimeType == "application/vnd.google-apps.folder" {
		go func() {
			apiRateHolder <- true
			go func() {
				time.Sleep(1 * time.Second)
				<-apiRateHolder
			}()
			err := <-StartDoFile(srv, i.Id, path+"/"+i.Name, dateOffset, afterCreate)
			worker.res <- err
			fmt.Printf("End(Folder, %s): %s/%s (%s)\n", time.Now().Sub(startTime), path, i.Name, i.Id)
		}()
	} else {
		t, e := time.Parse(DriveTimeFormat, i.ModifiedTime)
		if e != nil || t.After(dateOffset) {
			if e != nil {
				log.Fatalln("fail to parse time: ", e)
			}
			apiRateHolder <- true
			exportCall := srv.Files.Export(i.Id, "text/csv")
			resp, exportErr := exportCall.Download()
			go func() {
				time.Sleep(1 * time.Second)
				<-apiRateHolder
			}()
			if exportErr != nil {
				log.Fatalln("Export Error: ", exportErr)
			} else {
				fileWorker := NewWorker(1)
				fileWorker.job <- new(interface{})
				go FileHandleWork(resp, path+"/"+i.Name, afterCreate, fileWorker)
				e := <-fileWorker.res
				if e != nil {
					log.Println("Err to Download File: ", e, path+"/"+i.Name)
				}
				fmt.Printf("End(File, %s): %s/%s (lastModify:%s)(%s)\n", time.Now().Sub(startTime), path, i.Name, t, i.Id)
			}
			worker.res <- err
		} else {
			fmt.Printf("Skip(File, %s): %s/%s (lastModify:%s)(%s)\n", time.Now().Sub(startTime), path, i.Name, t, i.Id)
			worker.res <- err
		}
	}
}
func StartDoFile(srv *drive.Service, id string, path string, dateOffset time.Time, afterCreate func(string)) <-chan error {
	worker := NewWorker(1)
	worker.job <- new(interface{})
	go DoFiles(srv, id, path, dateOffset, afterCreate, worker)

	return worker.res
}
func DownloadFile(srv *drive.Service, id string, saveFullPath string, afterCreate func(string)) {
	exportCall := srv.Files.Export(id, "text/csv")
	resp, exportErr := exportCall.Download()
	starttime := time.Now()
	apiRateHolder <- true
	go func() {
		time.Sleep(1 * time.Second)
		log.Println(time.Now().Sub(starttime), <-apiRateHolder)
	}()
	if exportErr != nil {
		log.Fatalln("Export Error: ", exportErr)
	} else {
		fileWorker := NewWorker(1)
		fileWorker.job <- new(interface{})
		go FileHandleWork(resp, saveFullPath, afterCreate, fileWorker)
		e := <-fileWorker.res
		if e != nil {
			log.Println("Err to Download File: ", e, saveFullPath)
		}
	}
}
func DoFiles(srv *drive.Service, id string, path string, dateOffset time.Time, afterCreate func(string), worker FileWorker) {
	<-worker.job
	call := srv.Files.List().
		Q(fmt.Sprintf("'%s' in parents and trashed=false ", id)).
		PageSize(1000).
		Fields("nextPageToken, files(id, name, mimeType, modifiedTime)")

	r, err := call.Do()
	if err != nil {
		log.Fatalf("Unable to retrieve files: %v", err)
	}
	fileCount := len(r.Files)
	offset := 0
	if fileCount == 0 {
		fmt.Println("No files found.")
	} else {
		for fileCount > 0 {
			const WorkerMax = 5
			workerCount := 0
			if fileCount-WorkerMax > 0 {
				workerCount = WorkerMax
			} else {
				workerCount = fileCount
			}
			fileCount -= workerCount
			coWorker := NewWorker(workerCount)
			for i := 0; i < workerCount; i++ {
				coWorker.job <- new(interface{})
			}
			for i := 0; i < workerCount; i++ {
				go perFile(srv, r.Files[offset+i], path, dateOffset, afterCreate, coWorker)
			}
			offset += workerCount
			for i := 0; i < workerCount; i++ {
				err := <-coWorker.res
				if err != nil {
					log.Fatalln("perFile Err: ", err)
				}
			}
		}
	}
	worker.res <- err
}
