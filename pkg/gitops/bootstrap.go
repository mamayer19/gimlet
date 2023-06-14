package gitops

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluxcd/flux2/v2/pkg/manifestgen/install"
	"github.com/fluxcd/pkg/ssh"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	helper "github.com/gimlet-io/gimlet-cli/pkg/git/nativeGit"
	"github.com/gimlet-io/gimlet-cli/pkg/gitops/sync"
	"github.com/go-git/go-git/v5"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type ManifestOpts struct {
	ShouldGenerateController           bool
	ShouldGenerateDependencies         bool
	KustomizationPerApp                bool
	Env                                string
	SingleEnv                          bool
	GitopsRepoPath                     string
	ShouldGenerateKustomizationAndRepo bool
	ShouldGenerateDeployKey            bool
	GitopsRepoUrl                      string
	Branch                             string
}

func DefaultManifestOpts() ManifestOpts {
	return ManifestOpts{
		ShouldGenerateController:           true,
		ShouldGenerateDependencies:         true,
		KustomizationPerApp:                false,
		Env:                                "",
		SingleEnv:                          true,
		ShouldGenerateKustomizationAndRepo: true,
		ShouldGenerateDeployKey:            true,
		Branch:                             "main",
	}
}

func GenerateManifests(opts ManifestOpts) (string, string, string, error) {
	var (
		publicKey          string
		gitopsRepoName     string
		gitopsRepoFileName string
		secretFileName     string
	)

	installOpts := install.MakeDefaultOptions()
	installOpts.ManifestFile = "flux.yaml"
	installOpts.TargetPath = opts.Env
	installOpts.Version = "v0.41.2"

	if !opts.SingleEnv && opts.Env == "" {
		return "", "", "", fmt.Errorf("either `--env` or `--single-env` is mandatory")
	}
	if opts.SingleEnv && opts.Env != "" {
		return "", "", "", fmt.Errorf("`--env` and `--single-env` are mutually exclusive")
	}

	if opts.SingleEnv {
		opts.Env = "."
	}

	if opts.ShouldGenerateController {
		installManifest, err := install.Generate(installOpts, "")
		if err != nil {
			return "", "", "", fmt.Errorf("cannot generate installation manifests %s", err)
		}
		installManifest.Path = path.Join(opts.Env, "flux", installOpts.ManifestFile)
		_, err = installManifest.WriteFile(opts.GitopsRepoPath)
		if err != nil {
			return "", "", "", fmt.Errorf("cannot write installation manifests %s", err)
		}
	}

	if opts.ShouldGenerateKustomizationAndRepo {
		host, owner, repoName := ParseRepoURL(opts.GitopsRepoUrl)

		gitopsRepoName = UniqueGitopsRepoName(opts.SingleEnv, owner, repoName, opts.Env)
		gitopsRepoFileName = fmt.Sprintf("gitops-repo-%s.yaml", UniqueName(opts.SingleEnv, owner, repoName, opts.Env))
		secretName := fmt.Sprintf("deploy-key-%s", UniqueName(opts.SingleEnv, owner, repoName, opts.Env))
		secretFileName = secretName + ".yaml"

		fluxPath := filepath.Join(opts.Env, "flux")
		if opts.SingleEnv {
			fluxPath = "flux"
		}
		existingGitopsRepoFileName, existingGitopsRepoMetaName := GitopsRepoFileAndMetaNameFromRepo(opts.GitopsRepoPath, fluxPath)
		if existingGitopsRepoFileName != "" {
			gitopsRepoName = existingGitopsRepoMetaName
			gitopsRepoFileName = existingGitopsRepoFileName
		}

		syncOpts := sync.Options{
			Interval:             15 * time.Second,
			URL:                  fmt.Sprintf("ssh://git@%s/%s/%s", host, owner, repoName),
			Name:                 gitopsRepoName,
			Secret:               secretName,
			Namespace:            "flux-system",
			Branch:               opts.Branch,
			ManifestFile:         gitopsRepoFileName,
			GenerateDependencies: opts.ShouldGenerateDependencies,
		}

		syncOpts.DependenciesPath = opts.Env
		syncOpts.TargetPath = opts.Env
		if opts.SingleEnv {
			syncOpts.DependenciesPath = ""
			syncOpts.TargetPath = ""
		}
		if opts.KustomizationPerApp {
			syncOpts.TargetPath = fluxPath
		}
		syncManifest, err := sync.Generate(syncOpts)
		if err != nil {
			return "", "", "", fmt.Errorf("cannot generate git manifests %s", err)
		}
		syncManifest.Path = path.Join(opts.Env, "flux", syncOpts.ManifestFile)
		_, err = syncManifest.WriteFile(opts.GitopsRepoPath)
		if err != nil {
			return "", "", "", fmt.Errorf("cannot write git manifests %s", err)
		}

		if opts.ShouldGenerateDependencies {
			err = os.MkdirAll(path.Join(opts.GitopsRepoPath, opts.Env, "dependencies"), os.ModePerm)
			if err != nil {
				return "", "", "", fmt.Errorf("cannot create dependencies folder %s", err)
			}
			err = ioutil.WriteFile(path.Join(opts.GitopsRepoPath, opts.Env, "dependencies", ".sourceignore"), []byte(""), os.ModePerm)
			if err != nil {
				return "", "", "", fmt.Errorf("cannot populate dependencies folder %s", err)
			}
		}

		if opts.ShouldGenerateDeployKey {
			pKey, deployKeySecret, err := generateDeployKey(host, secretName)
			publicKey = pKey
			if err != nil {
				return "", "", "", fmt.Errorf("cannot generate deploy key %s", err)
			}
			err = ioutil.WriteFile(path.Join(opts.GitopsRepoPath, opts.Env, "flux", secretFileName), deployKeySecret, os.ModePerm)
			if err != nil {
				return "", "", "", fmt.Errorf("cannot write deploy key %s", err)
			}
		}
	}

	return gitopsRepoFileName, publicKey, secretFileName, nil
}

func UniqueName(singleEnv bool, owner string, repoName string, env string) string {
	if len(owner) > 10 {
		owner = owner[:10]
	}

	uniqueName := fmt.Sprintf("%s-%s-%s",
		strings.ToLower(owner),
		strings.ToLower(repoName),
		strings.ToLower(env),
	)
	if singleEnv {
		uniqueName = fmt.Sprintf("%s-%s",
			strings.ToLower(owner),
			strings.ToLower(repoName),
		)
	}
	return uniqueName
}

func UniqueGitopsRepoName(singleEnv bool, owner string, repoName string, env string) string {
	if len(owner) > 10 {
		owner = owner[:10]
	}
	repoName = strings.TrimPrefix(repoName, "gitops-")

	uniqueName := fmt.Sprintf("%s-%s-%s",
		strings.ToLower(owner),
		strings.ToLower(repoName),
		strings.ToLower(env),
	)
	if singleEnv {
		uniqueName = fmt.Sprintf("%s-%s",
			strings.ToLower(owner),
			strings.ToLower(repoName),
		)
	}
	return uniqueName
}

func GenerateManifestProviderAndAlert(
	env string,
	targetPath string,
	singleEnv bool,
	gitopsRepoPath string,
	gitopsRepoUrl string,
	gimletdUrl string,
	token string,
) (string, error) {
	_, owner, repoName := ParseRepoURL(gitopsRepoUrl)

	kustomizationName := fmt.Sprintf("gitops-repo-%s", UniqueName(singleEnv, owner, repoName, env))
	notificationsName := fmt.Sprintf("notifications-%s", UniqueName(singleEnv, owner, repoName, env))
	notificationsFileName := notificationsName + ".yaml"

	syncManifest, err := sync.GenerateProviderAndAlert(
		env,
		gimletdUrl,
		token,
		targetPath,
		kustomizationName,
		notificationsName,
		notificationsFileName,
	)
	if err != nil {
		return "", fmt.Errorf("cannot generate git manifests %s", err)
	}
	syncManifest.Path = path.Join(targetPath, "flux", notificationsFileName)
	_, err = syncManifest.WriteFile(gitopsRepoPath)
	if err != nil {
		return "", fmt.Errorf("cannot write git manifests %s", err)
	}

	return notificationsFileName, nil
}

func ParseRepoURL(url string) (string, string, string) {
	host := strings.Split(url, ":")[0]
	host = strings.Split(host, "@")[1]

	owner := strings.Split(url, ":")[1]
	owner = strings.Split(owner, "/")[0]

	repo := strings.Split(url, ":")[1]
	repo = strings.Split(repo, "/")[1]
	repo = strings.TrimSuffix(repo, ".git")

	return host, owner, repo
}

func generateDeployKey(host string, name string) (string, []byte, error) {
	privateKeyBytes, publicKeyBytes, err := GenerateEd25519()
	if err != nil {
		return "", []byte(""), err
	}

	hostKey, err := ssh.ScanHostKey(host+":22", 30*time.Second, []string{}, false)
	if err != nil {
		return "", []byte(""), err
	}

	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "flux-system",
		},
		StringData: map[string]string{
			"identity":     string(privateKeyBytes),
			"identity.pub": string(publicKeyBytes),
			"known_hosts":  string(hostKey),
		},
	}

	yamlString, err := yaml.Marshal(secret)
	return string(publicKeyBytes), yamlString, err
}

func GitopsRepoFileAndMetaNameFromRepo(repoPath string, contentPath string) (string, string) {
	var gitRepo sourcev1.GitRepository
	var gitopsRepoFileName string
	repo, err := git.PlainOpen(repoPath)
	if err == git.ErrRepositoryNotExists {
		return "", ""
	}
	branch, _ := helper.HeadBranch(repo)

	files, _ := helper.RemoteFolderOnBranchWithoutCheckout(repo, branch, contentPath)
	for fileName, fileContent := range files {
		if strings.Contains(fileName, "gitops-repo") {
			gitopsRepoFileName = fileName
			err := yaml.Unmarshal([]byte(fileContent), &gitRepo)
			if err != nil {
				logrus.Warnf("couldn't unmarshal %s: %s", fileName, err)
			}
		}
	}
	return gitopsRepoFileName, gitRepo.ObjectMeta.Name
}
