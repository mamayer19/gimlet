package customScm

import (
	"context"

	"github.com/gimlet-io/gimlet-cli/cmd/dashboard/dynamicconfig"
	"github.com/gimlet-io/gimlet-cli/pkg/dashboard/model"
	"github.com/gimlet-io/gimlet-cli/pkg/git/customScm/customGithub"
	"github.com/gimlet-io/gimlet-cli/pkg/git/customScm/customGitlab"
)

type CustomGitService interface {
	FetchCommits(owner string, repo string, token string, hashesToFetch []string) ([]*model.Commit, error)
	OrgRepos(installationToken string) ([]string, error)
	GetAppNameAndAppSettingsURLs(installationToken string, ctx context.Context) (string, string, string, error)
	CreateRepository(owner string, name string, loggedInUser string, orgToken string, token string) error
	AddDeployKeyToRepo(owner, repo, token, keyTitle, keyValue string, canWrite bool) error
}

func NewGitService(dynamicConfig *dynamicconfig.DynamicConfig) CustomGitService {
	var gitSvc CustomGitService

	if dynamicConfig.IsGithub() {
		gitSvc = &customGithub.GithubClient{}
	} else if dynamicConfig.IsGitlab() {
		gitSvc = &customGitlab.GitlabClient{
			BaseURL: dynamicConfig.ScmURL(),
		}
	} else {
		gitSvc = &DummyGitService{}
	}
	return gitSvc
}

type DummyGitService struct {
}

func (d *DummyGitService) FetchCommits(owner string, repo string, token string, hashesToFetch []string) ([]*model.Commit, error) {
	return nil, nil
}

func (d *DummyGitService) OrgRepos(installationToken string) ([]string, error) {
	return nil, nil
}

func (d *DummyGitService) GetAppNameAndAppSettingsURLs(installationToken string, ctx context.Context) (string, string, string, error) {
	return "", "", "", nil
}

func (d *DummyGitService) CreateRepository(owner string, name string, loggedInUser string, orgToken string, token string) error {
	return nil
}

func (d *DummyGitService) AddDeployKeyToRepo(owner, repo, token, keyTitle, keyValue string, canWrite bool) error {
	return nil
}
