package athenapdf

import (
	"log"
	"strings"

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
	CMD string
	// Aggressive will alter the athenapdf CLI conversion behaviour by passing
	// an '-A' command-line flag to indicate aggressive content extraction
	// (ideal for a clutter-free reading experience).
	Aggressive bool
	// WaitForStatus will wait until window.status === WINDOW_STATUS
	WaitForStatus bool
	// Cookie sets a cookie in the Electron Browser window to impersonate the calling user
	Cookie *Cookie
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
func constructCMD(base string, path string, aggressive bool, waitForStatus bool, cookie *Cookie) []string {
	args := strings.Fields(base)
	if aggressive {
		args = append(args, "-A")
	}
	if waitForStatus {
		args = append(args, "--wait-for-status")
	}
	if cookie != nil {
		args = append(args, "--cookieName", cookie.Name, "--cookieValue", cookie.Value, "--cookieUrl", cookie.Url)
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
	cmd := constructCMD(c.CMD, s.URI, c.Aggressive, c.WaitForStatus, c.Cookie)

	log.Printf("[AthenaPDF] executing: %s\n", cmd)

	out, err := gcmd.Execute(cmd, done)
	if err != nil {
		return nil, err
	}

	return out, nil
}
