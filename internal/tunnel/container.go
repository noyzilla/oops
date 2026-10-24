package tunnel

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

const (
	// TunnelImage defines the base container image
	TunnelImage = "google/cloud-sdk:alpine"

	// LabelTunnelRemote identifies the remote name for the tunnel
	LabelTunnelRemote = "oops.tunnel.remote"

	// LabelTunnelServices stores serialized port mappings
	LabelTunnelServices = "oops.tunnel.services"

	// LabelTunnelHostname stores the allocated domain
	LabelTunnelHostname = "oops.tunnel.hostname"
)

// BuildTunnelCommand constructs the in-container startup script using standard ssh
func BuildTunnelCommand(cfg *TunnelConfig, mappings []PortMapping) []string {
	var forwards []string
	var aliasCommands []string
	seenAliases := make(map[string]bool)

	for _, m := range mappings {
		bindIP := "0.0.0.0"
		if m.LocalIP != "" && m.LocalIP != "0.0.0.0" {
			bindIP = m.LocalIP
			if !seenAliases[m.LocalIP] {
				seenAliases[m.LocalIP] = true
				aliasCommands = append(aliasCommands, fmt.Sprintf("ip addr add %s/16 dev eth0 2>/dev/null || true", m.LocalIP))
			}
		}
		forwards = append(forwards, fmt.Sprintf("-L %s:%d:%s:%d", bindIP, m.LocalPort, m.ContainerIP, m.RemotePort))
	}

	var sshArgs []string
	sshArgs = append(sshArgs,
		"-N", "-g",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "IgnoreUnknown=UseKeychain",
	)
	sshArgs = append(sshArgs, forwards...)
	sshArgs = append(sshArgs, cfg.Host)

	var aliasSetup string
	if len(aliasCommands) > 0 {
		aliasSetup = strings.Join(aliasCommands, "\n") + "\n"
	}

	script := fmt.Sprintf(`mkdir -p /root/.config/gcloud
if [ -d /root/.config/gcloud-host ]; then
  cp -rf /root/.config/gcloud-host/* /root/.config/gcloud/ 2>/dev/null || true
  cp -rf /root/.config/gcloud-host/.[!.]* /root/.config/gcloud/ 2>/dev/null || true
  chmod -R 700 /root/.config/gcloud 2>/dev/null || true
fi
mkdir -p /root/.ssh
if [ -d /root/.ssh-host ]; then
  cp -rf /root/.ssh-host/* /root/.ssh/ 2>/dev/null || true
  cp -rf /root/.ssh-host/.[!.]* /root/.ssh/ 2>/dev/null || true
  chmod 700 /root/.ssh
  chmod 600 /root/.ssh/* 2>/dev/null || true
  if [ -f /root/.ssh/config ]; then
    sed -i -E '/^[[:space:]]*([Uu]se|[Aa]pple)[Kk]eychain/d' /root/.ssh/config 2>/dev/null || true
  fi
fi
%sexec ssh %s
`, aliasSetup, strings.Join(sshArgs, " "))

	return []string{"sh", "-c", script}
}

// BuildContainerConfig constructs Docker ContainerConfig and HostConfig
func BuildContainerConfig(cfg *TunnelConfig, mappings []PortMapping) (*container.Config, *container.HostConfig, error) {
	var svcsDesc []string
	var dnsMappings []string
	for _, m := range mappings {
		svcsDesc = append(svcsDesc, fmt.Sprintf("%s:%d->%s:%d", m.Service, m.LocalPort, m.ContainerIP, m.RemotePort))
		if m.LocalIP != "" && m.LocalIP != "0.0.0.0" {
			dnsMappings = append(dnsMappings, fmt.Sprintf("%s=%s", cfg.ServiceDomain(m.Service), m.LocalIP))
		}
	}

	dnsHostnames := fmt.Sprintf(".%s.tunnel.oops,tunnel-%s.oops", cfg.RemoteName, cfg.RemoteName)

	labels := map[string]string{
		"oops.dns.hostname": dnsHostnames,
		LabelTunnelRemote:   cfg.RemoteName,
		LabelTunnelServices: strings.Join(svcsDesc, ","),
		LabelTunnelHostname: cfg.Hostname,
		"oops.enable":        "false",
	}
	if len(dnsMappings) > 0 {
		labels["oops.dns.mappings"] = strings.Join(dnsMappings, ",")
	}

	cmd := BuildTunnelCommand(cfg, mappings)

	var mounts []mount.Mount
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		gcloudDir := filepath.Join(home, ".config", "gcloud")
		if fi, err := os.Stat(gcloudDir); err == nil && fi.IsDir() {
			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   gcloudDir,
				Target:   "/root/.config/gcloud-host",
				ReadOnly: true,
			})
		}

		sshDir := filepath.Join(home, ".ssh")
		if fi, err := os.Stat(sshDir); err == nil && fi.IsDir() {
			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   sshDir,
				Target:   "/root/.ssh-host",
				ReadOnly: true,
			})
		}

		// Smart bind mounts for referenced SSH key directories outside ~/.ssh
		extraDirs := ResolveSSHReferencedDirectories(home, cfg.Host)
		for _, dir := range extraDirs {
			mounts = append(mounts, mount.Mount{
				Type:     mount.TypeBind,
				Source:   dir,
				Target:   dir,
				ReadOnly: true,
			})
			if strings.HasPrefix(dir, home) {
				rel := strings.TrimPrefix(dir, home)
				targetRoot := filepath.Clean(filepath.Join("/root", rel))
				if targetRoot != dir {
					mounts = append(mounts, mount.Mount{
						Type:     mount.TypeBind,
						Source:   dir,
						Target:   targetRoot,
						ReadOnly: true,
					})
				}
			}
		}
	}

	env := []string{"CLOUDSDK_CORE_DISABLE_FILE_LOGGING=true"}
	for _, envKey := range []string{
		"CLOUDSDK_CORE_PROJECT",
		"CLOUDSDK_ACTIVE_CONFIG_NAME",
		"CLOUDSDK_COMPUTE_ZONE",
		"CLOUDSDK_COMPUTE_REGION",
	} {
		if val := strings.TrimSpace(os.Getenv(envKey)); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", envKey, val))
		}
	}

	config := &container.Config{
		Image:    TunnelImage,
		Cmd:      cmd,
		Hostname: cfg.Hostname,
		Labels:   labels,
		Env:      env,
	}

	hostConfig := &container.HostConfig{
		CapAdd:      []string{"NET_ADMIN"},
		Mounts:      mounts,
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	return config, hostConfig, nil
}

// ResolveSSHReferencedDirectories inspects OpenSSH configuration on the host for the given remote host
// and returns existing external directories (outside ~/.ssh) that contain referenced IdentityFile, CertificateFile, or Include paths.
func ResolveSSHReferencedDirectories(home, host string) []string {
	if home == "" {
		return nil
	}
	sshDir := filepath.Clean(filepath.Join(home, ".ssh"))
	var candidatePaths []string

	// 1. Query OpenSSH evaluated options for host (handles wildcards and includes)
	if host != "" {
		out, err := exec.Command("ssh", "-G", host).Output()
		if err == nil {
			scanner := bufio.NewScanner(bytes.NewReader(out))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				lower := strings.ToLower(line)
				if strings.HasPrefix(lower, "identityfile ") {
					candidatePaths = append(candidatePaths, strings.TrimSpace(line[len("identityfile "):]))
				} else if strings.HasPrefix(lower, "certificatefile ") {
					candidatePaths = append(candidatePaths, strings.TrimSpace(line[len("certificatefile "):]))
				}
			}
		}
	}

	// 2. Fallback / supplement: Read ~/.ssh/config directly
	configPath := filepath.Join(sshDir, "config")
	if content, err := os.ReadFile(configPath); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(content))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				key := strings.ToLower(fields[0])
				if key == "identityfile" || key == "certificatefile" || key == "include" {
					candidatePaths = append(candidatePaths, fields[1])
				}
			}
		}
	}

	var dirs []string
	seenDirs := make(map[string]bool)

	for _, rawPath := range candidatePaths {
		rawPath = strings.TrimSpace(rawPath)
		if rawPath == "" || rawPath == "none" {
			continue
		}
		// Expand ~
		expanded := rawPath
		if strings.HasPrefix(rawPath, "~/") {
			expanded = filepath.Join(home, rawPath[2:])
		} else if rawPath == "~" {
			expanded = home
		}

		cleaned := filepath.Clean(expanded)
		fi, err := os.Stat(cleaned)
		if err != nil {
			continue
		}

		dir := cleaned
		if !fi.IsDir() {
			dir = filepath.Dir(cleaned)
		}

		// Skip if inside or equal to ~/.ssh (already mounted as ~/.ssh-host)
		if dir == sshDir || strings.HasPrefix(dir, sshDir+string(filepath.Separator)) {
			continue
		}
		// Skip root or system directories
		if dir == "/" || dir == "/etc" || dir == "/var" {
			continue
		}

		if !seenDirs[dir] {
			seenDirs[dir] = true
			dirs = append(dirs, dir)
		}
	}

	return dirs
}
