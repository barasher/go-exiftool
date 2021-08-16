package exiftool_test

import (
	"fmt"
	"io"
	"os"

	"io/ioutil"
	"path/filepath"

	"github.com/barasher/go-exiftool"
)

func ExampleExiftool_Read() {
	et, err := exiftool.NewExiftool()
	if err != nil {
		fmt.Printf("Error when intializing: %v\n", err)
		return
	}
	defer et.Close()

	fileInfos := et.ExtractMetadata("testdata/20190404_131804.jpg")

	for _, fileInfo := range fileInfos {
		if fileInfo.Err != nil {
			fmt.Printf("Error concerning %v: %v\n", fileInfo.File, fileInfo.Err)
			continue
		}

		for k, v := range fileInfo.Fields {
			fmt.Printf("[%v] %v\n", k, v)
		}
	}
}

func copyFile(src, dest string) (err error) {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	if err != nil {
		return err
	}
	return nil
}

func ExampleExiftool_Write() {
	// error handling are skipped in this example

	tmpDir, _ := ioutil.TempDir("", "ExampleExiftoolWrite")
	testFile := filepath.Join(tmpDir, "20190404_131804.jpg")
	copyFile("testdata/20190404_131804.jpg", testFile)

	e, _ := exiftool.NewExiftool()
	defer e.Close()
	originals := e.ExtractMetadata(testFile)
	title, _ := originals[0].GetString("Title")
	fmt.Println("title:" + title)

	originals[0].SetString("Title", "newTitle")
	e.WriteMetadata(originals)

	altered := e.ExtractMetadata(testFile)
	title, _ = altered[0].GetString("Title")
	fmt.Println("title:" + title)

	// Output:
	// title:
	// title:newTitle
}
