package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

const (
	labelKey      = "~"
	excludeKey    = "!"
	supersetLabel = "all"
)

// Catalog contains a cached set of repositories and their metadata from Bitbucket
var Catalog = make(map[string]scm.Repository)

// Labels contains a mapping of label names with the set of repositories matching each label
var Labels = make(map[string]mapset.Set[string])

func Init() {
	if err := initRepositoryCatalog(); err != nil {
		fmt.Printf("ERROR: Could not load repository metadata: %v", err)
	}

	// Add locally-configured aliases to the defined labels
	for name, repos := range viper.GetStringMapStringSlice(config.RepoAliases) {
		if _, ok := Labels[name]; !ok {
			Labels[name] = mapset.NewSet(repos...)
		} else {
			Labels[name].Append(repos...)
		}
	}

	// Add superset label which matches all repositories in the catalog
	Labels[supersetLabel] = mapset.NewSet[string]()

	for name := range Catalog {
		Labels[supersetLabel].Add(name)
	}
}

func RepositoryList(filters ...string) mapset.Set[string] {
	includeSet := mapset.NewSet[string]()
	excludeSet := mapset.NewSet[string]()

	// Exclude unwanted labels by default
	if viper.GetBool(config.SkipUnwanted) {
		for _, unwanted := range viper.GetStringSlice(config.UnwantedLabels) {
			filters = append(filters, unwanted+labelKey+excludeKey)
		}
	}

	for _, filter := range filters {
		filterName := strings.ReplaceAll(strings.ReplaceAll(filter, labelKey, ""), excludeKey, "")

		if strings.Contains(filter, excludeKey) {
			if strings.Contains(filter, labelKey) {
				if set, ok := Labels[filterName]; ok {
					excludeSet = excludeSet.Union(set)
				}
			} else {
				excludeSet.Add(filterName)
			}
		} else {
			if strings.Contains(filter, labelKey) {
				if set, ok := Labels[filterName]; ok {
					includeSet = includeSet.Union(set)
				}
			} else {
				includeSet.Add(filterName)
			}
		}
	}

	return includeSet.Difference(excludeSet)
}

func initRepositoryCatalog() error {
	if len(Catalog) > 0 {
		return nil
	}

	if err := loadCatalogCache(); err != nil {
		fmt.Print(err.Error())
	} else {
		return nil
	}

	return fetchRepositoryData()
}

type repositoryCache struct {
	UpdatedAt    time.Time                 `json:"updated_at"`
	Repositories map[string]scm.Repository `json:"repositories"`
}

func loadCatalogCache() error {
	file, err := os.Open(catalogCachePath())
	if err != nil {
		return fmt.Errorf("local cache of repository catalog is missing or invalid - fetching remote info")
	}

	defer file.Close()

	var cached repositoryCache
	if err := json.NewDecoder(file).Decode(&cached); err != nil {
		return err
	}

	if time.Since(cached.UpdatedAt) > viper.GetDuration(config.CatalogCacheTTL) {
		return fmt.Errorf("local cache of repository catalog is too old - fetching remote info")
	}

	Catalog = cached.Repositories

	for _, repo := range Catalog {
		for _, label := range repo.Labels {
			if _, ok := Labels[label]; !ok {
				Labels[label] = mapset.NewSet(repo.Name)
			} else {
				Labels[label].Add(repo.Name)
			}
		}
	}

	return nil
}

func saveCatalogCache() error {
	cache := repositoryCache{
		UpdatedAt:    time.Now().UTC(),
		Repositories: Catalog,
	}

	data, err := json.Marshal(&cache)
	if err != nil {
		return err
	}

	return os.WriteFile(catalogCachePath(), data, 0644)
}

func fetchRepositoryData() error {
	provider := scm.Get(viper.GetString(config.GitProvider), viper.GetString(config.GitProject))

	repos, err := provider.ListRepositories()
	if err != nil {
		return fmt.Errorf("failed to fetch repositories from provider: %w", err)
	}

	for _, repo := range repos {
		Catalog[repo.Name] = *repo

		for _, label := range repo.Labels {
			if _, ok := Labels[label]; !ok {
				Labels[label] = mapset.NewSet(repo.Name)
			} else {
				Labels[label].Add(repo.Name)
			}
		}
	}

	return saveCatalogCache()
}

func catalogCachePath() string {
	return filepath.Join(viper.GetString(config.GitDirectory),
		viper.GetString(config.GitHost),
		viper.GetString(config.GitProject),
		viper.GetString(config.CatalogCacheFile),
	)
}
