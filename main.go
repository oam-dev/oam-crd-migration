package main

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	"github.com/oam-dev/oam-crd-migration/converter"
)

var (
	certFile string
	keyFile  string
	port     int
)

var ConversionWebhookArgs = &cobra.Command{
	Use:  "crd-conversion-webhook",
	Args: cobra.MaximumNArgs(0),
	Run:  transferArgs,
}

func init() {
	ConversionWebhookArgs.Flags().StringVar(&certFile, "tls-cert-file", "",
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
			"after server cert.")
	ConversionWebhookArgs.Flags().StringVar(&keyFile, "tls-private-key-file", "",
		"File containing the default x509 private key matching --tls-cert-file.")
	ConversionWebhookArgs.Flags().IntVar(&port, "port", 443,
		"Secure port that the webhook listens on")
}

// Config contains the server (the webhook) cert and key.
type Config struct {
	CertFile string
	KeyFile  string
}

func main() {
	ConversionWebhookArgs.Execute()
}

func transferArgs(cmd *cobra.Command, args []string) {
	config := Config{CertFile: certFile, KeyFile: keyFile}

	http.HandleFunc("/appconfigconvert", converter.ServeAppConfigConvert)
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		TLSConfig: configTLS(config),
	}
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		panic(err)
	}
}

func configTLS(config Config) *tls.Config {
	sCert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		klog.Error(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
		// TODO: uses mutual tls after we agree on what cert the apiserver should use.
		// ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}
