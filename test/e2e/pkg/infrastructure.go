package e2e

import (
    "encoding/json"
    "fmt"
    "net"
    "os"
)

const (
    // Network CIDR configurations
    dockerIPv4CIDR = "10.186.73.0/24"
    dockerIPv6CIDR = "fd80:b10c::/48"
    globalIPv4CIDR = "0.0.0.0/0"
    
    // Baron Chain specific network configurations
    baronNetworkPrefix = "baron"
    defaultIPAllocation = 254 // Maximum hosts in /24 subnet
)

type InfrastructureData struct {
    Provider  string                   `json:"provider"`
    Instances map[string]InstanceData  `json:"instances"`
    Network   string                   `json:"network"`
}

type InstanceData struct {
    IPAddress net.IP `json:"ip_address"`
}

func NewDockerInfrastructureData(m Manifest) (InfrastructureData, error) {
    netAddress := selectNetworkAddress(m.IPv6)
    ipNet, err := parseNetwork(netAddress)
    if err != nil {
        return InfrastructureData{}, err
    }

    instances, err := allocateInstances(m.Nodes, ipNet)
    if err != nil {
        return InfrastructureData{}, fmt.Errorf("failed to allocate instances: %w", err)
    }

    return InfrastructureData{
        Provider:  "docker",
        Instances: instances,
        Network:   netAddress,
    }, nil
}

func InfrastructureDataFromFile(path string) (InfrastructureData, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return InfrastructureData{}, fmt.Errorf("failed to read infrastructure data: %w", err)
    }

    var ifd InfrastructureData
    if err := json.Unmarshal(data, &ifd); err != nil {
        return InfrastructureData{}, fmt.Errorf("failed to parse infrastructure data: %w", err)
    }

    if err := validateInfrastructureData(&ifd); err != nil {
        return InfrastructureData{}, err
    }

    return ifd, nil
}

// Helper functions

func selectNetworkAddress(useIPv6 bool) string {
    if useIPv6 {
        return dockerIPv6CIDR
    }
    return dockerIPv4CIDR
}

func parseNetwork(netAddress string) (*net.IPNet, error) {
    _, ipNet, err := net.ParseCIDR(netAddress)
    if err != nil {
        return nil, fmt.Errorf("invalid network address %q: %w", netAddress, err)
    }
    return ipNet, nil
}

func allocateInstances(nodes map[string]*ManifestNode, ipNet *net.IPNet) (map[string]InstanceData, error) {
    ipGen := newIPGenerator(ipNet)
    instances := make(map[string]InstanceData, len(nodes))

    for name := range nodes {
        ip := ipGen.Next()
        if ip == nil {
            return nil, fmt.Errorf("IP address space exhausted at node %s", name)
        }
        
        instances[name] = InstanceData{
            IPAddress: ip,
        }
    }

    return instances, nil
}

func validateInfrastructureData(ifd *InfrastructureData) error {
    if ifd.Provider == "" {
        return fmt.Errorf("provider cannot be empty")
    }

    if len(ifd.Instances) == 0 {
        return fmt.Errorf("no instances specified")
    }

    if ifd.Network == "" {
        ifd.Network = globalIPv4CIDR
    }

    // Validate network range
    if _, _, err := net.ParseCIDR(ifd.Network); err != nil {
        return fmt.Errorf("invalid network CIDR: %w", err)
    }

    // Validate instance IP addresses
    for name, instance := range ifd.Instances {
        if instance.IPAddress == nil {
            return fmt.Errorf("invalid IP address for instance %s", name)
        }
    }

    return nil
}

// IPGenerator implementation
type ipGenerator struct {
    network *net.IPNet
    nextIP  net.IP
}

func newIPGenerator(network *net.IPNet) *ipGenerator {
    nextIP := make(net.IP, len(network.IP))
    copy(nextIP, network.IP)
    
    gen := &ipGenerator{
        network: network,
        nextIP:  nextIP,
    }
    
    // Skip network and gateway addresses
    gen.Next()
    gen.Next()
    
    return gen
}

func (g *ipGenerator) Next() net.IP {
    if g.nextIP == nil {
        return nil
    }

    ip := make(net.IP, len(g.nextIP))
    copy(ip, g.nextIP)

    // Increment IP address
    for i := len(g.nextIP) - 1; i >= 0; i-- {
        g.nextIP[i]++
        if g.nextIP[i] != 0 {
            break
        }
        if i == 0 {
            g.nextIP = nil
            break
        }
    }

    if g.nextIP != nil && !g.network.Contains(g.nextIP) {
        g.nextIP = nil
    }

    return ip
}
