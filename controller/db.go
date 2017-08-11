package controller

import (
	"errors"
	minio "github.com/minio/minio-go"
	"io"
	"log"
)

const (
	contentType = "application/gzip"
)

var (
	errBackupDoesNotExist = errors.New("This backup does not exist in minio yet")
)

func (c *controller) storeBackup(backupLocation string) error {
	if err := c.checkOrCreateBucket(); err != nil {
		return err
	}

	if _, err := c.minioClient.FPutObject(c.bucket, c.objectName, backupLocation, contentType); err != nil {
		return err
	}

	log.Println("Successfully uploaded backup to minio", backupLocation)

	return nil
}

func (c *controller) getBackup() (io.ReadCloser, error) {
	if err := c.checkOrCreateBucket(); err != nil {
		return nil, err
	}

	object, err := c.minioClient.GetObject(c.bucket, c.objectName)
	if err != nil {
		return nil, err
	}

	if _, err = object.Stat(); err != nil {
		if err.(minio.ErrorResponse).Code == "NoSuchKey" {
			return nil, errBackupDoesNotExist
		}
		return nil, err
	}

	return object, nil
}

func (c *controller) checkOrCreateBucket() error {
	exists, err := c.minioClient.BucketExists(c.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := c.minioClient.MakeBucket(c.bucket, ""); err != nil {
			return err
		}
	}
	return nil
}
