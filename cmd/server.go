package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/docker/docker/client"
	"github.com/noyzilla/oops/internal/dns"
	"github.com/noyzilla/oops/internal/webhook"
	"github.com/spf13/cobra"
)

func newServerCmd() *cobra.Command {
	var port string
	var configPath string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Starts the webhook deployment and DNS discovery daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if port == "" {
				port = os.Getenv("OOPS_PORT")
				if port == "" {
					port = os.Getenv("PORT")
					if port == "" {
						port = "8080"
					}
				}
			}

			// Initialize Docker client and Native DNS Daemon
			dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				log.Printf("[Server] Warning: Failed to connect to Docker daemon for DNS watcher: %v", err)
			} else {
				resolver := dns.NewResolver()
				go dns.WatchDockerEvents(context.Background(), dockerCli, resolver)
				if err := dns.StartDNSDaemon(context.Background(), ":53", resolver, nil); err != nil {
					log.Printf("[Server] Warning: Failed to start DNS daemon: %v", err)
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

	cmd.Flags().StringVarP(&port, "port", "p", "", "Webhook server listening port (default: 8080 or $OOPS_PORT)")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")

	return cmd
}
