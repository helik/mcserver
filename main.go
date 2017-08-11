package main

import (
	"flag"

	"github.com/helik/mcserver/controller"
)

func main() {
	//endpoint, bucket, filePath

	endpoint := flag.String("endpoint", "", "minio endpoint")
	accessKeyID := flag.String("accesskey", "", "minio access key")
	secretAccessKey := flag.String("secretkey", "", "minio secret key")
	useSSL := flag.Bool("usessl", true, "use ssl with minio")
	bucket := flag.String("bucket", "", "minio bucket for desired world")
	objectName := flag.String("name", "", "minio object name for desired world")
	flag.Parse()

	controller.Run(*endpoint, *accessKeyID, *secretAccessKey, *bucket, *objectName, *useSSL)
}
