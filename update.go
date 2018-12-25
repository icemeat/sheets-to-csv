package sheetstocsv

import (
	"fmt"
	"log"
	"time"

	"google.golang.org/api/drive/v3"
)

func getLazyLogger() (signal chan string) {
	signal = make(chan string)
	startTime := time.Now()
	go func() {
		select {
		case msg := <-signal:
			log.Println(time.Now().Sub(startTime), "::", msg)
		}
	}()
	return
}
func perFileCopy(srv *drive.Service, baseRootID string, bases []*drive.File, tgt *drive.File, worker FileWorker) {
	<-worker.job
	var err error
	exist := false
	for _, base := range bases {
		if tgt.Name == base.Name {
			exist = true
			if base.MimeType == "application/vnd.google-apps.folder" {
				err := <-StartUpdateFile(srv, base.Id, tgt.Id)
				worker.res <- err
			} else {
				var baseTime time.Time
				var targetTime time.Time
				var e error
				// baseTime, e = time.Parse(DriveTimeFormat, base.Description)
				// if e != nil {
				// }
				// targetTime, e = time.Parse(DriveTimeFormat, tgt.Description)
				// if e != nil {
				// }
				baseTime, e = time.Parse(DriveTimeFormat, base.ModifiedTime)
				targetTime, e = time.Parse(DriveTimeFormat, tgt.ModifiedTime)
				if e != nil || targetTime.After(baseTime) {
					if e != nil {
						log.Fatalln("fail to parse time: ", e)
					}
					l := getLazyLogger()
					waitAPIRate()
					exportCall := srv.Files.Export(tgt.Id, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
					resp, err2 := exportCall.Download()
					if err2 != nil {
						log.Fatalln("Fail to Copy File", err2)
					}
					l <- fmt.Sprintf("Download For Copy %s (%s)", tgt.Name, tgt.Id)
					l = getLazyLogger()
					waitAPIRate()
					f := drive.File{
						Name: base.Name,
						//Description:  fmt.Sprint(baseTime),
						//CreatedTime:  tgt.CreatedTime,
						ModifiedTime: tgt.ModifiedTime,
					}
					r, err3 := srv.Files.Update(base.Id, &f).Media(resp.Body).Do()
					if err3 != nil {
						log.Fatalln("Fail to Update File", err3)
					}
					l <- fmt.Sprintf("Update FileTo New %s (%s)", r.Name, r.Id)
					worker.res <- err
				} else {
					//log.Printf("Skip: %s (lastModify:%s)(%s)\n", base.Name, base.ModifiedTime, base.Id)
					worker.res <- err
				}
			}
		}
	}
	if exist == false {
		fForCreate := drive.File{
			Name:     tgt.Name,
			Parents:  []string{baseRootID},
			MimeType: tgt.MimeType,
			//Description:  fmt.Sprint(tgt.ModifiedTime),
			CreatedTime:  tgt.CreatedTime,
			ModifiedTime: tgt.ModifiedTime,
		}
		if fForCreate.MimeType == "application/vnd.google-apps.folder" {
			l := getLazyLogger()
			waitAPIRate()
			newDir, err2 := srv.Files.Create(&fForCreate).Do()
			if err2 != nil {
				log.Fatalln("Failt to Create Directory", err2)
			}
			l <- fmt.Sprintf("CreateNewFolder: %s (%s)", newDir.Name, newDir.Id)
			<-StartUpdateFile(srv, newDir.Id, tgt.Id)
			worker.res <- err

		} else {
			l := getLazyLogger()
			waitAPIRate()
			res, err2 := srv.Files.Copy(tgt.Id, &fForCreate).Do()
			if err2 != nil {
				log.Fatalln("Fail to CopyCreate File", err2)
			}
			l <- fmt.Sprintf("CreateCopy: %s (%s)", res.Name, res.Id)
			l = getLazyLogger()
			//CopyがModifiedTime矯正的に変えてしまうっぽい…
			waitAPIRate()
			fForTouch := drive.File{
				ModifiedTime: tgt.ModifiedTime,
			}
			_, err3 := srv.Files.Update(res.Id, &fForTouch).Do()
			if err3 != nil {
				log.Fatalln("Fail to Update FileStatus File", err3)
			}
			l <- fmt.Sprintf("UpdateFileStatus(ModifiedTime): %s (%s)", res.Name, res.Id)
			worker.res <- err
		}
	}
}
func StartUpdateFile(srv *drive.Service, baseRootID string, targetRootID string) <-chan error {
	worker := NewWorker(1)
	worker.job <- new(interface{})
	go UpdateFiles(srv, baseRootID, targetRootID, worker)

	return worker.res
}
func UpdateFiles(srv *drive.Service, baseRootID string, targetRootID string, worker FileWorker) {
	<-worker.job
	l := getLazyLogger()
	waitAPIRate()
	call := srv.Files.List().
		Q(fmt.Sprintf("'%s' in parents and trashed=false ", baseRootID)).
		PageSize(1000).
		Fields("nextPageToken, files(id, name, mimeType, createdTime, modifiedTime)")

	baseList, err := call.Do()
	if err != nil {
		log.Fatalf("Unable to retrieve files: %v", err)
	}
	l <- fmt.Sprintf("SearchFiles(Base): (%s)", baseRootID)
	l = getLazyLogger()
	waitAPIRate()
	call = srv.Files.List().
		Q(fmt.Sprintf("'%s' in parents and trashed=false ", targetRootID)).
		PageSize(1000).
		Fields("nextPageToken, files(id, name, mimeType, createdTime, modifiedTime)")

	targetList, err := call.Do()
	if err != nil {
		log.Fatalf("Unable to retrieve files: %v", err)
	}
	l <- fmt.Sprintf("SearchFiles(Target): (%s)", targetRootID)
	fileCount := len(targetList.Files)
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
				go perFileCopy(srv, baseRootID, baseList.Files, targetList.Files[offset+i], coWorker)
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
