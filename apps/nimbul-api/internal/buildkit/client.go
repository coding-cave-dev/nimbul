package buildkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	bkclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/filesync"
	"github.com/tonistiigi/fsutil"
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
	Push       bool   // whether to push to registry
}

func (b *Builder) BuildAndPush(ctx context.Context, req BuildRequest) error {
	c, err := bkclient.New(ctx, b.Addr)
	if err != nil {
		return fmt.Errorf("buildkit client: %w", err)
	}
	defer c.Close()

	// Create session
	sess, err := session.NewSession(ctx, "nimbul")
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}

	// Add filesync provider for local directories
	contextFS, err := fsutil.NewFS(req.ContextDir)
	if err != nil {
		return fmt.Errorf("failed to create context fs: %w", err)
	}
	dockerfileFS, err := fsutil.NewFS(req.ContextDir)
	if err != nil {
		return fmt.Errorf("failed to create dockerfile fs: %w", err)
	}
	sess.Allow(filesync.NewFSSyncProvider(filesync.StaticDirSource{
		"context":    contextFS,
		"dockerfile": dockerfileFS,
	}))

	// Add auth provider for registry
	dockerConfig, err := config.Load(b.DockerConfig)
	if err != nil {
		return fmt.Errorf("failed to load docker config: %w", err)
	}
	auth := authprovider.NewDockerAuthProvider(authprovider.DockerAuthProviderConfig{
		ConfigFile: dockerConfig,
	})
	sess.Allow(auth)

	// Run session in background
	go func() {
		_ = sess.Run(ctx, c.Dialer())
	}()
	defer sess.Close()

	// Set Dockerfile path in frontend attrs if specified
	frontendAttrs := map[string]string{}
	if req.Dockerfile != "" && req.Dockerfile != "Dockerfile" {
		frontendAttrs["filename"] = req.Dockerfile
	}

	// Configure exports
	exports := []bkclient.ExportEntry{
		{
			Type: "image",
			Attrs: map[string]string{
				"name": req.ImageRef,
				"push": fmt.Sprintf("%t", req.Push),
			},
		},
	}

	// Solve with status channel for build logs
	statusCh := make(chan *bkclient.SolveStatus)
	statusDone := make(chan struct{})

	// Process status updates in background
	go func() {
		for {
			select {
			case status, ok := <-statusCh:
				if !ok {
					close(statusDone)
					return
				}
				// Print build logs
				for _, vertex := range status.Vertexes {
					if vertex.Error != "" {
						fmt.Fprintf(os.Stderr, "ERROR: %s\n", vertex.Error)
					}
				}
				// Print log output
				for _, log := range status.Logs {
					fmt.Fprintf(os.Stderr, "%s", log.Data)
				}
			case <-ctx.Done():
				close(statusDone)
				return
			}
		}
	}()

	// Use Solve with status channel
	_, err = c.Solve(ctx, nil, bkclient.SolveOpt{
		Frontend:      "dockerfile.v0",
		FrontendAttrs: frontendAttrs,
		Exports:       exports,
		SharedSession: sess,
	}, statusCh)
	if err != nil {
		return fmt.Errorf("solve: %w", err)
	}

	// Wait for status processing to complete
	<-statusDone

	return nil
}
