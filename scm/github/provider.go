package github

import (
	"net/http"

	"github.com/google/go-github/v74/github"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

func init() {
	// Register the GitHub provider factory
	scm.Register("github", New)
}

func New(project string) scm.Provider {
	return &Github{
		// TODO: Add support for enterprise GitHub instances (currently SaaS only)
		client:  github.NewClient(http.DefaultClient).WithAuthToken(viper.GetString(config.AuthToken)),
		project: project,
	}
}

type Github struct {
	client  *github.Client
	project string
}
