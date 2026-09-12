package onvifsvc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/0x524a/onvif-go/discovery"
)

const (
	discoveryTimeout       = 5 * time.Second
	maxDiscoveryInterfaces = 16
)

func Discover(ctx context.Context) ([]DiscoveredDevice, error) {
	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout+2*time.Second)
	defer cancel()

	devices, err := discoverAllInterfaces(ctx)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	result := make([]DiscoveredDevice, 0, len(devices))
	for _, d := range devices {
		if d == nil || d.GetDeviceEndpoint() == "" {
			continue
		}
		endpoint, normalizeErr := normalizeHTTPURL(d.GetDeviceEndpoint())
		if normalizeErr != nil {
			continue
		}
		if _, ok := seen[endpoint]; ok {
			continue
		}
		seen[endpoint] = struct{}{}

		name, _ := url.PathUnescape(d.GetName())
		location, _ := url.PathUnescape(d.GetLocation())
		if name == "" {
			name = hostLabel(endpoint)
		}
		result = append(result, DiscoveredDevice{
			Name: name, Location: location, Endpoint: endpoint,
			EndpointRef: d.EndpointRef, Scopes: append([]string(nil), d.Scopes...),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func discoverAllInterfaces(ctx context.Context) ([]*discovery.Device, error) {
	interfaces, listErr := discovery.ListNetworkInterfaces()
	if listErr != nil {
		devices, err := discovery.Discover(ctx, discoveryTimeout)
		if err != nil && !isDiscoveryTimeout(err) {
			return nil, fmt.Errorf("discover ONVIF devices: %w", err)
		}
		return devices, nil
	}

	names := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Up && iface.Multicast {
			names = append(names, iface.Name)
			if len(names) == maxDiscoveryInterfaces {
				break
			}
		}
	}
	if len(names) == 0 {
		names = append(names, "")
	}

	var (
		mu       sync.Mutex
		all      []*discovery.Device
		firstErr error
		success  bool
		wg       sync.WaitGroup
	)
	for _, name := range names {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			devices, err := discovery.DiscoverWithOptions(ctx, discoveryTimeout, &discovery.DiscoverOptions{NetworkInterface: name})
			mu.Lock()
			defer mu.Unlock()
			if err != nil && !isDiscoveryTimeout(err) {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			success = true
			all = append(all, devices...)
		}()
	}
	wg.Wait()
	if !success && firstErr != nil {
		return nil, fmt.Errorf("discover ONVIF devices: %w", firstErr)
	}
	return all, nil
}

func isDiscoveryTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}
