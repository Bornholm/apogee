// Package discovery finds servers on the local network with mDNS (GDD 10.1).
// Many corporate networks block multicast, so clients must always offer a
// manual connection by address as well.
package discovery

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/libp2p/zeroconf/v2"

	"apogee/internal/proto"
)

// Advertisement is a running mDNS announcement. Stop ends it.
type Advertisement struct{ srv *zeroconf.Server }

// Advertise announces a server named name on port.
func Advertise(name string, port int) (*Advertisement, error) {
	srv, err := zeroconf.Register(name, proto.ServiceType, "local.", port,
		[]string{fmt.Sprintf("version=%d", proto.Version)}, nil)
	if err != nil {
		return nil, err
	}
	return &Advertisement{srv}, nil
}

// Stop withdraws the announcement.
func (a *Advertisement) Stop() { a.srv.Shutdown() }

// Server is a server found on the network.
type Server struct {
	Name string
	Addr string // host:port, ready to dial
}

// Browse listens for servers during wait and returns those found, sorted by
// name. An empty result is not an error: multicast may simply be blocked.
func Browse(ctx context.Context, wait time.Duration) ([]Server, error) {
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	entries := make(chan *zeroconf.ServiceEntry, 16)
	found := map[string]Server{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-entries:
				if s, ok := toServer(e); ok {
					found[s.Name+"@"+s.Addr] = s
				}
			}
		}
	}()
	err := zeroconf.Browse(ctx, proto.ServiceType, "local.", entries)
	<-done
	if err != nil && ctx.Err() == nil {
		return nil, err
	}
	out := make([]Server, 0, len(found))
	for _, s := range found {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func toServer(e *zeroconf.ServiceEntry) (Server, bool) {
	if e == nil {
		return Server{}, false
	}
	var ip net.IP
	switch {
	case len(e.AddrIPv4) > 0:
		ip = e.AddrIPv4[0]
	case len(e.AddrIPv6) > 0:
		ip = e.AddrIPv6[0]
	default:
		return Server{}, false
	}
	return Server{Name: unescape(e.Instance), Addr: net.JoinHostPort(ip.String(), fmt.Sprint(e.Port))}, true
}

// unescape removes the DNS escaping of an instance name (`Ciel\ bleu`).
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
