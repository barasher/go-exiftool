package exiftool

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExiftoolEmpty(t *testing.T) {
	t.Parallel()

	e, err := NewExiftool()
	assert.Nil(t, err)

	defer e.Close()
}

func TestNewExifToolOptOk(t *testing.T) {
	t.Parallel()

	var exec1, exec2 bool

	f1 := func(*Exiftool) error {
		exec1 = true
		return nil
	}

	f2 := func(*Exiftool) error {
		exec2 = true
		return nil
	}

	e, err := NewExiftool(f1, f2)
	assert.Nil(t, err)

	defer e.Close()

	assert.True(t, exec1)
	assert.True(t, exec2)
}

func TestNewExifToolOptKo(t *testing.T) {
	t.Parallel()

	f := func(*Exiftool) error {
		return fmt.Errorf("err")
	}
	_, err := NewExiftool(f)
	assert.NotNil(t, err)
}
func TestSingleExtract(t *testing.T) {
	var tcs = []struct {
		tcID    string
		inFiles []string
		expOk   []bool
	}{
		{"single", []string{"./testdata/20190404_131804.jpg"}, []bool{true}},
		{"multiple", []string{"./testdata/20190404_131804.jpg", "./testdata/20190404_131804.jpg"}, []bool{true, true}},
		{"nonExisting", []string{"./testdata/nonExisting"}, []bool{false}},
		{"empty", []string{"./testdata/empty.jpg"}, []bool{true}},
	}

	for _, tc := range tcs {
		tc := tc // Pin variable
		t.Run(tc.tcID, func(t *testing.T) {
			t.Parallel()

			e, err := NewExiftool()
			assert.Nilf(t, err, "error not nil: %v", err)
			defer e.Close()
			fms := e.ExtractMetadata(tc.inFiles...)
			assert.Equal(t, len(tc.expOk), len(fms))
			for i, fm := range fms {
				t.Log(fm)
				assert.Equalf(t, tc.expOk[i], fm.Err == nil, "#%v different", i)
			}
		})
	}
}

func TestMultiExtract(t *testing.T) {
	t.Parallel()

	e, err := NewExiftool()

	assert.Nilf(t, err, "error not nil: %v", err)

	defer e.Close()

	f := e.ExtractMetadata("./testdata/20190404_131804.jpg", "./testdata/20190404_131804.jpg")

	assert.Equal(t, 2, len(f))
	assert.Nil(t, f[0].Err)
	assert.Nil(t, f[1].Err)

	f = e.ExtractMetadata("./testdata/nonExisting.bla")

	assert.Equal(t, 1, len(f))
	assert.NotNil(t, f[0].Err)

	f = e.ExtractMetadata("./testdata/20190404_131804.jpg")

	assert.Equal(t, 1, len(f))
	assert.Nil(t, f[0].Err)
}

func TestSplitReadyToken(t *testing.T) {
	rt := string(readyToken)

	var tcs = []struct {
		tcID    string
		in      string
		expOk   bool
		expVals []string
	}{
		{"mono", "a" + rt, true, []string{"a"}},
		{"multi", "a" + rt + "b" + rt, true, []string{"a", "b"}},
		{"empty", "", true, []string{}},
		{"monoNoFinalToken", "a", false, []string{}},
		{"multiNoFinalToken", "a" + rt + "b", false, []string{}},
		{"emptyWithToken", rt, true, []string{""}},
	}

	for _, tc := range tcs {
		tc := tc // Pin variable
		t.Run(tc.tcID, func(t *testing.T) {
			sc := bufio.NewScanner(strings.NewReader(tc.in))
			sc.Split(splitReadyToken)
			vals := []string{}
			for sc.Scan() {
				vals = append(vals, sc.Text())
			}
			assert.Equal(t, tc.expOk, sc.Err() == nil)
			if tc.expOk {
				assert.Equal(t, tc.expVals, vals)
			}
		})
	}
}

func TestCloseNominal(t *testing.T) {
	var rClosed, wClosed bool

	r := readWriteCloserMock{closed: &rClosed}
	w := readWriteCloserMock{closed: &wClosed}
	e := Exiftool{stdin: r, stdMergedOut: w}

	assert.Nil(t, e.Close())
	assert.True(t, rClosed)
	assert.True(t, wClosed)
}

func TestCloseErrorOnStdin(t *testing.T) {
	var rClosed, wClosed bool

	r := readWriteCloserMock{closed: &rClosed, closeErr: fmt.Errorf("error")}
	w := readWriteCloserMock{closed: &wClosed}
	e := Exiftool{stdin: r, stdMergedOut: w}

	assert.NotNil(t, e.Close())
	assert.True(t, rClosed)
	assert.True(t, wClosed)
}

func TestCloseErrorOnStdout(t *testing.T) {
	var rClosed, wClosed bool

	r := readWriteCloserMock{closed: &rClosed}
	w := readWriteCloserMock{closed: &wClosed, closeErr: fmt.Errorf("error")}
	e := Exiftool{stdin: r, stdMergedOut: w}

	assert.NotNil(t, e.Close())
	assert.True(t, rClosed)
	assert.True(t, wClosed)
}

func TestCloseExifToolNominal(t *testing.T) {
	t.Parallel()

	e, err := NewExiftool()

	assert.Nil(t, err)
	assert.Nil(t, e.Close())
}

type readWriteCloserMock struct {
	writeInt int
	writeErr error
	readInt  int
	readErr  error
	closeErr error
	closed   *bool
}

func (e readWriteCloserMock) Write(p []byte) (n int, err error) {
	return e.writeInt, e.writeErr
}

func (e readWriteCloserMock) Read(p []byte) (n int, err error) {
	return e.readInt, e.readErr
}

func (e readWriteCloserMock) Close() error {
	*(e.closed) = true
	return e.closeErr
}

func TestBuffer(t *testing.T) {
	t.Parallel()

	e, err := NewExiftool()
	assert.Nil(t, err)
	defer e.Close()
	assert.Equal(t, false, e.bufferSet)

	buf := make([]byte, 128)
	assert.Nil(t, Buffer(buf, 64)(e))
	assert.Equal(t, true, e.bufferSet)
	assert.Equal(t, buf, e.buffer)
	assert.Equal(t, 64, e.bufferMaxSize)
}

func TestNewExifTool_WithBuffer(t *testing.T) {
	t.Parallel()

	buf := make([]byte, 128*1000)
	e, err := NewExiftool(Buffer(buf, 64*1000))
	assert.Nil(t, err)
	defer e.Close()

	metas := e.ExtractMetadata("./testdata/20190404_131804.jpg")
	assert.Equal(t, 1, len(metas))
	assert.Nil(t, metas[0].Err)
}

func TestCharset(t *testing.T) {
	t.Parallel()

	e, err := NewExiftool()
	assert.Nil(t, err)
	defer e.Close()
	lengthBefore := len(e.extraInitArgs)

	assert.Nil(t, Charset("charsetValue")(e))
	assert.Equal(t, lengthBefore+2, len(e.extraInitArgs))
	assert.Equal(t, "-charset", e.extraInitArgs[lengthBefore])
	assert.Equal(t, "charsetValue", e.extraInitArgs[lengthBefore+1])
}

func TestNewExifTool_WithCharset(t *testing.T) {
	t.Parallel()

	e, err := NewExiftool(Charset("filename=utf8"))
	assert.Nil(t, err)
	defer e.Close()

	metas := e.ExtractMetadata("./testdata/20190404_131804.jpg")
	assert.Equal(t, 1, len(metas))
	assert.Nil(t, metas[0].Err)
}

func TestNoPrintConversion(t *testing.T) {
	t.Parallel()

	e, err := NewExiftool(NoPrintConversion())
	assert.Nil(t, err)
	defer e.Close()

	metas := e.ExtractMetadata("./testdata/20190404_131804.jpg")
	assert.Equal(t, 1, len(metas))
	assert.Nil(t, metas[0].Err)

	for _, meta := range metas {
		if meta.Err != nil {
			continue
		}
		expProgram, err := meta.GetInt("ExposureProgram")
		assert.Nil(t, err)
		assert.Equal(t, int64(2), expProgram)
	}
}

func TestExtractEmbedded(t *testing.T) {
	t.Parallel()

	eWithout, err := NewExiftool()
	assert.Nil(t, err)
	defer eWithout.Close()
	metas := eWithout.ExtractMetadata("./testdata/extractEmbedded.mp4")
	assert.Equal(t, 1, len(metas))
	assert.Nil(t, metas[0].Err)
	_, err = metas[0].GetString("OtherSerialNumber")
	assert.Equal(t, ErrKeyNotFound, err)

	eWith, err := NewExiftool(ExtractEmbedded())
	assert.Nil(t, err)
	defer eWith.Close()
	metas = eWith.ExtractMetadata("./testdata/extractEmbedded.mp4")
	assert.Equal(t, 1, len(metas))
	assert.Nil(t, metas[0].Err)
	osn, err := metas[0].GetString("OtherSerialNumber")
	assert.Nil(t, err)
	assert.Equal(t, "HERO4 Silver", osn)
}

func TestExtractAllBinaryMetadata(t *testing.T) {
	t.Parallel()

	eWithout, err := NewExiftool()
	assert.Nil(t, err)
	defer eWithout.Close()
	metas := eWithout.ExtractMetadata("./testdata/binary.mp3")
	assert.Equal(t, 1, len(metas))
	assert.Nil(t, metas[0].Err)
	osn, err := metas[0].GetString("Picture")
	assert.Nil(t, err)
	assert.False(t, strings.HasPrefix(osn, "base64")) // backward compatibility

	eWith, err := NewExiftool(ExtractAllBinaryMetadata())
	assert.Nil(t, err)
	defer eWith.Close()
	metas = eWith.ExtractMetadata("./testdata/binary.mp3")
	assert.Equal(t, 1, len(metas))
	assert.Nil(t, metas[0].Err)
	osn, err = metas[0].GetString("Picture")
	assert.Nil(t, err)
	assert.True(t, strings.HasPrefix(osn, "base64"))
}

func TestSetExiftoolBinaryPath(t *testing.T) {
	t.Parallel()

	// default
	eDefault, err := NewExiftool()
	assert.Nil(t, err)
	defer eDefault.Close()
	f := eDefault.ExtractMetadata("./testdata/20190404_131804.jpg")
	assert.Equal(t, 1, len(f))
	assert.Nil(t, f[0].Err)

	// set path
	exiftoolPath, err := exec.LookPath(exiftoolBinary)
	assert.Nil(t, err)
	t.Logf("exiftool path: %v", exiftoolPath)
	eSet, err := NewExiftool(SetExiftoolBinaryPath(exiftoolPath))
	assert.Nil(t, err)
	defer eSet.Close()
	f = eSet.ExtractMetadata("./testdata/20190404_131804.jpg")
	assert.Equal(t, 1, len(f))
	assert.Nil(t, f[0].Err)

	// error on init
	_, err = NewExiftool(SetExiftoolBinaryPath("/non/existing/path"))
	assert.NotNil(t, err)
}

func TestWriteMetadataSuccessTokenHandling(t *testing.T) {
	testCases := []struct {
		name      string
		testResp  string
		expectErr bool
	}{
		{name: "token as full resp", testResp: writeMetadataSuccessToken, expectErr: false},
		{name: "token at resp end", testResp: "prefix text" + writeMetadataSuccessToken,
			expectErr: false},
		{name: "token at resp middle",
			testResp:  "prefix text" + writeMetadataSuccessToken + "suffix text",
			expectErr: true},
		{name: "token at resp beginning", testResp: writeMetadataSuccessToken + "suffix text",
			expectErr: true},
		{name: "no token", testResp: "some error message", expectErr: true},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			err := handleWriteMetadataResponse(tc.testResp)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestWriteMetadataFails(t *testing.T) {
	t.Parallel()

	nonWritableFile := filepath.Join(t.TempDir(), "binary.mp3")
	require.Nil(t, copyFile("testdata/binary.mp3", nonWritableFile))

	e, err := NewExiftool()
	require.Nil(t, err)
	defer e.Close()

	testCases := []struct {
		tcID   string
		inFile string
		expOk  bool
	}{
		{"nonExisting", "nonExisting", false},
		{"nonWritable", nonWritableFile, false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.tcID, func(t *testing.T) {
			fm := EmptyFileMetadata()
			fm.File = tc.inFile
			fm.SetString("Title", "fakeTitle")
			fms := []FileMetadata{fm}
			e.WriteMetadata(fms)

			if tc.expOk {
				require.Nil(t, fms[0].Err)
			} else {
				require.NotNil(t, fms[0].Err)
			}
		})
	}

}

func TestWriteMetadataNominal(t *testing.T) {
	t.Parallel()

	testFile := filepath.Join(t.TempDir(), "20190404_131804.jpg")
	require.Nil(t, copyFile("testdata/20190404_131804.jpg", testFile))

	e, err := NewExiftool()
	require.Nil(t, err)
	defer e.Close()

	mds := e.ExtractMetadata(testFile)
	require.Len(t, mds, 1)
	require.Nil(t, mds[0].Err)

	mds[0].SetString("Title", "fakeTitle")
	mds[0].SetFloat("CameraImagingModelPixelAspectRatio", float64(1.5))
	mds[0].SetString("ImageUniqueID", "newID")
	mds[0].SetStrings("Keywords", []string{"kw1", "kw2"})
	mds[0].Clear("Flash")
	mds[0].Err = nil // TODO should be nilled before writing medatada
	e.WriteMetadata(mds)
	require.Nil(t, mds[0].Err)

	mds2 := e.ExtractMetadata(testFile)
	require.Len(t, mds2, 1)
	require.Nil(t, mds2[0].Err)

	gotStr, err := mds2[0].GetString("Title")
	require.Nil(t, err)
	require.Equal(t, "fakeTitle", gotStr)

	gotFloat, err := mds2[0].GetFloat("CameraImagingModelPixelAspectRatio")
	require.Nil(t, err)
	require.Equal(t, float64(1.5), gotFloat)

	gotStrings, err := mds2[0].GetStrings("Keywords")
	require.Nil(t, err)
	require.Equal(t, []string{"kw1", "kw2"}, gotStrings)

	// override
	gotOverStr, err := mds2[0].GetString("ImageUniqueID")
	require.Nil(t, err)
	require.Equal(t, "newID", gotOverStr)

	// delete
	_, err = mds2[0].GetString("Flash")
	require.Equal(t, ErrKeyNotFound, err)
}

func TestWriteMetadataInvalidField(t *testing.T) {
	t.Parallel()

	testFile := filepath.Join(t.TempDir(), "20190404_131804.jpg")
	require.Nil(t, copyFile("testdata/20190404_131804.jpg", testFile))

	e, err := NewExiftool()
	require.Nil(t, err)
	defer e.Close()

	mds := []FileMetadata{EmptyFileMetadata()}
	mds[0].SetString("InvalidField", "ValueDoesNotMatter")
	e.WriteMetadata(mds)
	require.NotNil(t, mds[0].Err)
}

func TestWriteMetadataClearExistingFields(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		tcID      string
		inEnabled bool
		expExists bool
	}{
		{"disabled", false, true},
		{"enabled", true, false},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.tcID, func(t *testing.T) {
			testFile := filepath.Join(t.TempDir(), "20190404_131804.jpg")
			require.Nil(t, copyFile("testdata/20190404_131804.jpg", testFile))

			var e *Exiftool
			var err error
			if tc.inEnabled {
				e, err = NewExiftool(ClearFieldsBeforeWriting())
			} else {
				e, err = NewExiftool()
			}
			require.Nil(t, err)
			defer e.Close()

			mds := e.ExtractMetadata(testFile)
			require.Len(t, mds, 1)
			require.Nil(t, mds[0].Err)
			_, err = mds[0].GetString("ImageUniqueID")
			require.Nil(t, err)

			mds2 := []FileMetadata{EmptyFileMetadata()}
			mds2[0].File = mds[0].File
			mds2[0].SetString("Title", "fakeTitle")
			e.WriteMetadata(mds2)
			require.Nil(t, mds2[0].Err)

			mds3 := e.ExtractMetadata(testFile)
			require.Len(t, mds3, 1)
			require.Nil(t, mds3[0].Err)
			_, err = mds3[0].GetString("ImageUniqueID")
			if tc.expExists {
				require.Nil(t, err)
			} else {
				require.Equal(t, ErrKeyNotFound, err)
			}
		})
	}
}

func TestWriteMetadataBackupOriginal(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		tcID      string
		inEnabled bool
	}{
		{"disabled", false},
		{"enabled", true},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.tcID, func(t *testing.T) {

			filename := fmt.Sprintf("20190404_131804_%v.jpg", tc.tcID)
			testFile := filepath.Join(t.TempDir(), filename)
			require.Nil(t, copyFile("testdata/20190404_131804.jpg", testFile))

			var e *Exiftool
			var err error
			if tc.inEnabled {
				e, err = NewExiftool(BackupOriginal())
			} else {
				e, err = NewExiftool()
			}
			require.Nil(t, err)
			defer e.Close()

			mds := []FileMetadata{EmptyFileMetadata()}
			mds[0].File = testFile
			mds[0].SetString("ImageUniqueID", "newValue")
			e.WriteMetadata(mds)
			require.Nil(t, mds[0].Err)

			backupedFile := fmt.Sprintf("%v_original", testFile)
			_, err = os.Stat(backupedFile)
			if tc.inEnabled {
				require.Nil(t, err)
			} else {
				require.True(t, errors.Is(err, os.ErrNotExist))
			}
		})
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
