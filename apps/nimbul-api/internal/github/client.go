package github

import (
	"context"

	"github.com/google/go-github/v81/github"
	"golang.org/x/oauth2"
)

// NewClient creates a new GitHub client authenticated with the given access token
func NewClient(ctx context.Context, accessToken string) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

// NewClientWithToken creates a new GitHub client directly with a token (no OAuth2 wrapper)
func NewClientWithToken(token string) *github.Client {
	return github.NewClient(nil).WithAuthToken(token)
}
