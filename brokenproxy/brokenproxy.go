package brokenproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/OpenSlides/openslides-performance/client"
)

// Run runs the command.
func (o Options) Run(ctx context.Context, cfg client.Config) error {
	target, err := url.Parse(cfg.Addr())
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("reading response: %w", err)
		}
		_ = body
		r.Body = io.NopCloser(bytes.NewReader(body))
		return nil
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
