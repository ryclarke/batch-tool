// Package catalog provides repository catalog management for caching and querying repository metadata.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	defaultCacheFile = ".batch-tool-cache.json"
	flushTTL         = 5 * time.Second
)

// Catalog contains a cached set of repositories and their metadata from Bitbucket
var Catalog = make(map[string]scm.Repository)

// Labels contains a mapping of label names with the set of repositories matching each label
var Labels = make(map[string]mapset.Set[string])

// Init initializes the repository catalog and label mappings, updating the cache if necessary (based on configured TTL).
func Init(ctx context.Context, flush bool) {
	viper := config.Viper(ctx)

	// Register catalog lookup function for utils package
	utils.CatalogLookup = GetProjectForRepo

	if err := initRepositoryCatalog(ctx, flush); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Could not load repository metadata: %v\n", err)
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
	Labels[viper.GetString(config.SuperSetLabel)] = mapset.NewSet[string]()

	for name := range Catalog {
		Labels[viper.GetString(config.SuperSetLabel)].Add(name)
	}
}

// GetRepository retrieves a repository from the catalog by name.
// It attempts to find the repository using different lookup strategies:
// 1. Direct lookup (if repo contains project/name)
// 2. Lookup with default project prefix
// 3. Search through all catalog entries for matching name
func GetRepository(ctx context.Context, repoName string) (*scm.Repository, bool) {
	viper := config.Viper(ctx)

	// Try direct lookup first (project/name format)
	if repo, exists := Catalog[repoName]; exists {
		return &repo, true
	}

	// Try with default project prefix
	defaultProject := viper.GetString(config.GitProject)
	if defaultProject != "" {
		qualifiedName := defaultProject + "/" + repoName
		if repo, exists := Catalog[qualifiedName]; exists {
			return &repo, true
		}
	}

	// Search through all catalog entries for a name match (in any project)
	for key, repo := range Catalog {
		if repo.Name == repoName || strings.HasSuffix(key, "/"+repoName) {
			return &repo, true
		}
	}

	return nil, false
}

// GetProjectForRepo returns the project for a given repository name.
// It checks the catalog first, then falls back to the default project.
func GetProjectForRepo(ctx context.Context, repoName string) string {
	if repo, exists := GetRepository(ctx, repoName); exists {
		return repo.Project
	}

	// Fallback to default project if not in catalog
	return config.Viper(ctx).GetString(config.GitProject)
}

// RepositoryList returns the set of repository names matching the given filters.
func RepositoryList(ctx context.Context, filters ...string) mapset.Set[string] {
	viper := config.Viper(ctx)

	includeSet := mapset.NewSet[string]()
	excludeSet := mapset.NewSet[string]()
	forcedSet := mapset.NewSet[string]()

	// Exclude unwanted labels by default
	if viper.GetBool(config.SkipUnwanted) {
		for _, unwanted := range viper.GetStringSlice(config.UnwantedLabels) {
			filters = append(filters, unwanted+viper.GetString(config.TokenLabel)+viper.GetString(config.TokenSkip))
		}
	}

	for _, filter := range filters {
		replacer := strings.NewReplacer(
			viper.GetString(config.TokenLabel), "",
			viper.GetString(config.TokenSkip), "",
			viper.GetString(config.TokenForced), "",
		)
		filterName := replacer.Replace(filter)

		if strings.Contains(filter, viper.GetString(config.TokenForced)) {
			// if force token is present, add repo (or label) to forced include set
			if strings.Contains(filter, viper.GetString(config.TokenLabel)) {
				if set, ok := Labels[filterName]; ok {
					forcedSet = forcedSet.Union(set)
				}
			} else {
				forcedSet.Add(filterName)
			}
		} else if strings.Contains(filter, viper.GetString(config.TokenSkip)) {
			// if skip token is present, add repo (or label) to exclude set
			if strings.Contains(filter, viper.GetString(config.TokenLabel)) {
				if set, ok := Labels[filterName]; ok {
					excludeSet = excludeSet.Union(set)
				}
			} else {
				excludeSet.Add(filterName)
			}
		} else {
			// otherwise, add repo (or label) to include set
			if strings.Contains(filter, viper.GetString(config.TokenLabel)) {
				if set, ok := Labels[filterName]; ok {
					includeSet = includeSet.Union(set)
				}
			} else {
				includeSet.Add(filterName)
			}
		}
	}

	// Final set is (forced ∪ (include \ exclude)) - forced repos and matched repos which aren't excluded
	return forcedSet.Union(includeSet.Difference(excludeSet))
}

func initRepositoryCatalog(ctx context.Context, flush bool) error {
	// If catalog is already loaded and not flushing, skip subsequent initialization
	if len(Catalog) > 0 && !flush {
		return nil
	}

	ttl := flushTTL // Use short TTL when flushing to force refetch
	if !flush {
		ttl = config.Viper(ctx).GetDuration(config.CatalogCacheTTL)
	}

	if err := loadCatalogCache(ctx, ttl); err != nil {
		if !flush {
			// Only log error if not flushing - flush implies we want to refetch
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	} else {
		// Cache loaded successfully, no need to refetch
		return nil
	}

	return fetchRepositoryData(ctx)
}

type repositoryCache struct {
	UpdatedAt    time.Time                 `json:"updated_at"`
	Repositories map[string]scm.Repository `json:"repositories"`
}

func loadCatalogCache(ctx context.Context, ttl time.Duration) error {
	file, err := os.Open(catalogCachePath(ctx))
	if err != nil {
		return fmt.Errorf("local cache of repository catalog is missing or invalid - fetching remote info")
	}

	defer file.Close()

	var cached repositoryCache
	if err := json.NewDecoder(file).Decode(&cached); err != nil {
		return err
	}

	if time.Since(cached.UpdatedAt) > ttl {
		return fmt.Errorf("local cache of repository catalog is too old - fetching remote info")
	}

	Catalog = cached.Repositories

	for _, repo := range Catalog {
		// Use project-qualified name for consistent scoping
		repoKey := repo.Project + "/" + repo.Name

		for _, label := range repo.Labels {
			if _, ok := Labels[label]; !ok {
				Labels[label] = mapset.NewSet(repoKey)
			} else {
				Labels[label].Add(repoKey)
			}
		}
	}

	return nil
}

func saveCatalogCache(ctx context.Context) error {
	cache := repositoryCache{
		UpdatedAt:    time.Now().UTC(),
		Repositories: Catalog,
	}

	data, err := json.Marshal(&cache)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(catalogCachePath(ctx)), 0755); err != nil {
		return err
	}

	return os.WriteFile(catalogCachePath(ctx), data, 0644)
}

func fetchRepositoryData(ctx context.Context) error {
	viper := config.Viper(ctx)

	// Build set of all projects to fetch
	projects := mapset.NewSet(viper.GetStringSlice(config.GitProjects)...)
	if defaultProject := viper.GetString(config.GitProject); defaultProject != "" {
		projects.Add(defaultProject)
	}

	// Fetch repositories from all projects
	for project := range projects.Iter() {
		provider := scm.Get(ctx, viper.GetString(config.GitProvider), project)

		repos, err := provider.ListRepositories()
		if err != nil {
			return fmt.Errorf("failed to fetch repositories from project %s: %w", project, err)
		}

		for _, repo := range repos {
			// Always store with project-qualified name for consistency
			repoKey := repo.Project + "/" + repo.Name

			Catalog[repoKey] = *repo

			for _, label := range repo.Labels {
				if _, ok := Labels[label]; !ok {
					Labels[label] = mapset.NewSet(repoKey)
				} else {
					Labels[label].Add(repoKey)
				}
			}
		}
	}

	return saveCatalogCache(ctx)
}

func catalogCachePath(ctx context.Context) string {
	viper := config.Viper(ctx)

	// If a custom path is configured, use it
	if customPath := viper.GetString(config.CatalogCachePath); customPath != "" {
		return customPath
	}

	// Default: store in gitdir/host/.batch-tool-cache.json
	return filepath.Join(viper.GetString(config.GitDirectory), viper.GetString(config.GitHost), defaultCacheFile)
}
