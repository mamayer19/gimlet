package genericScm

import (
	"context"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/gimlet-io/gimlet-cli/cmd/dashboard/dynamicconfig"
	"github.com/gimlet-io/go-scm/scm"
	"github.com/gimlet-io/go-scm/scm/driver/github"
	"github.com/gimlet-io/go-scm/scm/driver/gitlab"
	"github.com/gimlet-io/go-scm/scm/transport/oauth2"
	"github.com/sirupsen/logrus"
)

type GoScmHelper struct {
	client *scm.Client
}

func NewGoScmHelper(dynamicConfig *dynamicconfig.DynamicConfig, tokenUpdateCallback func(token *scm.Token)) *GoScmHelper {
	var client *scm.Client
	var err error

	if dynamicConfig.IsGithub() {
		client, err = github.New("https://api.github.com")
		if err != nil {
			logrus.WithError(err).
				Fatalln("main: cannot create the GitHub client")
		}
		if dynamicConfig.Github.Debug {
			client.DumpResponse = httputil.DumpResponse
		}

		client.Client = &http.Client{
			Transport: &oauth2.Transport{
				Source: &Refresher{
					ClientID:     dynamicConfig.Github.ClientID,
					ClientSecret: dynamicConfig.Github.ClientSecret,
					Endpoint:     "https://github.com/login/oauth/access_token",
					Source:       oauth2.ContextTokenSource(),
					tokenUpdater: tokenUpdateCallback,
				},
			},
		}
	} else if dynamicConfig.IsGitlab() {
		client, err = gitlab.New(dynamicConfig.ScmURL())
		if err != nil {
			logrus.WithError(err).
				Fatalln("main: cannot create the Gitlab client")
		}
		if dynamicConfig.Gitlab.Debug {
			client.DumpResponse = httputil.DumpResponse
		}

		client.Client = &http.Client{
			Transport: &oauth2.Transport{
				Source: &Refresher{
					ClientID:     dynamicConfig.Gitlab.ClientID,
					ClientSecret: dynamicConfig.Gitlab.ClientSecret,
					Endpoint:     "",
					Source:       oauth2.ContextTokenSource(),
					tokenUpdater: tokenUpdateCallback,
				},
			},
		}
	}

	return &GoScmHelper{
		client: client,
	}
}

func (helper *GoScmHelper) Parse(req *http.Request, fn scm.SecretFunc) (scm.Webhook, error) {
	return helper.client.Webhooks.Parse(req, fn)
}

func (helper *GoScmHelper) UserRepos(accessToken string, refreshToken string, expires time.Time) ([]string, error) {
	var repos []string
	if helper.client == nil {
		return repos, nil
	}

	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: refreshToken,
		Expires: expires,
	})

	opts := scm.ListOptions{Size: 100}
	for {
		scmRepos, meta, err := helper.client.Repositories.List(ctx, opts)
		if err != nil {
			return []string{}, err
		}
		for _, repo := range scmRepos {
			repos = append(repos, repo.Namespace+"/"+repo.Name)
		}

		opts.Page = meta.Page.Next
		opts.URL = meta.Page.NextURL

		if opts.Page == 0 && opts.URL == "" {
			break
		}
	}

	return repos, nil
}

func (helper *GoScmHelper) User(accessToken string, refreshToken string) (*scm.User, error) {
	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: refreshToken,
	})
	user, _, err := helper.client.Users.Find(ctx)
	return user, err
}

func (helper *GoScmHelper) Organizations(accessToken string, refreshToken string) ([]*scm.Organization, error) {
	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: refreshToken,
	})
	organizations, _, err := helper.client.Organizations.List(ctx, scm.ListOptions{
		Size: 50,
	})

	return organizations, err
}

func (helper *GoScmHelper) CreatePR(
	accessToken string,
	repoPath string,
	sourceBranch string,
	targetBranch string,
	title string,
	description string,
) (*scm.PullRequest, *scm.Response, error) {
	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: "",
	})

	newPR := &scm.PullRequestInput{
		Title:  title,
		Body:   description,
		Source: sourceBranch,
		Target: targetBranch,
	}

	pr, res, err := helper.client.PullRequests.Create(ctx, repoPath, newPR)

	return pr, res, err
}

func (helper *GoScmHelper) CreateBranch(accessToken string, repoPath string, branchName string, headSha string) error {
	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: "",
	})

	params := scm.CreateBranch{
		Name: branchName,
		Sha:  headSha,
	}

	_, err := helper.client.Git.CreateBranch(ctx, repoPath, &params)

	return err
}

func (helper *GoScmHelper) Content(accessToken string, repo string, path string, branch string) (string, string, error) {
	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: "",
	})
	content, _, err := helper.client.Contents.Find(
		ctx,
		repo,
		path,
		branch)

	return string(content.Data), string(content.BlobID), err
}

func (helper *GoScmHelper) CreateContent(
	accessToken string,
	repo string,
	path string,
	content []byte,
	branch string,
	message string,
) error {
	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: "",
	})
	_, err := helper.client.Contents.Create(
		ctx,
		repo,
		path,
		&scm.ContentParams{
			Data:    content,
			Branch:  branch,
			Message: message,
			Signature: scm.Signature{
				Name:  "Gimlet",
				Email: "gimlet-dashboard@gimlet.io",
			},
		})

	return err
}

func (helper *GoScmHelper) UpdateContent(
	accessToken string,
	repo string,
	path string,
	content []byte,
	blobID string,
	branch string,
	message string,
) error {
	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: "",
	})
	_, err := helper.client.Contents.Update(
		ctx,
		repo,
		path,
		&scm.ContentParams{
			Data:    content,
			Message: message,
			Branch:  branch,
			BlobID:  blobID,
			Signature: scm.Signature{
				Name:  "Gimlet",
				Email: "gimlet-dashboard@gimlet.io",
			},
		})

	return err
}

// DirectoryContents returns a map of file paths as keys and their file contents in the values
func (helper *GoScmHelper) DirectoryContents(accessToken string, repo string, directoryPath string) (map[string]string, error) {
	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: "",
	})
	directoryFiles, _, err := helper.client.Contents.List(
		ctx,
		repo,
		directoryPath,
		"HEAD",
		scm.ListOptions{
			Size: 50,
		},
	)

	files := map[string]string{}
	for _, file := range directoryFiles {
		files[file.Path] = file.BlobID
	}

	return files, err
}

func (helper *GoScmHelper) RegisterWebhook(
	host string,
	token string,
	webhookSecret string,
	owner string,
	repo string,
) error {
	if strings.Contains(host, "localhost") {
		logrus.Warnf("Not registering webhook for localhost")
		return nil
	}

	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   token,
		Refresh: "",
	})

	hook := &scm.HookInput{
		Name:   "Gimlet",
		Target: host + "/hook",
		Secret: webhookSecret,
		Events: scm.HookEvents{
			Push:   true,
			Status: true,
			Branch: true,
			//CheckRun: true,
		},
	}

	return replaceHook(ctx, helper.client, scm.Join(owner, repo), hook)
}

func (helper *GoScmHelper) ListOpenPRs(accessToken string, repoPath string) ([]*scm.PullRequest, error) {
	ctx := context.WithValue(context.Background(), scm.TokenKey{}, &scm.Token{
		Token:   accessToken,
		Refresh: "",
	})

	prListOptions := scm.PullRequestListOptions{
		Open:   true,
		Closed: false,
	}

	prList, _, err := helper.client.PullRequests.List(ctx, repoPath, prListOptions)

	return prList, err
}
