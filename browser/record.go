package browser

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/OpenSlides/openslides-performance/client"
)

const prefix = "request:"

func (o record) Run(ctx context.Context, cfg client.Config) error {
	target, err := url.Parse(cfg.Addr())
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	proxy.FlushInterval = -1

	director := proxy.Director
	requestCh := make(chan request, 1)
	proxy.Director = func(r *http.Request) {
		director(r)

		if o.Filter != "" && !strings.Contains(r.URL.RequestURI(), o.Filter) {
			return
		}

		var body []byte
		if r.Body != nil {
			body, err = io.ReadAll(r.Body)
			if err != nil {
				// Ignore a body that can not be read.
				body = nil
			}
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
		}

		requestCh <- request{
			method: r.Method,
			uri:    r.URL.RequestURI(),
			body:   body,
		}
	}

	go func() {
		for req := range requestCh {
			if err := o.handleRequest(req); err != nil {
				fmt.Printf("Error handle request: %v\n", err)
			}
		}
	}()

	cert, err := selfSingedCert()
	if err != nil {
		return fmt.Errorf("getting cert: %w", err)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", o.Port),
		Handler: proxy,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}

	wait := make(chan error)
	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(context.Background()); err != nil {
			wait <- fmt.Errorf("HTTP Proxy shutdown: %w", err)
			return
		}
		wait <- nil
	}()

	fmt.Printf("Listen on: '%s'\n", srv.Addr)
	if err := srv.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		return fmt.Errorf("HTTP Proxy failed: %v", err)
	}

	return nil
}

type request struct {
	method string
	uri    string
	body   []byte
}

func (o *record) handleRequest(req request) (err error) {
	if req.body != nil {
		req.body, err = jsonReformat(req.body, o.Files != "")
		if err != nil {
			return fmt.Errorf("reformating body: %w", err)
		}
	}

	if o.Files == "" {
		fmt.Printf("%s %s %s %s\n", prefix, req.method, req.uri, req.body)
		return nil
	}

	fmt.Printf("%s %s %s\n", prefix, req.method, req.uri)

	if len(req.body) == 0 {
		return nil
	}

	f, err := os.Create(fmt.Sprintf("%s_%d.json", o.Files, o.count))
	if err != nil {
		return fmt.Errorf("open output file: %w", err)

	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("closing file: %w", closeErr))
		}
	}()

	o.count++
	if _, err := f.Write(req.body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}

	return nil
}

func jsonReformat(content []byte, indent bool) ([]byte, error) {
	dst := new(bytes.Buffer)
	if indent {
		if err := json.Indent(dst, content, "", "  "); err != nil {
			return nil, fmt.Errorf("indent: %w", err)
		}
		return dst.Bytes(), nil
	}

	if err := json.Compact(dst, content); err != nil {
		return nil, fmt.Errorf("compact: %w", err)
	}

	return dst.Bytes(), nil
}

func selfSingedCert() (tls.Certificate, error) {
	certPem := []byte(`-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`)
	keyPem := []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`)

	return tls.X509KeyPair(certPem, keyPem)
}
