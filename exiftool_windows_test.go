package exiftool

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"syscall"
	"testing"
)

func TestSystemProcessAttributes(t *testing.T) {
	t.Parallel()

	const CreateNoWindow = 0x08000000

	var sysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CreateNoWindow,
	}

	// I don't know what a good solution is to verify the process attributes are set on the actual process
	et, err := NewExiftool(SystemProcessAttributes(sysProcAttr))
	require.Nil(t, err, fmt.Sprintf("%v", err))
	defer et.Close()

	assert.Equal(t, sysProcAttr, et.sysProcAttr)
	assert.Equal(t, sysProcAttr, et.cmd.SysProcAttr)
}
