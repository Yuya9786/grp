package bgp

import (
	"net"
)

func lookupLocalAddr(remote net.IP) (int, net.IP, error) {
	ifList, err := net.Interfaces()
	if err != nil {
		return -1, nil, err
	}
	for _, i := range ifList {
		addrs, err := i.Addrs()
		if err != nil {
			return -1, nil, err
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if v.Contains(remote) {
					return i.Index, v.IP, nil
				}
			}
		}
	}
	return -1, nil, ErrGivenAddrIsNotNeighbor
}
