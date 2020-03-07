package cmd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/spf13/cobra"
)

var config = struct {
	LogPath         string
	HealthCheckPath string

	NodeName    string
	FloatingIPs []net.IP
	HcloudToken string
}{
	LogPath:         getEnv("NOTIFY_LOG_PATH", "/var/run/keepalived.notify.log"),
	HealthCheckPath: getEnv("NOTIFY_HEALTH_CHECK_PATH", "/var/run/keepalived.state"),
}

func getEnvRequired(key string) (string, error) {
	if value, ok := os.LookupEnv(key); ok {
		return value, nil
	}
	return "", fmt.Errorf("required environment variable missing: %s", key)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func matchFloatingIP(f *hcloud.FloatingIP) bool {
	for _, myIP := range config.FloatingIPs {
		if f.Type == hcloud.FloatingIPTypeIPv4 {
			if myIP.Equal(f.IP) {
				return true
			}
		}
		if f.Type == hcloud.FloatingIPTypeIPv6 {
			if f.Network != nil && f.Network.Contains(myIP) {
				return true
			}
		}
	}
	return false
}

func init() {
	log.SetOutput(os.Stderr)
	if config.LogPath == "" {
		return
	}
	logFile, err := os.OpenFile(
		config.LogPath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		log.Printf("Failed to log to file: %s", err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	log.Printf("logging to file %s", config.LogPath)
}

var appName = "hcloud-keepalived-notify"

var RootCmd = &cobra.Command{
	Use: appName,
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(args); err != nil {
			log.Fatal(err)
		}
	},
}

func ipSliceToStrSlice(ips []net.IP) []string {
	out := make([]string, len(ips))
	for pos := range ips {
		out[pos] = ips[pos].String()
	}
	return out
}

func run(args []string) error {
	log.Printf("called with args: %#v", args)

	if len(args) < 3 {
		return fmt.Errorf("Not enough args given")
	}
	var state = args[2]

	var configErrs error

	if hcloudToken, err := getEnvRequired("NOTIFY_HCLOUD_TOKEN"); err != nil {
		configErrs = multierror.Append(configErrs, err)
	} else {
		config.HcloudToken = hcloudToken
	}

	if nodeName, err := getEnvRequired("NOTIFY_NODE_NAME"); err != nil {
		configErrs = multierror.Append(configErrs, err)
	} else {
		config.NodeName = nodeName
	}

	if floatingIPs, err := getEnvRequired("NOTIFY_FLOATING_IPS"); err != nil {
		configErrs = multierror.Append(configErrs, err)
	} else {
		for _, floatingIP := range strings.Split(floatingIPs, ",") {
			ip := net.ParseIP(floatingIP)
			if ip == nil {
				configErrs = multierror.Append(configErrs, fmt.Errorf("invalid IP address: %s", floatingIP))
			} else {
				config.FloatingIPs = append(config.FloatingIPs, ip)
			}
		}
	}

	if configErrs != nil {
		return configErrs
	}

	log.Printf("configured floatingIPs: %#v", ipSliceToStrSlice(config.FloatingIPs))

	// write out statefile
	if err := ioutil.WriteFile(config.HealthCheckPath, []byte(state), 0644); err != nil {
		return fmt.Errorf("failed to health check path: %s", err)
	}
	log.Printf("writing state '%s' to '%s'", state, config.HealthCheckPath)

	if state == "MASTER" {
		hc := hcloud.NewClient(hcloud.WithToken(config.HcloudToken))

		ctx := context.Background()

		// find myself in the hetzner API
		server, _, err := hc.Server.GetByName(ctx, config.NodeName)
		if err != nil {
			return fmt.Errorf("unable to find myself in the api: %s", err)
		}
		log.Printf("found myself in the api id=%d", server.ID)

		// get all floating IPs
		floatingIPs, err := hc.FloatingIP.All(ctx)
		if err != nil {
			return fmt.Errorf("unable to list floating IPs in the api: %s", err)
		}
		log.Printf("found floatingIPs=%#+v", floatingIPs)

		var actions = make([]*hcloud.Action, len(floatingIPs))

		for pos, floatingIP := range floatingIPs {
			if !matchFloatingIP(floatingIP) {
				continue
			}

			if floatingIP.Server != nil && floatingIP.Server.ID == server.ID {
				log.Printf("floatingIP name=%s id=%d already points to me", floatingIP.Name, floatingIP.ID)
				continue
			}

			actions[pos], _, err = hc.FloatingIP.Assign(ctx, floatingIP, server)
			if err != nil {
				return fmt.Errorf(
					"unable to assign floating IP name=%s id=%d to myself id=%d: %w",
					floatingIP.Name,
					floatingIP.ID,
					server.ID,
					err,
				)
			}
			log.Printf("floatingIP pos=%d name=%s id=%d assigned to id=%d", pos, floatingIP.Name, floatingIP.ID, server.ID)
		}
		// TODO: wait for floating IPs to be attached successfully
		// hc.Action.WatchProgress
	}

	// write out statefile
	if err := ioutil.WriteFile(config.HealthCheckPath, []byte(state), 0644); err != nil {
		return fmt.Errorf("failed to health check path: %s", err)
	}
	log.Printf("writing state '%s' to '%s'", state, config.HealthCheckPath)

	return nil
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatalf("problem executing rootCmd: %v", err)
	}
}
