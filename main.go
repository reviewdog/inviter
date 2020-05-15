// Invite contributors to reviewdog organization.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v31/github"
	"golang.org/x/oauth2"
)

var (
	dryRun = flag.Bool("dry-run", true, "dry run")
	// Default is 5 days which is smaller than default 7 days expiration of
	// GitHub invitation.
	within          = flag.Duration("within", 5*24*time.Hour, "process Pull Requests within the given duration")
	targetOrg       = flag.String("org", "reviewdog", "target org name")
	defaultTeamSlug = flag.String("team", "reviewdog", "target default team slug")
	actionTeamSlug  = flag.String("action-team", "actions-maintainer", "target action maintainers team name slug")
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()
	ctx := context.Background()
	cli := githubClient(ctx, os.Getenv("INVITER_GITHUB_API_TOKEN"))
	iv := &inviter{cli: cli, pendings: make(map[string]bool)}
	if err := iv.setupPendings(ctx, *targetOrg); err != nil {
		return err
	}
	repos, err := iv.listRepos(ctx, *targetOrg)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		iv.processRepo(ctx, repo)
	}
	return nil
}

type inviter struct {
	cli      *github.Client
	pendings map[string]bool
}

func (iv *inviter) setupPendings(ctx context.Context, org string) error {
	invitations, _, err := iv.cli.Organizations.ListPendingOrgInvitations(ctx, org, &github.ListOptions{})
	if err != nil {
		return err
	}
	debugJson(invitations)
	for _, invitation := range invitations {
		iv.pendings[invitation.GetLogin()] = true
	}
	return nil
}

func (iv *inviter) listRepos(ctx context.Context, org string) ([]*github.Repository, error) {
	repos, _, err := iv.cli.Repositories.ListByOrg(ctx, org, &github.RepositoryListByOrgOptions{})
	if err != nil {
		return nil, err
	}
	return repos, nil
}

func (iv *inviter) processRepo(ctx context.Context, repo *github.Repository) error {
	var (
		owner    = repo.GetOwner().GetLogin()
		repoName = repo.GetName()
	)
	debug("id=%d: %s/%s\n", repo.GetID(), owner, repoName)

	pulls, _, err := iv.cli.PullRequests.List(ctx, owner, repoName, &github.PullRequestListOptions{
		State:     "closed",
		Sort:      "updated",
		Direction: "desc",
	})
	if err != nil {
		return err
	}
	for _, pull := range pulls {
		if err := iv.processPulls(ctx, owner, repoName, pull); err != nil {
			return err
		}
	}
	return nil
}

func (iv *inviter) processPulls(ctx context.Context, owner, repo string, pull *github.PullRequest) error {
	userName := pull.GetUser().GetLogin()
	link := pull.GetLinks().GetHTML().GetHRef()
	if pull.MergedAt == nil {
		debug("[Not merged] %v: %v\n", userName, link)
		return nil
	}
	if closedAgo := time.Since(pull.GetClosedAt()); closedAgo > *within {
		debug("[Skip too old (%v > %v)] %v: %v\n", closedAgo, *within, userName, link)
		return nil
	}
	if pull.GetUser().GetType() == "Bot" {
		debug("[Skip bot] %v: %v\n", userName, link)
		return nil
	}
	authorAssociation := pull.GetAuthorAssociation()
	switch authorAssociation {
	case "OWNER", "MEMBER", "COLLABORATOR":
		debug("[Skip OWNER/MEMBER/COLLABORATOR] %v: %v\n", userName, link)
		return nil
	}
	debug("[Merged] %v: %v [AuthorAssociation=%s]\n", userName, link, authorAssociation)
	if iv.pendings[userName] {
		debug("[Skip pending member] %v: %v\n", userName, link)
		return nil
	}
	teamSlug := *defaultTeamSlug
	if strings.HasPrefix(repo, "action-") {
		teamSlug = *actionTeamSlug
	}
	if err := iv.invite(ctx, userName, owner, teamSlug, link); err != nil {
		return err
	}
	return nil
}

func (iv *inviter) invite(ctx context.Context, user, org, teamSlug, pr string) error {
	if *dryRun {
		fmt.Printf("[dry-run] Invite %q to https://github.com/orgs/%s/teams/%s based on %s\n",
			user, org, teamSlug, pr)
		return nil
	}
	membership, _, err := iv.cli.Teams.AddTeamMembershipBySlug(
		ctx, org, teamSlug, user, &github.TeamAddTeamMembershipOptions{})
	if err != nil {
		return err
	}
	debugJson(membership)
	fmt.Printf("[state=%s] Invite %q to https://github.com/orgs/%s/teams/%s based on %s\n",
		membership.GetState(), user, org, teamSlug, pr)
	return nil
}

func debugJson(v interface{}) error {
	if os.Getenv("DEBUG_JSON") == "" {
		return nil
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", b)
	return nil
}

func debug(log string, args ...interface{}) {
	if os.Getenv("DEBUG") == "" {
		return
	}
	fmt.Printf(log, args...)
}

func githubClient(ctx context.Context, token string) *github.Client {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{})
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}
