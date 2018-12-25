package sheetstocsv

import (
	"log"
	"testing"
	"time"

	drive "google.golang.org/api/drive/v3"
)

func TestStartDoFile(t *testing.T) {
	srv := GetService("./testdata/credentials.json", "./testdata")
	lastExecTime := time.Now().AddDate(0, 0, -6)

	log.Println("res: ", <-StartDoFile(srv, "1_8S1qcl_o1fAz3tBbK1t_T2tCRdFzyXA", "./testdata/out", lastExecTime, func(path string) {
	}))
}

func TestStartUpdateFile(t *testing.T) {
	srv := GetService("./testdata/credentials.json", "./testdata")
	log.Println("res: ", <-StartUpdateFile(srv, "1K8RqvwqsAgOoBGzVfA71F6FvdnTZcO7K", "18Vk6tyth8P9Fb8XxKFw2TGB-_hJatMy0"))
}

func TestCreateFile(t *testing.T) {
	srv := GetService("./testdata/credentials.json", "./testdata")
	f := &drive.File{
		Name:        "uploadtest.csv",
		Parents:     []string{"1Sn1_pMTtuPM6p8nlmhM1y0Vgq6j9pGwq"},
		Description: "test",
		MimeType:    "application/vnd.google-apps.folder"}

	_, err2 := srv.Files.Create(f).Do()
	if err2 != nil {
		log.Fatalf("Upload Failed %v", err2)
	}
}

func TestCopyFile(t *testing.T) {

	srv := GetService("./testdata/credentials.json", "./testdata")
	ff, err1 := srv.Files.Get("1WEespEQ2oXQRTwQwXKmei_8Rvh0yFNVl").Do()
	if err1 != nil {
		log.Fatalf("Upload Failed 1 %v", err1)
	}
	f := &drive.File{
		Name:         ff.Name + "_copied",
		Parents:      []string{"1Sn1_pMTtuPM6p8nlmhM1y0Vgq6j9pGwq"},
		Description:  "test",
		MimeType:     "application/vnd.google-apps.folder",
		ModifiedTime: ff.ModifiedTime,
	}

	_, err2 := srv.Files.Copy("1WEespEQ2oXQRTwQwXKmei_8Rvh0yFNVl", f).Do()
	if err2 != nil {
		log.Fatalf("Upload Failed 2 %v", err2)
	}
}

func TestUpdateFile(t *testing.T) {

	srv := GetService("./testdata/credentials.json", "./testdata")
	ff, err1 := srv.Files.Get("1Ak-rLBykxoTfsBlKsVeHmq9S830PFAD99hq--WVeyrw").Do()
	if err1 != nil {
		log.Fatalf("Upload Failed 1 %v", err1)
	}
	ff.Id = "1kciEnSQqRIKGeMbxF0OSUxHXuRBpU406w-LPRT6OkFs"

	_, err2 := srv.Files.Update("1kciEnSQqRIKGeMbxF0OSUxHXuRBpU406w-LPRT6OkFs", ff).Do()
	if err2 != nil {
		log.Fatalf("Upload Failed 2 %v", err2)
	}
}
