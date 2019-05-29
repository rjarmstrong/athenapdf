package main

import (
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/getsentry/raven-go"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/rjarmstrong/athenapdf/weaver/converter"
	"github.com/rjarmstrong/athenapdf/weaver/converter/athenapdf"
	"github.com/rjarmstrong/athenapdf/weaver/converter/cloudconvert"
	"gopkg.in/alexcesaro/statsd.v2"
)

var (
	// ErrURLInvalid should be returned when a conversion URL is invalid.
	ErrURLInvalid = errors.New("invalid URL provided")
	// ErrFileInvalid should be returned when a conversion file is invalid.
	ErrFileInvalid = errors.New("invalid file provided")
)

// indexHandler returns a JSON string indicating that the microservice is online.
// It does not actually check if conversions are working. It is nevertheless,
// used for monitoring.
func indexHandler(c *gin.Context) {
	// We can do better than this...
	c.JSON(http.StatusOK, gin.H{"status": "online"})
}

// statsHandler returns a JSON string containing the number of running
// Goroutines, and pending jobs in the work queue.
func statsHandler(c *gin.Context) {
	q := c.MustGet("queue").(chan<- converter.Work)
	c.JSON(http.StatusOK, gin.H{
		"goroutines": runtime.NumGoroutine(),
		"pending":    len(q),
	})
}

func CreateAthenaConversion(c *gin.Context, uc converter.UploadConversion) *athenapdf.AthenaPDF {

	_, aggressive := c.GetQuery("aggressive")
	_, waitForStatus := c.GetQuery("waitForStatus")

	delay := 10000
	athena := &athenapdf.AthenaPDF{
		UploadConversion: uc,
		CMD:              AthenaBaseCommand,
		AthenaArgs: athenapdf.Args{
			Delay:         &delay,
			Aggressive:    aggressive,
			WaitForStatus: waitForStatus,
		},
	}

	cookieUrl := c.Request.URL.Query().Get("cookieUrl")
	if cookieUrl != "" {
		athena.AthenaArgs.Cookie = &athenapdf.Cookie{
			Url:   cookieUrl,
			Name:  c.Request.URL.Query().Get("cookieName"),
			Value: c.Request.URL.Query().Get("cookieValue"),
		}
	}

	return athena
}

func CreateAthenaConversionFromCookie(c *gin.Context, uc converter.UploadConversion) (*athenapdf.AthenaPDF, error) {

	_, aggressive := c.GetQuery("aggressive")
	_, waitForStatus := c.GetQuery("waitForStatus")

	var delay = 5
	delayStr := c.Request.URL.Query().Get("delay")
	if delayStr != "" {
		d, err2 := strconv.ParseInt(delayStr, 10, 32)
		if err2 != nil {
			return nil, err2
		}
		delay = int(d)
	}

	athena := &athenapdf.AthenaPDF{
		UploadConversion: uc,
		CMD:              AthenaBaseCommand,
		AthenaArgs: athenapdf.Args{
			Delay:         &delay,
			Aggressive:    aggressive,
			WaitForStatus: waitForStatus,
		},
	}

	cook, err := c.Request.Cookie("x-synergia-auth")
	if err != nil {
		log.Printf("%+v", err)
		return nil, ErrAuthorization
	}

	athena.AthenaArgs.Cookie = &athenapdf.Cookie{
		Url:   cook.Domain,
		Name:  cook.Name,
		Value: cook.Value,
	}

	return athena, nil
}

func conversionHandler(c *gin.Context, source converter.ConversionSource, ath converter.Converter, up converter.UploadConversion) {
	// GC if converting temporary file
	if source.IsLocal {
		defer func() {
			err := os.Remove(source.URI)
			if err != nil {
				log.Printf("%+v", errors.WithStack(err))
			}
		}()
	}

	conf := c.MustGet("config").(Config)
	wq := c.MustGet("queue").(chan<- converter.Work)
	s := c.MustGet("statsd").(*statsd.Client)
	r, ravenOk := c.Get("sentry")

	var conversion converter.Converter
	t := s.NewTiming()
	var work converter.Work
	attempts := 0

StartConversion:
	conversion = ath
	if attempts != 0 {
		cc := cloudconvert.Client{
			BaseURL: conf.CloudConvert.APIUrl,
			APIKey:  conf.CloudConvert.APIKey,
			Timeout: time.Second * time.Duration(conf.WorkerTimeout+5),
		}
		conversion = cloudconvert.CloudConvert{UploadConversion: up, Client: cc}
	}
	work = converter.NewWork(wq, conversion, source)

	select {
	case <-c.Writer.CloseNotify():
		work.Cancel()
	case <-work.Uploaded():
		t.Send("conversion_duration")
		s.Increment("success")
		c.JSON(200, gin.H{"status": "uploaded"})
	case out := <-work.Success():
		t.Send("conversion_duration")
		s.Increment("success")
		c.Data(200, "application/pdf", out)
	case err := <-work.Error():
		// log.Println(err)

		// Log, and stats collection
		if err == converter.ErrConversionTimeout {
			s.Increment("conversion_timeout")
		} else if _, awsError := err.(awserr.Error); awsError {
			s.Increment("s3_upload_error")
			if ravenOk {
				r.(*raven.Client).CaptureError(err, map[string]string{"url": source.GetActualURI()})
			}
		} else {
			s.Increment("conversion_error")
			if ravenOk {
				r.(*raven.Client).CaptureError(err, map[string]string{"url": source.GetActualURI()})
			}
		}

		if attempts == 0 && conf.ConversionFallback {
			s.Increment("cloudconvert")
			log.Println("falling back to CloudConvert...")
			attempts++
			goto StartConversion
		}

		s.Increment("conversion_failed")

		if err == converter.ErrConversionTimeout {
			c.AbortWithError(http.StatusGatewayTimeout, converter.ErrConversionTimeout).SetType(gin.ErrorTypePublic)
			return
		}

		c.Error(err)
	}
}

func convertByCookieHandler(c *gin.Context) {
	//cmd = append(cmd, "-D", "10000", "-P", "A3", "-Z", "0.1")
	s := c.MustGet("statsd").(*statsd.Client)
	r, ravenOk := c.Get("sentry")

	url := c.Query("url")
	if url == "" {
		c.AbortWithError(http.StatusBadRequest, ErrURLInvalid).SetType(gin.ErrorTypePublic)
		s.Increment("invalid_url")
		return
	}

	ext := c.Query("ext")

	source, err := converter.NewConversionSource(url, nil, ext)
	if err != nil {
		s.Increment("conversion_error")
		if ravenOk {
			r.(*raven.Client).CaptureError(err, map[string]string{"url": url})
		}
		c.Error(err)
		return
	}

	uploadConversion := CreateAwsUploader(c)
	athena, err := CreateAthenaConversionFromCookie(c, uploadConversion)
	if err != nil {
		_ = c.AbortWithError(401, err)
		return

	}
	conversionHandler(c, *source, athena, uploadConversion)
}

// convertByURLHandler is the main v1 API handler for converting a HTML to a PDF
// via a GET request. It can either return a JSON string indicating that the
// output of the conversion has been uploaded or it can return the output of
// the conversion to the client (raw bytes).
func convertByURLHandler(c *gin.Context) {
	s := c.MustGet("statsd").(*statsd.Client)
	r, ravenOk := c.Get("sentry")

	url := c.Query("url")
	if url == "" {
		c.AbortWithError(http.StatusBadRequest, ErrURLInvalid).SetType(gin.ErrorTypePublic)
		s.Increment("invalid_url")
		return
	}

	ext := c.Query("ext")

	source, err := converter.NewConversionSource(url, nil, ext)
	if err != nil {
		s.Increment("conversion_error")
		if ravenOk {
			r.(*raven.Client).CaptureError(err, map[string]string{"url": url})
		}
		c.Error(err)
		return
	}

	uploadConversion := CreateAwsUploader(c)
	athena := CreateAthenaConversion(c, uploadConversion)
	conversionHandler(c, *source, athena, uploadConversion)
}

func CreateAwsUploader(c *gin.Context) converter.UploadConversion {
	return converter.UploadConversion{Conversion: converter.Conversion{}, AWSS3: converter.AWSS3{
		Region:       c.Query("aws_region"),
		AccessKey:    c.Query("aws_id"),
		AccessSecret: c.Query("aws_secret"),
		S3Bucket:     c.Query("s3_bucket"),
		S3Key:        c.Query("s3_key"),
		S3Acl:        c.Query("s3_acl"),
	}}

}

func convertByFileHandler(c *gin.Context) {
	s := c.MustGet("statsd").(*statsd.Client)
	r, ravenOk := c.Get("sentry")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, ErrFileInvalid).SetType(gin.ErrorTypePublic)
		s.Increment("invalid_file")
		return
	}

	ext := c.Query("ext")

	source, err := converter.NewConversionSource("", file, ext)
	if err != nil {
		s.Increment("conversion_error")
		if ravenOk {
			r.(*raven.Client).CaptureError(err, map[string]string{"url": header.Filename})
		}
		c.Error(err)
		return
	}

	uploadConversion := CreateAwsUploader(c)
	athena := CreateAthenaConversion(c, uploadConversion)
	conversionHandler(c, *source, athena, uploadConversion)
}
