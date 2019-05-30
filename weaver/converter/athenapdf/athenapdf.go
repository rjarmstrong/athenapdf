package athenapdf

import (
	"log"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rjarmstrong/athenapdf/weaver/converter"
	"github.com/rjarmstrong/athenapdf/weaver/gcmd"
)

// AthenaPDF represents a conversion job for athenapdf CLI.
// AthenaPDF implements the Converter interface with a custom Convert method.
type AthenaPDF struct {
	// AthenaPDF inherits properties from UploadConversion, and as such,
	// it supports uploading of its results to S3
	// (if the necessary credentials are given).
	// See UploadConversion for more information.
	converter.UploadConversion
	// CMD is the base athenapdf CLI command that will be executed.
	// e.g. 'athenapdf -S -T 120'
	CMD        string
	AthenaArgs Args
}

type Args struct {
	PageSize *string
	Delay    *int
	Zoom     *int
	// Cookie sets a cookie in the Electron Browser window to impersonate the calling user
	*Cookie
	// Aggressive will alter the athenapdf CLI conversion behaviour by passing
	// an '-A' command-line flag to indicate aggressive content extraction
	// (ideal for a clutter-free reading experience).
	Aggressive bool
	// WaitForStatus will wait until window.status === WINDOW_STATUS
	WaitForStatus bool
}

type Cookie struct {
	Url   string
	Name  string
	Value string
}

// constructCMD returns a string array containing the AthenaPDF command to be
// executed by Go's os/exec Output. It does this using a base command, and path
// string.
// It will set an additional '-A' flag if aggressive is set to true.
// See athenapdf CLI for more information regarding the aggressive mode.
func constructCMD(base string, path string, athArgs Args) []string {
	args := strings.Fields(base)
	if athArgs.Aggressive {
		args = append(args, "-A")
	}
	if athArgs.WaitForStatus {
		args = append(args, "--wait-for-status")
	}
	if athArgs.Cookie != nil {
		args = append(args, "--cookie-name", athArgs.Cookie.Name, "--cookie-value", athArgs.Cookie.Value, "--cookie-url", athArgs.Cookie.Url)
	}
	if athArgs.Zoom != nil {
		args = append(args, "-Z", strconv.Itoa(*athArgs.Zoom))
	}
	if athArgs.Delay != nil {
		args = append(args, "-D", strconv.Itoa(*athArgs.Delay))
	}
	if athArgs.PageSize != nil {
		args = append(args, "-P", *athArgs.PageSize)
	}
	args = append(args, path)
	return args
}

// Convert returns a byte slice containing a PDF converted from HTML
// using athenapdf CLI.
// See the Convert method for Conversion for more information.
func (c AthenaPDF) Convert(s converter.ConversionSource, done <-chan struct{}) ([]byte, error) {
	log.Printf("[AthenaPDF] converting to PDF: %s\n", s.GetActualURI())

	// Construct the command to execute
	cmd := constructCMD(c.CMD, s.URI, c.AthenaArgs)

	// TASK: dev env
	//cmd[0] = "/Volumes/development/go/src/github.com/rjarmstrong/athenapdf/cli/bin/athenapdf"

	log.Printf("[AthenaPDF] executing: %s\n", cmd)

	out, err := gcmd.Execute(cmd, done)
	if err != nil {
		return nil, errors.WithMessage(err, "error running athena pdf:")
	}

	return out, nil
}
