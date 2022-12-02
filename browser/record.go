package browser

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
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
	count := 0
	proxy.Director = func(r *http.Request) {
		director(r)
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
			dst := new(bytes.Buffer)
			if err := json.Compact(dst, body); err == nil {
				body = dst.Bytes()
			}
		}
		if bytes.Contains(body, []byte("\n")) {
			fmt.Printf("request with newline: %s", r.URL.RequestURI())
			return
		}
		if o.Filter != "" && !strings.Contains(r.URL.RequestURI(), o.Filter) {
			return
		}

		fmt.Printf("%s %s %s\n", prefix, r.Method, r.URL.RequestURI())
		var out bytes.Buffer
		if len(body) > 0 {
			if o.IndentJson {
				error := json.Indent(&out, body, "", "  ")
				if error != nil {
					fmt.Println("JSON parse error: ", error)
					return
				}
			} else {
				out = *bytes.NewBuffer(body)
			}
			if o.Output != "" {
				fo, err := os.Create(fmt.Sprintf("%s_%d.json", o.Output, count))
				count++
				if err != nil {
					fmt.Printf("Cannot open file for write: %v", err)
					return
				}
				// close fo on exit and check for its returned error
				defer func() {
					if err := fo.Close(); err != nil {
						fmt.Printf("Error closing file: %v", err)
					}
				}()
				if _, err := fo.Write(out.Bytes()); err != nil {
					panic(err)
				}
			} else {
				fmt.Printf("Payload:\n%s\n\n", &out)
			}
		}
	}

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
