package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"
)

type GithubClient struct {
	gh *github.Client
}

func NewGithubClient() (*GithubClient, error) {
	key := "GITHUB_TOKEN"
	val, ok := os.LookupEnv(key)
	if !ok {
		fmt.Printf("%s not set\n", key)
		return nil, fmt.Errorf("%s not set", key)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: val},
	)
	tc := oauth2.NewClient(ctx, ts)

	gh := github.NewClient(tc)

	return &GithubClient{
		gh: gh,
	}, nil
}

func (gc *GithubClient) Client() *github.Client {
	return gc.gh
}

func (gc *GithubClient) SearchCommit(hash string) (*github.Commit, error) {
	commits, _, err := gc.Client().Search.Commits(context.Background(), "hash:"+hash, &github.SearchOptions{Sort: "created", Order: "asc"})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	if commits.GetTotal() == 0 {
		return nil, fmt.Errorf("error getting commit: no data found for %s", hash)
	}
	if commits.GetTotal() != 1 {
		return nil, fmt.Errorf("error getting commit: not unique data returned for %s", hash)
	}

	commit := commits.Commits[0].Commit
	return commit, nil
}
