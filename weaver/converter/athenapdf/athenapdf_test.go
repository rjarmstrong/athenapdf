package athenapdf

import (
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rjarmstrong/athenapdf/weaver/converter"
	"github.com/rjarmstrong/athenapdf/weaver/testutil"
)

func TestConstructCMD(t *testing.T) {
	got := constructCMD("athenapdf -S -T 120", "test_file.html", true, false, &Cookie{
		Url:   "http://cookie-url",
		Name:  "my-cookie",
		Value: "my-cookie-val",
	})
	want := []string{"athenapdf", "-S", "-T", "120", "-A", "--cookieName", "my-cookie", "--cookieValue", "my-cookie-val", "--cookieUrl", "http://cookie-url", "test_file.html"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected constructed athenapdf command to be %+v, got %+v", want, got)
	}
}

func mockConversion(path string, tmp bool, cmd string) ([]byte, error) {
	c := AthenaPDF{}
	c.CMD = cmd
	s := converter.ConversionSource{}
	s.URI = path
	s.IsLocal = tmp
	t := make(chan struct{}, 1)
	return c.Convert(s, t)
}

func TestConvert(t *testing.T) {
	ts := testutil.MockHTTPServer("", "test AthenaPDF convert", false)
	defer ts.Close()
	got, err := mockConversion(ts.URL, false, "echo")
	if err != nil {
		t.Fatalf("convert returned an unexpected error: %+v", err)
	}
	if want := []byte(ts.URL + "\n"); !reflect.DeepEqual(got, want) {
		t.Errorf("expected output of athenapdf conversion to be %s, got %s", want, got)
	}
}

func TestConvert_local(t *testing.T) {
	f, err := ioutil.TempFile("/tmp", "tmp")
	if err != nil {
		t.Fatalf("unable to create temporary file for testing: %+v", err)
	}
	p, err := filepath.Abs(f.Name())
	if err != nil {
		t.Fatalf("unable to get full temporary file path: %+v", err)
	}
	got, err := mockConversion(p, true, "echo")
	if err != nil {
		t.Fatalf("convert returned an unexpected error: %+v", err)
	}
	if want := []byte(p + "\n"); !reflect.DeepEqual(got, want) {
		t.Errorf("expected output of athenapdf conversion to be %s, got %s", want, got)
	}
}

func TestConvert_badCMD(t *testing.T) {
	ts := testutil.MockHTTPServer("", "test Athena convert", false)
	defer ts.Close()
	got, err := mockConversion(ts.URL, false, "echo-broken")
	if err == nil {
		t.Fatalf("expected error to be returned")
	}
	if got != nil {
		t.Errorf("expected output of athenapdf conversion to be nil, got %s", got)
	}
}
