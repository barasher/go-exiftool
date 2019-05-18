# go-exiftool

[![Build Status](https://travis-ci.org/barasher/go-exiftool.svg?branch=master)](https://travis-ci.org/barasher/go-exiftool)
[![go report card](https://goreportcard.com/badge/github.com/barasher/go-exiftool "go report card")](https://goreportcard.com/report/github.com/barasher/go-exiftool)
[![GoDoc](https://godoc.org/github.com/barasher/go-exiftool?status.svg)](https://godoc.org/github.com/barasher/go-exiftool)

**go-exiftool** is a golang library that wraps [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/).

## Requirements

**go-exiftool** needs [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/) to be installed.

- On Debian : `sudo apt-get install exiftool`

## Usage

```go
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
```