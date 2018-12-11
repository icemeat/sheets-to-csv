package sheetstocsv

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"context"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

const (
	DriveTimeFormat = time.RFC3339
)

var (
	apiRateHolder = make(chan bool, 1)
)

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config, tokenSavePath string) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := tokenSavePath + "/token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		log.Println("get Token Err: ", err)
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
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

//GetService credentailFilePathからグーグルドライブの
func GetService(credentialFilePath string, tokenSavePath string) *drive.Service {
	b, err := ioutil.ReadFile(credentialFilePath)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b,
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/drive.file")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config, tokenSavePath)

	srv, err := drive.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Drive client: %v", err)
	}
	log.Println("basePath: " + srv.BasePath)
	return srv
}
