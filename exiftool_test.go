package exiftool

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/otiai10/copy"
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
	testCases := []struct{
		name string
		testResp string
		expectErr bool
	}{
		{name: "token as full resp", testResp: writeMetadataSuccessToken, expectErr: false},
		{name: "token at resp end", testResp: "prefix text" + writeMetadataSuccessToken,
			expectErr: false},
		{name: "token at resp middle",
			testResp: "prefix text" + writeMetadataSuccessToken + "suffix text",
			expectErr: true},
		{name: "token at resp beginning", testResp: writeMetadataSuccessToken + "suffix text",
			expectErr: true},
		{name: "no token", testResp: "some error message", expectErr: true},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T){
			err := handleWriteMetadataResponse(tc.testResp)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestWriteMetadata(t *testing.T) {
	t.Parallel()

	fields := map[string]interface{} {
		"title": "fake title",
		"description": "fake description",
	}

	runWriteTest(t, func(t *testing.T, tmpDir string) {
		e, err := NewExiftool()
		require.Nil(t, err)

		testCases := []struct{
			md FileMetadata
			expectErr bool
		}{
			{md: FileMetadata{File: filepath.Join(tmpDir, "20190404_131804.jpg"), Fields: fields},
				expectErr: false},
			{md: FileMetadata{File: filepath.Join(tmpDir, "binary.mp3"), Fields: fields},
				expectErr: true},
			{md: FileMetadata{File: filepath.Join(tmpDir, "empty.jpg"), Fields: fields},
				expectErr: true},
			{md: FileMetadata{File: filepath.Join(tmpDir, "extractEmbedded.mp4"), Fields: fields},
				expectErr: false},
		}

		mds := make([]FileMetadata, 0, len(testCases))
		for _, tc := range testCases {
			mds = append(mds, tc.md)
		}

		e.WriteMetadata(mds)
		for i, md := range mds {
			if testCases[i].expectErr {
				assert.Error(t, md.Err, "file: " + md.File)
			} else {
				assert.Nil(t, md.Err, "file: " + md.File)
			}
		}
	})
}

func TestWriteMetadataInvalidField(t *testing.T) {
	t.Parallel()

	fields := map[string]interface{} {
		"not a valid field": "invalid field value",
	}

	runWriteTest(t, func(t *testing.T, tmpDir string) {
		e, err := NewExiftool()
		require.Nil(t, err)

		mds := []FileMetadata{
			{File: filepath.Join(tmpDir, "20190404_131804.jpg"), Fields: fields},
			{File: filepath.Join(tmpDir, "binary.mp3"), Fields: fields},
			{File: filepath.Join(tmpDir, "empty.jpg"), Fields: fields},
			{File: filepath.Join(tmpDir, "extractEmbedded.mp4"), Fields: fields},
		}

		e.WriteMetadata(mds)
		for _, md := range mds {
			assert.Error(t, md.Err, "file: " + md.File)
		}
	})
}

func TestWriteMetadataOverwriteOriginal(t *testing.T) {
	fields := map[string]interface{} {
		"title": "fake title",
		"description": "fake description",
	}

	testCases := []struct{
		name string
		args []func(*Exiftool) error
		expectedNumMatches int
	}{
		{name: "keep original", args: nil, expectedNumMatches: 1},
		{name: "overwrite original", args: []func(*Exiftool) error{OverwriteOriginal()}, expectedNumMatches: 0},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runWriteTest(t, func(t *testing.T, tmpDir string) {
				e, err := NewExiftool(tc.args...)
				require.Nil(t, err)

				mds := []FileMetadata{{File: filepath.Join(tmpDir, "20190404_131804.jpg"), Fields: fields}}
				e.WriteMetadata(mds)
				for _, md := range mds {
					assert.Nil(t, md.Err, "file: " + md.File)
				}

				matches, err := filepath.Glob(filepath.Join(tmpDir, "*_original"))
				assert.Nil(t, err)

				t.Log("matches:", matches)
				assert.Equal(t, tc.expectedNumMatches, len(matches))
			})
		})
	}
}

func runWriteTest(t *testing.T, f func(t *testing.T, tmpDir string)) {
	tmpDir, err := os.MkdirTemp("", "testdata*")
	require.Nil(t, err, "Unable to create temporary directory")
	defer func() {
		err := os.RemoveAll(tmpDir)
		assert.Nil(t, err, "Unable to remove temporary directory: " + tmpDir)
	}()

	err = copy.Copy("testdata", tmpDir, copy.Options{Sync: true});
	require.Nil(t, err, "Unable to copy testdata to temporary directory: " + tmpDir)

	f(t, tmpDir)
}
