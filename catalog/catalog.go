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

	// Register catalog lookup functions for utils package
	utils.CatalogProjectLookup = GetProjectForRepo
	utils.CatalogBranchLookup = GetBranchForRepo

	if err := initRepositoryCatalog(ctx, flush); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Could not load repository metadata: %v\n", err)
	}

	// Add locally-configured aliases to the defined labels. Alias members may be given in
	// either bare or project-qualified form, so resolve each one to its canonical name.
	for name, repos := range viper.GetStringMapStringSlice(config.RepoAliases) {
		resolved := make([]string, 0, len(repos))
		for _, repo := range repos {
			resolved = append(resolved, ResolveName(ctx, repo))
		}

		if _, ok := Labels[name]; !ok {
			Labels[name] = mapset.NewSet(resolved...)
		} else {
			Labels[name].Append(resolved...)
		}
	}

	// Add superset label which matches all repositories in the catalog
	Labels[viper.GetString(config.SuperSetLabel)] = mapset.NewSet[string]()

	for name := range Catalog {
		Labels[viper.GetString(config.SuperSetLabel)].Add(name)
	}
}

// projectOrder returns the configured projects in precedence order: the default
// project first, followed by each additional project in order of inclusion.
func projectOrder(ctx context.Context) []string {
	viper := config.Viper(ctx)

	additional := viper.GetStringSlice(config.GitProjects)
	projects := make([]string, 0, len(additional)+1)

	seen := mapset.NewSet[string]()

	for _, project := range append([]string{viper.GetString(config.GitProject)}, additional...) {
		if project == "" || !seen.Add(project) {
			continue
		}

		projects = append(projects, project)
	}

	return projects
}

// ResolveName returns the canonical project-qualified name for the given repository selector.
// Names which already carry a project prefix are normalized and returned as-is, deferring to
// the provided project without consulting the catalog. Bare names are matched against the
// catalog, with collisions resolved in favor of the default project first and then each
// additional configured project in order of inclusion. Names which are unknown to the catalog
// fall back to the default project so they can still be cloned and processed.
func ResolveName(ctx context.Context, name string) string {
	name = strings.Trim(strings.TrimSpace(name), "/")
	if name == "" || name == "." {
		return name
	}

	// An explicit project prefix always wins - use the last two path segments verbatim.
	if parts := strings.Split(name, "/"); len(parts) > 1 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}

	projects := projectOrder(ctx)

	// Prefer a catalog match from a configured project, in precedence order
	for _, project := range projects {
		if _, exists := Catalog[project+"/"+name]; exists {
			return project + "/" + name
		}
	}

	// Otherwise accept a match from any project tracked in the catalog. Iteration order over
	// the catalog is undefined, so pick the lowest-sorting key to keep resolution stable.
	var match string
	for key, repo := range Catalog {
		if repo.Name == name && (match == "" || key < match) {
			match = key
		}
	}

	if match != "" {
		return match
	}

	if len(projects) > 0 {
		return projects[0] + "/" + name
	}

	return name
}

// GetRepository retrieves a repository from the catalog by name. Bare names are resolved
// to their canonical project-qualified form before lookup.
func GetRepository(ctx context.Context, repoName string) (*scm.Repository, bool) {
	repo, exists := Catalog[ResolveName(ctx, repoName)]
	if !exists {
		return nil, false
	}

	return &repo, true
}

// GetProjectForRepo returns the project for a given repository name.
// It checks the catalog first, then falls back to the default project.
func GetProjectForRepo(ctx context.Context, repoName string) string {
	if repo, exists := GetRepository(ctx, repoName); exists && repo.Project != "" {
		return repo.Project
	}

	// Fallback to default project if not in catalog
	return config.Viper(ctx).GetString(config.GitProject)
}

// GetLabelsForRepo returns the names of all labels that include the given repository name.
func GetLabelsForRepo(name string) []string {
	var labels []string

	for label, repos := range Labels {
		if repos.Contains(name) {
			labels = append(labels, label)
		}
	}

	return labels
}

// GetBranchForRepo returns the default branch for a given repository name.
// It checks the catalog first, then falls back to the configured default.
func GetBranchForRepo(ctx context.Context, repoName string) string {
	if repo, exists := GetRepository(ctx, repoName); exists && repo.DefaultBranch != "" {
		return repo.DefaultBranch
	}

	// Fallback to configured default branch if not in catalog
	return config.Viper(ctx).GetString(config.DefaultBranch)
}

// RepositoryList returns the set of repository names matching the given filters.
func RepositoryList(ctx context.Context, filters ...string) mapset.Set[string] {
	// Parse filters into include/exclude/forced sets for set-theory operations
	includeSet, excludeSet, forcedSet := parseLabelFilters(ctx, filters...)

	// Final set is (forced ∪ (include \ exclude)) - forced repos and matched repos which aren't excluded
	return forcedSet.Union(includeSet.Difference(excludeSet))
}

func parseLabelFilters(ctx context.Context, filters ...string) (include, exclude, forced mapset.Set[string]) {
	viper := config.Viper(ctx)
	include, exclude, forced = mapset.NewSet[string](), mapset.NewSet[string](), mapset.NewSet[string]()

	for _, filter := range filters {
		switch {
		case strings.Contains(filter, viper.GetString(config.TokenSkip)):
			addFilterToSet(ctx, filter, exclude)

		case strings.Contains(filter, viper.GetString(config.TokenForced)):
			addFilterToSet(ctx, filter, forced)

		default:
			addFilterToSet(ctx, filter, include)
		}
	}

	// If configured to skip unwanted labels, expand each label to its repos and exclude them
	if viper.GetBool(config.SkipUnwanted) {
		for _, label := range viper.GetStringSlice(config.UnwantedLabels) {
			addFilterToSet(ctx, viper.GetString(config.TokenLabel)+label, exclude)
		}
	}

	// If configured to skip archived repos, add them to the exclude set
	if viper.GetBool(config.SkipArchived) {
		exclude.Append(archivedRepos()...)
	}

	return include, exclude, forced
}

func addFilterToSet(ctx context.Context, filter string, set mapset.Set[string]) {
	replacer := strings.NewReplacer(
		config.Viper(ctx).GetString(config.TokenLabel), "",
		config.Viper(ctx).GetString(config.TokenSkip), "",
		config.Viper(ctx).GetString(config.TokenForced), "",
	)
	filterName := replacer.Replace(filter)

	if strings.Contains(filter, config.Viper(ctx).GetString(config.TokenLabel)) {
		// if it's a label filter, add all repos matching that label to the set
		if labelSet, ok := Labels[filterName]; ok {
			set.Append(labelSet.ToSlice()...)
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: Label '%s' not recognized\n", filterName)
		}
	} else {
		// if it's a repo filter, add the canonical repo name to the set
		set.Add(ResolveName(ctx, filterName))
	}
}

func archivedRepos() []string {
	archived := []string{}

	for name, repo := range Catalog {
		if repo.Archived {
			archived = append(archived, name)
		}
	}

	return archived
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

	if err := os.MkdirAll(filepath.Dir(catalogCachePath(ctx)), 0o750); err != nil {
		return err
	}

	return os.WriteFile(catalogCachePath(ctx), data, 0o600)
}

func fetchRepositoryData(ctx context.Context) error {
	viper := config.Viper(ctx)

	// Fetch repositories from all configured projects, in precedence order
	for _, project := range projectOrder(ctx) {
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
