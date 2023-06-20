# go-exiftool

[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![Build Status](https://github.com/barasher/go-exiftool/workflows/go-exiftool-ci/badge.svg)](https://github.com/barasher/go-exiftool/actions)
[![go report card](https://goreportcard.com/badge/github.com/barasher/go-exiftool "go report card")](https://goreportcard.com/report/github.com/barasher/go-exiftool)
[![GoDoc](https://godoc.org/github.com/barasher/go-exiftool?status.svg)](https://godoc.org/github.com/barasher/go-exiftool)
[![codecov](https://codecov.io/gh/barasher/go-exiftool/branch/master/graph/badge.svg)](https://codecov.io/gh/barasher/go-exiftool)

**go-exiftool** is a golang library that wraps [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/).

[ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/)'s purpose is to extract and update as much metadata as possible (EXIF, IPTC, XMP, GPS, ...) from [a lots of differents file types](https://www.sno.phy.queensu.ca/~phil/exiftool/#supported) (Office documents, pictures, movies, PDF, ...).

**go-exiftool** uses [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/)'s *`stay_open`* feature to [optimize performance](https://www.sno.phy.queensu.ca/~phil/exiftool/#performance).

## Requirements

**go-exiftool** needs [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/) to be installed.

- On Debian : `sudo apt-get install exiftool`

By default, `go-exiftool` binary will look for `exiftool` binary in $PATH, but another location can be specified (see `SetExiftoolBinaryPath` functional option).

## Usage

### Metadata extraction

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

Output :

```
[FOV] 69.4 deg
[Orientation] Rotate 90 CW
[ColorSpace] sRGB
[Compression] JPEG (old-style)
[YCbCrSubSampling] YCbCr4:2:2 (2 1)
[Aperture] 1.7
[ColorComponents] 3
[SubSecCreateDate] 2019:04:04 13:18:03.0937
[FileSize] 26 kB
[FileAccessDate] 2019:05:17 22:44:26+02:00
[DateTimeOriginal] 2019:04:04 13:18:03
[CreateDate] 2019:04:04 13:18:03
(...)
```

### Metadata update

See [example function ExampleExiftool_Write in exiftool_sample_test.go](exiftool_sample_test.go)

## Changelog

- v1.1.0 : initial release
- [v1.1.1](https://github.com/barasher/go-exiftool/milestone/2)
  - [add typed getters for filemetadata fields](https://github.com/barasher/go-exiftool/issues/2)
- [v1.1.3](https://github.com/barasher/go-exiftool/milestone/3)
  - [add Mac & Windows support](https://github.com/barasher/go-exiftool/pull/7) (thanks to @PROger4ever)
  - [increase errors readability](https://github.com/barasher/go-exiftool/pull/8) (thanks to @PROger4ever)
- [v1.2.0](https://github.com/barasher/go-exiftool/milestone/4)
  - [add stdout and stderr buffer configuration capabilities](https://github.com/barasher/go-exiftool/issues/6) (thank to @asannikov)
- [v1.3.0](https://github.com/barasher/go-exiftool/milestone/5)
  - Add functionnal option to set [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/)'s `-charset` parameter (thank to @PROger4ever)
- [v1.3.1](https://github.com/barasher/go-exiftool/milestone/6)
  - [fix line separator on macOS](https://github.com/barasher/go-exiftool/issues/16)
- [v1.3.2](https://github.com/barasher/go-exiftool/milestone/7)
  - [add Freebsd support](https://github.com/barasher/go-exiftool/pull/21) (thanks to @ghyman1)
- [v1.4.0](https://github.com/barasher/go-exiftool/milestone/8)
  - ["NoPrintConversion" functional option](https://github.com/barasher/go-exiftool/pull/24) (thanks to [Kjeldgaard](https://github.com/Kjeldgaard))
- [v1.5.0](https://github.com/barasher/go-exiftool/milestone/9)
  - "ExtractEmbedded" functional option
- [v1.6.0](https://github.com/barasher/go-exiftool/milestone/10)
  - "ExtractAllBinaryMetadata" functional option
- [v1.6.1](https://github.com/barasher/go-exiftool/milestone/11)
  - "SetExiftoolBinaryPath" functional option : specify where to find exiftool binary (default : $path)
- [v1.6.2](https://github.com/barasher/go-exiftool/milestone/12)
  - Several improvements (thanks to [Dale Hui](https://github.com/dhui))
- [v1.7.0](https://github.com/barasher/go-exiftool/milestone/13)
  - Add metadata writing capabilities (thanks to [Dale Hui](https://github.com/dhui))
- [v1.8.0](https://github.com/barasher/go-exiftool/milestone/14)
  - [Fix a bug](https://github.com/barasher/go-exiftool/issues/52) that was blocking `go-exiftool` when a folder was provided. Folder metadata extraction will now return a new sentinel error (ErrNotFile).
  - Add a new sentinel error (`ErrBufferTooSmall`) that is returned when `go-exiftool`'s buffer isn't big enough for a specifi file (countermeasure: initialize `go-exiftool` with the `Buffer` option)
  - [New option to specify a date format (`DateFormat`)](https://github.com/barasher/go-exiftool/pull/60) (thanks to [Andy Gorman](https://github.com/agorman))
  - [New option to output format for GPS coordinates (`CoordFormat`)](https://github.com/barasher/go-exiftool/pull/60) (thanks to [Andy Gorman](https://github.com/agorman))
- [v1.9.0](https://github.com/barasher/go-exiftool/milestone/15)
  - [New option to specify `-api` parameter to Exiftool](https://github.com/barasher/go-exiftool/issues/59) (thanks to [Blesmol](https://github.com/Blesmol))
- [v1.10.0](https://github.com/barasher/go-exiftool/milestone/16)
  - [New option to prefix metadata keys with group names (specifies `-G` parameter to Exiftool)](https://github.com/barasher/go-exiftool/issues/67) (thanks to [Andy Gorman](https://github.com/agorman))

