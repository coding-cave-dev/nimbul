package buildkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/cli/cli/config/configfile"
	bkclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
)

type Builder struct {
	Addr         string // e.g. tcp://127.0.0.1:1234 or tcp://buildkitd...:1234
	DockerConfig string // e.g. ~/.docker or /docker (mounted secret)
}

func NewFromEnv() *Builder {
	addr := os.Getenv("BUILDKIT_ADDR")
	if addr == "" {
		addr = "tcp://127.0.0.1:1234"
	}
	dcfg := os.Getenv("DOCKER_CONFIG")
	if dcfg == "" {
		home, _ := os.UserHomeDir()
		dcfg = filepath.Join(home, ".docker")
	}
	return &Builder{Addr: addr, DockerConfig: dcfg}
}

type BuildRequest struct {
	ContextDir string // local path (for local mode)
	Dockerfile string // path to Dockerfile relative to context (e.g., "Dockerfile" or "path/to/Dockerfile")
	ImageRef   string // ghcr.io/coding-cave-dev/nimbul-api:sha-xxxx
	CacheRef   string // ghcr.io/coding-cave-dev/nimbul-api:buildcache
}

func (b *Builder) BuildAndPush(ctx context.Context, req BuildRequest) error {
	c, err := bkclient.New(ctx, b.Addr)
	if err != nil {
		return fmt.Errorf("buildkit client: %w", err)
	}
	defer c.Close()

	// Docker config for registry auth
	configfile := configfile.New(filepath.Join(b.DockerConfig, "config.json"))
	auth := authprovider.NewDockerAuthProvider(authprovider.DockerAuthProviderConfig{
		ConfigFile: configfile,
	})

	// Set Dockerfile path in frontend attrs if specified
	frontendAttrs := map[string]string{}
	if req.Dockerfile != "" && req.Dockerfile != "Dockerfile" {
		frontendAttrs["filename"] = req.Dockerfile
	}

	// Use Solve for standard Dockerfile builds
	_, err = c.Solve(ctx, nil, bkclient.SolveOpt{
		Frontend:      "dockerfile.v0",
		FrontendAttrs: frontendAttrs,
		LocalDirs: map[string]string{
			"context":    req.ContextDir,
			"dockerfile": req.ContextDir,
		},
		Exports: []bkclient.ExportEntry{
			{
				Type: "docker",
				Attrs: map[string]string{
					"name": req.ImageRef,
				},
			},
		},
		Session: []session.Attachable{auth},
	}, nil)
	if err != nil {
		return fmt.Errorf("solve: %w", err)
	}

	return nil
}
