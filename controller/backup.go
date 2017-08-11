package controller

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func (c *controller) createBackup(retries int) {
	if retries == 0 {
		log.Println("Could not create backup")
		return
	}

	backupLocation := "backup.tar.gz"
	err := tarGzipSave(backupLocation)
	if err != nil {
		// if something happened, try again
		log.Println("Something went wrong with backup, trying again...")
		c.createBackup(retries - 1)
		return
	}

	err = c.storeBackup(backupLocation)
	if err != nil {
		// if something happened, try again
		log.Println("Something went wrong with backup, trying again...")
		c.createBackup(retries - 1)
		return
	}

	log.Println("Created backup")
}

func tarGzipSave(backupLocation string) error {
	files, err := ioutil.ReadDir(".")
	if err != nil {
		return err
	}

	backup, err := os.Create(backupLocation)
	if err != nil {
		return err
	}
	defer backup.Close()

	gzw := gzip.NewWriter(backup)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") || file.Name() == "world" || file.Name() == "server.properties" {
			if err := saveFile(tw, file.Name(), file.IsDir()); err != nil {
				return err
			}
		}
	}

	return nil
}

func saveFile(tw *tar.Writer, fileName string, isDir bool) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(fileInfo, fileName)
	if err != nil {
		return err
	}
	header.Name = fileName

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if isDir {
		files, err := ioutil.ReadDir(fileName)
		if err != nil {
			return err
		}

		for _, childFile := range files {
			if err := saveFile(tw, filepath.Join(fileName, childFile.Name()), childFile.IsDir()); err != nil {
				return err
			}
		}
		return nil
	}

	if _, err := io.Copy(tw, file); err != nil {
		return err
	}

	return nil
}

// Assumes that the current working directory is the target and it is clean
func (c *controller) restoreBackup() error {
	backup, err := c.getBackup()
	// if there is no backup to restore, start a fresh server
	if err == errBackupDoesNotExist {
		return nil
	}
	if err != nil {
		log.Fatal(err)
	}
	defer backup.Close()

	gzr, err := gzip.NewReader(backup)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		// end of tar file, done
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		default:
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(header.Name, 0777); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(header.Name, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
		default:
			return errors.New("Unknown tar type:" + string(header.Typeflag))
		}

	}
	log.Println("Restored backup")
	return nil
}
