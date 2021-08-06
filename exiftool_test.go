package exiftool

import (
	"bufio"
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

var fieldsForWriting = map[string]interface{}{
	"Title":       "fake title",
	"Description": "fake description",
	// TODO: Test ints
	//       Not testing ints since we'd need to update the
	//       decoder and FileMetadata to use UseNumber()
	// "CameraImagingModelImageHeight": 5, // test ints
	"CameraImagingModelPixelAspectRatio": 1.5, // test reals/floats
}

func TestWriteMetadata(t *testing.T) {
	t.Parallel()

	runWriteTest(t, func(t *testing.T, tmpDir string) {
		e, err := NewExiftool()
		require.Nil(t, err)

		type testCase struct {
			md        FileMetadata
			expectErr bool
		}
		testCases := []testCase{
			{md: FileMetadata{File: filepath.Join(tmpDir, "20190404_131804.jpg"),
				Fields: fieldsForWriting}, expectErr: false},
			{md: FileMetadata{File: filepath.Join(tmpDir, "binary.mp3"), Fields: fieldsForWriting},
				expectErr: true},
			{md: FileMetadata{File: filepath.Join(tmpDir, "empty.jpg"), Fields: fieldsForWriting},
				expectErr: true},
			{md: FileMetadata{File: filepath.Join(tmpDir, "extractEmbedded.mp4"),
				Fields: fieldsForWriting}, expectErr: false},
			{md: FileMetadata{File: filepath.Join(tmpDir, "file_does_not_exist"),
				Fields: fieldsForWriting}, expectErr: true},
		}

		mds := make([]FileMetadata, 0, len(testCases))
		filenames := make([]string, 0, len(testCases))
		for _, tc := range testCases {
			mds = append(mds, tc.md)
			filenames = append(filenames, tc.md.File)
		}

		e.WriteMetadata(mds)
		for i, md := range mds {
			if testCases[i].expectErr {
				assert.Error(t, md.Err, "file: "+md.File)
			} else {
				assert.Nil(t, md.Err, "file: "+md.File)
			}
		}

		updatedMDs := e.ExtractMetadata(filenames...)
		require.Equal(t, len(mds), len(updatedMDs))

		for i := 0; i < len(mds); i++ {
			tc := testCases[i]
			expected := mds[i]
			actual := updatedMDs[i]

			if tc.expectErr {
				if expected.Err != nil {
					// handles the case where exiftool supports reading from a file type
					// but not writing
					return
				}
				assert.Error(t, actual.Err)
				return
			}

			assert.Nil(t, actual.Err)
			assert.Equal(t, tc.md.File, actual.File)

			for k := range expected.Fields {
				assert.Equal(t, expected.Fields[k], actual.Fields[k], "Field %s differs", k)
			}
		}
	})
}

func TestWriteMetadataInvalidField(t *testing.T) {
	t.Parallel()

	fields := map[string]interface{}{
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
			assert.Error(t, md.Err, "file: "+md.File)
		}
	})
}

var fieldsNotDeleted = map[string]struct{}{
	"ExifToolVersion":     {},
	"FileName":            {},
	"SourceFile":          {},
	"Directory":           {},
	"FileSize":            {},
	"FileModifyDate":      {},
	"FileAccessDate":      {},
	"FileInodeChangeDate": {},
	"FilePermissions":     {},
	"FileType":            {},
	"FileTypeExtension":   {},
	"MIMEType":            {},
	"ImageWidth":          {},
	"ImageHeight":         {},
	"EncodingProcess":     {},
	"BitsPerSample":       {},
	"ColorComponents":     {},
	"YCbCrSubSampling":    {},
	"ImageSize":           {},
	"Megapixels":          {},
}

func TestWriteMetadataClearExistingFields(t *testing.T) {
	t.Parallel()

	runWriteTest(t, func(t *testing.T, tmpDir string) {
		e, err := NewExiftool(ClearFieldsBeforeWriting())
		require.Nil(t, err)

		filename := filepath.Join(tmpDir, "20190404_131804.jpg")
		mds := []FileMetadata{{File: filename}}

		fieldsToBeDeleted := []string{}
		origMDs := e.ExtractMetadata(filename)
		assert.Equal(t, len(mds), len(origMDs))
		assert.Nil(t, origMDs[0].Err)
		for f := range origMDs[0].Fields {
			if _, ok := fieldsNotDeleted[f]; ok {
				continue
			}
			fieldsToBeDeleted = append(fieldsToBeDeleted, f)
		}
		assert.NotEmpty(t, fieldsToBeDeleted)

		e.WriteMetadata(mds)
		assert.Nil(t, mds[0].Err)

		updatedMDs := e.ExtractMetadata(filename)
		assert.Equal(t, len(mds), len(updatedMDs))
		assert.Nil(t, updatedMDs[0].Err)
		for _, f := range fieldsToBeDeleted {
			assert.NotContains(t, updatedMDs[0].Fields, f)
		}
	})
}

func TestWriteMetadataClearBeforeWriting(t *testing.T) {
	t.Parallel()

	runWriteTest(t, func(t *testing.T, tmpDir string) {
		e, err := NewExiftool(ClearFieldsBeforeWriting())
		require.Nil(t, err)

		filename := filepath.Join(tmpDir, "20190404_131804.jpg")
		mds := []FileMetadata{{File: filename, Fields: fieldsForWriting}}

		fieldsToBeDeleted := []string{}
		origMDs := e.ExtractMetadata(filename)
		assert.Equal(t, len(mds), len(origMDs))
		assert.Nil(t, origMDs[0].Err)
		for f := range origMDs[0].Fields {
			if _, ok := fieldsNotDeleted[f]; ok {
				continue
			}
			fieldsToBeDeleted = append(fieldsToBeDeleted, f)
		}
		assert.NotEmpty(t, fieldsToBeDeleted)

		e.WriteMetadata(mds)
		assert.Nil(t, mds[0].Err)

		updatedMDs := e.ExtractMetadata(filename)
		assert.Equal(t, len(mds), len(updatedMDs))
		assert.Nil(t, updatedMDs[0].Err)
		for _, f := range fieldsToBeDeleted {
			assert.NotContains(t, updatedMDs[0].Fields, f)
		}
		for f := range fieldsForWriting {
			assert.Equal(t, fieldsForWriting[f], updatedMDs[0].Fields[f])
		}
	})
}

func TestWriteMetadataDeleteField(t *testing.T) {
	t.Parallel()

	runWriteTest(t, func(t *testing.T, tmpDir string) {
		e, err := NewExiftool()
		require.Nil(t, err)

		fieldToDelete := "Flash"
		filename := filepath.Join(tmpDir, "20190404_131804.jpg")
		mds := []FileMetadata{{File: filename, Fields: map[string]interface{}{}}}
		mds[0].Fields[fieldToDelete] = nil

		origMDs := e.ExtractMetadata(filename)
		assert.Equal(t, len(mds), len(origMDs))
		assert.Nil(t, origMDs[0].Err)
		assert.Equal(t, "No Flash", origMDs[0].Fields[fieldToDelete])

		e.WriteMetadata(mds)
		assert.Nil(t, mds[0].Err)

		updatedMDs := e.ExtractMetadata(filename)
		assert.Equal(t, len(mds), len(updatedMDs))
		assert.Nil(t, updatedMDs[0].Err)
		_, ok := updatedMDs[0].Fields[fieldToDelete]
		assert.False(t, ok)
	})
}

func TestWriteMetadataBackupOriginal(t *testing.T) {
	testCases := []struct {
		name               string
		args               []func(*Exiftool) error
		expectedNumMatches int
	}{
		{name: "backup original", args: []func(*Exiftool) error{BackupOriginal()}, expectedNumMatches: 1},
		{name: "overwrite original", args: nil, expectedNumMatches: 0},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runWriteTest(t, func(t *testing.T, tmpDir string) {
				e, err := NewExiftool(tc.args...)
				require.Nil(t, err)

				mds := []FileMetadata{{File: filepath.Join(tmpDir, "20190404_131804.jpg"),
					Fields: fieldsForWriting}}
				e.WriteMetadata(mds)
				for _, md := range mds {
					assert.Nil(t, md.Err, "file: "+md.File)
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
	tmpDir := t.TempDir()
	err := copyDir("testdata", tmpDir)
	require.Nil(t, err, "Unable to copy testdata to temporary directory: "+tmpDir)

	f(t, tmpDir)
}

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if path == src {
				return nil
			}
			return filepath.SkipDir
		}
		if !info.Mode().IsRegular() {
			// ignore non-regular files
			return nil
		}
		return copyFile(path, filepath.Join(dest, info.Name()))
	})
}

func copyFile(src, dest string) (err error) {
	var (
		s *os.File
		d *os.File
	)

	s, err = os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		e := s.Close()
		if err != nil {
			err = fmt.Errorf("Unable to close src file: %s. Orig error: %w", e.Error(), err)
		}
		err = e
	}()

	d, err = os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		e := d.Close()
		if err != nil {
			err = fmt.Errorf("Unable to close dest file: %s. Orig error: %w", e.Error(), err)
		}
		err = e
	}()

	_, err = io.Copy(d, s)
	if err != nil {
		return err
	}
	return nil
}
