package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

func randomString() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return strconv.Itoa(r.Int())
}

func handleErrors(err error) {
	if err != nil {
		if serr, ok := err.(azblob.StorageError); ok {
			switch serr.ServiceCode() {
			case azblob.ServiceCodeContainerAlreadyExists:
				fmt.Println("received 409.  container already exists")
				return
			}
		}
		log.Fatal(err)
	}
}

func main() {
	fmt.Printf("azure blob storage quick start sample\n")
	//os.Getenv("")
	os.Setenv("EndpointSuffix", "core.chinacloudapi.cn")
	accountName, accountKey := "omcpsafscamceint", "KgObhqh5r0/dfioDDkl4U53vffqKR8V/8acF9dJu+DV+rxhWOXF6cdl2iPi/gRwCh7d/0b5iFBecPzFIHYbAmQ=="
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal("invalid credentials with error: ", err.Error())
	}
	//credentialNew:=azblob.NewTokenCredential("DefaultEndpointsProtocol=https;AccountName=recallstroage;AccountKey=NRwswm4KOEpVbkV6SIQuAzS4d50JP7eNOjwEIgTg/6Czc/X9ZUHGqLEayyKnC7dKI2fqfb7ru+vUz43MMrPXUA==;EndpointSuffix=core.windows.net",nil)

	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	containerName := fmt.Sprintf("container-name-%s", randomString())

	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.chinacloudapi.cn/%s", accountName, containerName))

	containerURL := azblob.NewContainerURL(*URL, p)
	fmt.Printf("creating a container named %s\n", containerName)
	ctx := context.Background()
	_, err = containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
	handleErrors(err)

	fmt.Printf("creating a dumy file to test the upload and download\n")
	data := []byte("hello world this is a blob\n")
	fileName := randomString()
	err = ioutil.WriteFile(fileName, data, 0700)
	handleErrors(err)

	blobURL := containerURL.NewBlockBlobURL(fileName)
	file, err := os.Open(fileName)
	handleErrors(err)

	fmt.Printf("uploading the file with blob name: %s\n", fileName)
	_, err = azblob.UploadFileToBlockBlob(ctx, file, blobURL, azblob.UploadToBlockBlobOptions{
		BlockSize:   4 * 1024 * 1024,
		Parallelism: 16,
	})
	handleErrors(err)

	fmt.Println("listen the blobs in the container:")
	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlob, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		handleErrors(err)
		marker = listBlob.NextMarker

		for _, blobInfo := range listBlob.Segment.BlobItems {
			fmt.Print("  Blob name: " + blobInfo.Name + "\n")
		}
	}

	downloadResponse, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	bodyStream := downloadResponse.Body(azblob.RetryReaderOptions{MaxRetryRequests: 20})

	downloadedData := bytes.Buffer{}
	_, err = downloadedData.ReadFrom(bodyStream)
	handleErrors(err)

	fmt.Printf("downloaded the blob : " + downloadedData.String())

	fmt.Printf("press enter key to delete the sample files , example container ,and exit the app\n")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	fmt.Printf("cleanding up.\n")
	containerURL.Delete(ctx, azblob.ContainerAccessConditions{})
	file.Close()
	os.Remove(fileName)
}
