package request

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/moul/http2curl"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Result struct {
	Job        Job
	Body       []byte
	Error      error
	StatusCode int
}

type Job struct {
	Url             string
	Method          string
	Payload         []byte
	Headers         map[string]string
	RetryIfError    time.Duration
	Type            string
	CurlStr         string
	Info            string
	HeadlessBrowser bool
}

func (j Job) String() string {
	return strings.Join([]string{j.Type, j.Info, j.CurlStr}, " : ")
}

func Request(job *Job, requestTimeout time.Duration) (*Result, error) {
	method := strings.ToUpper(job.Method)
	if job.HeadlessBrowser && method == "GET" {
		logrus.Tracef("headless request: job.HeadlessBrowser: %+v, method: %+v, url: %+v", job.HeadlessBrowser, method, job.Url)
		return HeadlessBrowser(job, requestTimeout)
	}
	logrus.Tracef("go request: job.HeadlessBrowser: %+v, method: %+v, url: %+v, counter: %s", job.HeadlessBrowser, method, job.Url, GoRequesterCounter)
	r, err := Go(job, requestTimeout)
	if err != nil {
		GoRequesterCounter.Failed()
		return nil, errors.Wrapf(err, "counter: %s", GoRequesterCounter)
	}
	GoRequesterCounter.Ok()
	return r, nil
}

func Go(job *Job, requestTimeout time.Duration) (*Result, error) {
	client := http.Client{
		Timeout: requestTimeout,
	}
	req, err := http.NewRequest(job.Method, job.Url, bytes.NewBuffer(job.Payload))
	if err != nil {
		err := errors.Wrap(err, "couldn't create request")
		return nil, err
	}
	for k, v := range job.Headers {
		req.Header.Add(k, v)
	}
	curlCmd, toCurlErr := http2curl.GetCurlCommand(req)
	if toCurlErr != nil {
		logrus.Error(toCurlErr)
	}
	job.CurlStr = curlCmd.String()
	logrus.Tracef("requesting %+v with request: %s", job, curlCmd)
	res, err := client.Do(req)
	if err != nil {
		err = errors.Wrapf(err, "couldn't make request for req %s and timeout: %s", curlCmd, requestTimeout)
		return nil, err
	}
	if res.StatusCode > 399 || res.StatusCode < 200 {
		err := errors.WithStack(fmt.Errorf("not good status code %+v requesting %+v, curl: %s", res.StatusCode, job, curlCmd))
		return nil, err
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		err = errors.Wrap(err, "couldn't read body")
		return nil, err
	}
	if len(b) == 0 || string(b) == "" {
		err := errors.Errorf("empty body in response for requesting %s, status code: %+v", job.CurlStr, res.StatusCode)
		return nil, err
	}
	return &Result{Job: *job, Body: b, StatusCode: res.StatusCode}, nil
}

var HeadlessCounter = Counter{Started: time.Now()}
var GoRequesterCounter = Counter{Started: time.Now()}

func AddState(le *logrus.Entry) *logrus.Entry {
	return le.WithField("go-req", fmt.Sprintf("%s", GoRequesterCounter))
}

func HeadlessBrowser(job *Job, requestTimeout time.Duration) (*Result, error) {
	const chromeDPDir = "/tmp"
	if _, err := os.Stat(chromeDPDir); os.IsNotExist(err) {
		logrus.Infof("directory %+v doesn't exist, creating it...", chromeDPDir)
		if err := os.Mkdir(chromeDPDir, 0777); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	// create chrome instance
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	u := job.Url
	selector := `html`
	logrus.Tracef("requesting %+v selector: %+v", u, selector)
	var html string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(u),
		chromedp.WaitReady(selector),
		chromedp.OuterHTML(selector, &html),
	); err != nil {
		HeadlessCounter.Failed()
		return nil, errors.Wrapf(err, "counter: %s, requestTimeout: %s", HeadlessCounter, requestTimeout)
	}
	HeadlessCounter.Ok()
	logrus.Tracef("headless reqs stats: %s, freq conf: %+v", HeadlessCounter)
	return &Result{Job: *job, Body: []byte(html)}, nil
}
