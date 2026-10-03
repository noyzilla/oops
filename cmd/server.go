package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/noyzilla/oops/internal/webhook"
	"github.com/spf13/cobra"
)

func newServerCmd() *cobra.Command {
	var port string
	var configPath string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Starts the webhook deployment daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if port == "" {
				port = os.Getenv("PORT")
				if port == "" {
					port = "8080"
				}
			}

			http.HandleFunc("/update", webhook.HandleUpdate)
			http.HandleFunc("/deploy", webhook.HandleUpdate)
			http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok\n"))
			})

			log.Printf("Listening for webhooks on :%s (config: %s)\n", port, configPath)
			return http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
		},
	}

	cmd.Flags().StringVarP(&port, "port", "p", "", "Webhook server listening port (default: 8080 or $PORT)")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")

	return cmd
}
