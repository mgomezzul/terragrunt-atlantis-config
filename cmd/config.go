package cmd

import (
	"io/ioutil"

	log "github.com/sirupsen/logrus"

	"github.com/ghodss/yaml"
)

// Represents an entire config file
type AtlantisConfig struct {
	// Version of the config syntax
	Version int `json:"version"`

	// If Atlantis should merge after finishing `atlantis apply`
	AutoMerge bool `json:"automerge"`

	// If Atlantis should allow automatically deletes the source branch on merge
	DeleteSourceBranchOnMergeGlobal bool `json:"delete_source_branch_on_merge"`

	// If Atlantis should allow plans to occur in parallel
	ParallelPlan bool `json:"parallel_plan"`

	// If Atlantis should allow applies to occur in parallel
	ParallelApply bool `json:"parallel_apply"`

	// The project settings
	Projects []AtlantisProject `json:"projects,omitempty"`

	// Workflows, which are not managed by this library other than
	// the fact that this library preserves any existing workflows
	Workflows interface{} `json:"workflows,omitempty"`

	// Allowed regexp prefixed, Lists the allowed regexp prefixes to use when
	// the --enable-regexp-cmd flag is used. ?omitempty?

}

// Represents an Atlantis Project directory
type AtlantisProject struct {
	// Define project name
	Name string `json:"name,omitempty"`

	// Represents a regex matching projects by the base branch of pull request
	// (the branch the  pull request is getting merged into).
	// Only projects that match the PR's branch will be considered.
	// By default, all branches are matched.
	Branch string `json:"branch,omitempty"`

	// The directory with the terragrunt.hcl file
	Dir string `json:"dir"`

	// Define workspace name
	Workspace string `json:"workspace,omitempty"`

	// Atlantis use ExecutionOrderGroup for sort projects before applying/planning
	ExecutionOrderGroup int `json:"execution_order_group,omitempty"`

	// Atlantis use ExecutionOrderGroup for sort projects before applying/planning
	DeleteSourceBranchOnMergeProject bool `json:"delete_source_branch_on_merge,omitempty"`

	// Atlantis use RepoLocking for get a repository lock in this project when plan.
	RepoLocking bool `json:"repo_locking,omitempty"`

	// Autoplan settings for which plans affect other plans ?omitempty?
	Autoplan AutoplanConfig `json:"autoplan"`

	// The terraform version to use for this project
	TerraformVersion string `json:"terraform_version,omitempty"`

	// We only want to output `plan_requirements` if explicitly stated in a local value
	PlanRequirements *[]string `json:"plan_requirements,omitempty"`

	// We only want to output `apply_requirements` if explicitly stated in a local value
	ApplyRequirements *[]string `json:"apply_requirements,omitempty"`

	// We only want to output `import_requirements` if explicitly stated in a local value
	ImportRequirements *[]string `json:"import_requirements,omitempty"`

	// Define workflow name
	Workflow string `json:"workflow,omitempty"`
}

// Autoplan settings for which plans affect other plans
type AutoplanConfig struct {
	// Relative paths from this modules directory to modules it depends on
	WhenModified []string `json:"when_modified"`

	// If autoplan should be enabled for this dir
	Enabled bool `json:"enabled"`
}

// Checks if an output file already exists. If it does, it reads it
// in to preserve some parts of the old config
func readOldConfig() (*AtlantisConfig, error) {
	// The old file not existing is not an error, as it should not exist on the very first run
	bytes, err := ioutil.ReadFile(outputPath)
	if err != nil {
		log.Info("Could not find an old config file. Starting from scratch")
		return nil, nil
	}

	// The old file being malformed is an actual error
	config := AtlantisConfig{}
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
