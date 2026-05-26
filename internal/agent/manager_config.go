package agent

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"csgclaw/internal/config"
	"csgclaw/internal/runtime/picoclawsandbox"
)

const managerAgentsDirName = "agents"

type localIPDetector struct {
	listInterfaces func() ([]net.Interface, error)
	interfaceAddrs func(iface net.Interface) ([]net.Addr, error)
	dialUDP4       func() (net.Conn, error)
}

var defaultLocalIPDetector = localIPDetector{
	listInterfaces: net.Interfaces,
	interfaceAddrs: func(iface net.Interface) ([]net.Addr, error) {
		return iface.Addrs()
	},
	dialUDP4: func() (net.Conn, error) {
		return net.Dial("udp4", "8.8.8.8:80")
	},
}

func ensureManagerPicoClawConfig(server config.ServerConfig, model config.ModelConfig) (string, error) {
	return ensureAgentPicoClawConfig(ManagerName, "u-manager", server, model)
}

func ensureAgentPicoClawConfig(agentName, botID string, server config.ServerConfig, model config.ModelConfig) (string, error) {
	agentHome, err := agentHomeDir(agentName)
	if err != nil {
		return "", err
	}
	return picoclawsandbox.EnsureConfig(agentHome, botID, server, model, resolveManagerBaseURL)
}

func managerPicoClawRoot() (string, error) {
	return agentPicoClawRoot(ManagerName)
}

func agentWorkspacePicoClawConfigRoot(agentName string) (string, error) {
	return agentPicoClawRoot(agentName)
}

func agentPicoClawRoot(agentName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve host home dir: %w", err)
	}
	return picoclawsandbox.Root(filepath.Join(homeDir, config.AppDirName, managerAgentsDirName, agentName)), nil
}

func renderManagerPicoClawConfig(server config.ServerConfig, model config.ModelConfig) ([]byte, error) {
	return renderAgentPicoClawConfig("u-manager", server, model)
}

func renderAgentPicoClawConfig(botID string, server config.ServerConfig, model config.ModelConfig) ([]byte, error) {
	return picoclawsandbox.RenderConfig(botID, server, model, resolveManagerBaseURL)
}

func picoclawBridgeModelID(modelID string) string {
	return picoclawsandbox.BridgeModelID(modelID)
}

func resolveManagerBaseURL(server config.ServerConfig) string {
	if server.AdvertiseBaseURL != "" {
		return strings.TrimRight(server.AdvertiseBaseURL, "/")
	}
	port := config.ListenPort(server.ListenAddr)
	if ip := localIPv4Resolver(); ip != "" {
		return fmt.Sprintf("http://%s:%s", ip, port)
	}
	return ""
}

func localIPv4() string {
	return defaultLocalIPDetector.localIPv4()
}

func (d localIPDetector) localIPv4() string {
	if ip := d.interfaceIPv4(); ip != "" {
		return ip
	}
	return d.outboundIPv4()
}

func (d localIPDetector) outboundIPv4() string {
	if d.dialUDP4 == nil {
		return ""
	}
	conn, err := d.dialUDP4()
	if err != nil {
		return ""
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return ""
	}
	ip := addr.IP.To4()
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return ""
	}
	return ip.String()
}

func (d localIPDetector) interfaceIPv4() string {
	if d.listInterfaces == nil || d.interfaceAddrs == nil {
		return ""
	}
	ifaces, err := d.listInterfaces()
	if err != nil {
		return ""
	}
	bestFallback := ""
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagPointToPoint != 0 {
			continue
		}
		addrs, err := d.interfaceAddrs(iface)
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipv4FromAddr(addr)
			if ip == "" {
				continue
			}
			parsed := net.ParseIP(ip)
			if parsed == nil {
				continue
			}
			if parsed.IsPrivate() {
				return ip
			}
			if bestFallback == "" {
				bestFallback = ip
			}
		}
	}
	return bestFallback
}

func ipv4FromAddr(addr net.Addr) string {
	switch v := addr.(type) {
	case *net.IPNet:
		ip := v.IP.To4()
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			return ""
		}
		return ip.String()
	case *net.IPAddr:
		ip := v.IP.To4()
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			return ""
		}
		return ip.String()
	default:
		return ""
	}
}

func renderManagerSecurityConfig(server config.ServerConfig, model config.ModelConfig) string {
	return picoclawsandbox.RenderSecurityConfig(server, model)
}
