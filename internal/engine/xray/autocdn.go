package xray

import (
	"context"
	"errors"

	"github.com/AMINOPY/pg-core/internal/autocdn"
	"github.com/AMINOPY/pg-core/internal/fronting"
)

// PrepareAutoCDNProfile probes an operator-supplied regional catalog, selects
// the best certificate-valid SNI/edge pair, and applies it to an existing Xray
// profile. It does not invent a server, credentials, transport, path, or Xray
// profile: those remain required inputs. The caller must start the resulting
// profile and verify its actual data path before reporting the route healthy.
func PrepareAutoCDNProfile(
	ctx context.Context,
	profileJSON []byte,
	catalog autocdn.Catalog,
	scanner autocdn.Scanner,
	probe autocdn.ProbeFunc,
) ([]byte, autocdn.Route, error) {
	if len(profileJSON) == 0 {
		return nil, autocdn.Route{}, errors.New("an existing Xray profile is required; CDN/SNI alone cannot create a tunnel")
	}
	routes, err := scanner.SelectCandidates(ctx, catalog, probe)
	if err != nil {
		return nil, autocdn.Route{}, err
	}
	if len(routes) == 0 {
		return nil, autocdn.Route{}, errors.New("auto CDN scanner returned no candidate routes")
	}
	selected := routes[0]
	rewriter := &Adapter{
		Fronting: &fronting.Settings{
			SNI:     selected.SNI,
			EdgeIPs: fronting.IPList{selected.CDNIP},
		},
	}
	prepared, err := rewriter.applyFronting(profileJSON)
	if err != nil {
		return nil, autocdn.Route{}, err
	}
	return prepared, selected, nil
}
