package discovery

import (
	"github.com/shirou/gopsutil/v3/net"
	"github.com/tfvjr/tap/internal/model"
)

// DiscoverPorts returns a map from PID to the list of ports that PID is
// listening on. It uses gopsutil to retrieve all network connections in a
// cross-platform manner and filters for TCP sockets in the LISTEN state.
func DiscoverPorts() (map[int32][]model.PortBinding, error) {
	conns, err := net.Connections("inet")
	if err != nil {
		return map[int32][]model.PortBinding{}, err
	}

	result := make(map[int32][]model.PortBinding)

	for _, c := range conns {
		if c.Status != "LISTEN" {
			continue
		}

		binding := model.PortBinding{
			Port:     uint16(c.Laddr.Port),
			Protocol: "tcp",
			Address:  c.Laddr.IP,
		}

		result[c.Pid] = append(result[c.Pid], binding)
	}

	return result, nil
}
