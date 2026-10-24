package transportprofile

import "testing"

func FuzzProfileValidation(f *testing.F) {
	f.Add("https://tunnel.example.com/transport", "tunnel.example.com", "websocket", "/transport", "env:PGCORE_TOKEN")
	f.Add("http://example.com", "example.com", "https", "/", "")
	f.Add("https://192.0.2.1", "192.0.2.1", "grpc", "/t", "raw-secret")
	f.Fuzz(func(t *testing.T, endpoint, serverName, kind, path, authRef string) {
		p := Profile{
			Version:    1,
			Name:       "fuzz",
			Endpoint:   endpoint,
			ServerName: serverName,
			Kind:       Kind(kind),
			Path:       path,
			AuthRef:    authRef,
		}
		_ = p.Validate()
	})
}
