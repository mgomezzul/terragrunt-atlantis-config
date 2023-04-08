package cmd

// Terragrunt doesn't give us an easy way to access all of the Locals from a module
// in an easy to digest way. This file is mostly just follows along how Terragrunt
// parses the `locals` blocks and evaluates their contents.

import (
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"

	"path/filepath"
)

// ResolvedLocals are the parsed result of local values this module cares about
type ResolvedLocals struct {
	// The project branch to use for some project
	BranchProject string

	// The project branch to use for some project
	DeleteSourceBranchOnMergeProject *bool

	// The Atlantis workflow to use for some project
	AtlantisWorkflow string

	// Plan requirements to override the global `--plan-requirements` flag
	PlanRequirements []string

	// Apply requirements to override the global `--apply-requirements` flag
	ApplyRequirements []string

	// Import requirements to override the global `--import-requirements` flag
	ImportRequirements []string

	// Extra dependencies that can be hardcoded in config
	ExtraAtlantisDependencies []string

	// If set, a single module will have autoplan turned to this setting
	AutoPlan *bool

	// If set, a repository lock is applied in this project when plan.
	RepoLocking *bool

	// If set to true, the module will not be included in the output
	Skip *bool

	// Terraform version to use just for this project
	TerraformVersion string

	// If set to true, create Atlantis project
	markedProject *bool
}

// parseHcl uses the HCL2 parser to parse the given string into an HCL file body.
func parseHcl(parser *hclparse.Parser, hcl string, filename string) (file *hcl.File, err error) {
	// The HCL2 parser and especially cty conversions will panic in many types of errors, so we have to recover from
	// those panics here and convert them to normal errors
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.WithStackTrace(config.PanicWhileParsingConfig{RecoveredValue: recovered, ConfigFile: filename})
		}
	}()

	if filepath.Ext(filename) == ".json" {
		file, parseDiagnostics := parser.ParseJSON([]byte(hcl), filename)
		if parseDiagnostics != nil && parseDiagnostics.HasErrors() {
			return nil, parseDiagnostics
		}

		return file, nil
	}

	file, parseDiagnostics := parser.ParseHCL([]byte(hcl), filename)
	if parseDiagnostics != nil && parseDiagnostics.HasErrors() {
		return nil, parseDiagnostics
	}

	return file, nil
}

// Merges in values from a child into a parent set of `local` values
func mergeResolvedLocals(parent ResolvedLocals, child ResolvedLocals) ResolvedLocals {
	if child.BranchProject != "" {
		parent.BranchProject = child.BranchProject
	}

	if child.DeleteSourceBranchOnMergeProject != nil {
		parent.DeleteSourceBranchOnMergeProject = child.DeleteSourceBranchOnMergeProject
	}

	if child.AtlantisWorkflow != "" {
		parent.AtlantisWorkflow = child.AtlantisWorkflow
	}

	if child.TerraformVersion != "" {
		parent.TerraformVersion = child.TerraformVersion
	}

	if child.AutoPlan != nil {
		parent.AutoPlan = child.AutoPlan
	}

	if child.RepoLocking != nil {
		parent.RepoLocking = child.RepoLocking
	}

	if child.Skip != nil {
		parent.Skip = child.Skip
	}

	if child.markedProject != nil {
		parent.markedProject = child.markedProject
	}

	if child.PlanRequirements != nil || len(child.PlanRequirements) > 0 {
		parent.PlanRequirements = child.PlanRequirements
	}

	if child.ApplyRequirements != nil || len(child.ApplyRequirements) > 0 {
		parent.ApplyRequirements = child.ApplyRequirements
	}

	if child.ImportRequirements != nil || len(child.ImportRequirements) > 0 {
		parent.ImportRequirements = child.ImportRequirements
	}

	parent.ExtraAtlantisDependencies = append(parent.ExtraAtlantisDependencies, child.ExtraAtlantisDependencies...)

	return parent
}

// Parses a given file, returning a map of all it's `local` values
func parseLocals(path string, terragruntOptions *options.TerragruntOptions, includeFromChild *config.IncludeConfig) (ResolvedLocals, error) {
	configString, err := util.ReadFileAsString(path)
	if err != nil {
		return ResolvedLocals{}, err
	}

	// Parse the HCL string into an AST body
	parser := hclparse.NewParser()
	file, err := parseHcl(parser, configString, path)
	if err != nil {
		return ResolvedLocals{}, err
	}

	// Decode just the Base blocks. See the function docs for DecodeBaseBlocks for more info on what base blocks are.
	localsAsCty, trackInclude, err := config.DecodeBaseBlocks(terragruntOptions, parser, file, path, includeFromChild, nil)
	if err != nil {
		return ResolvedLocals{}, err
	}

	// Recurse on the parent to merge in the locals from that file
	mergedParentLocals := ResolvedLocals{}
	if trackInclude != nil && includeFromChild == nil {
		for _, includeConfig := range trackInclude.CurrentList {
			parentLocals, _ := parseLocals(includeConfig.Path, terragruntOptions, &includeConfig)
			mergedParentLocals = mergeResolvedLocals(mergedParentLocals, parentLocals)
		}
	}
	childLocals := resolveLocals(*localsAsCty)

	return mergeResolvedLocals(mergedParentLocals, childLocals), nil
}

func resolveLocals(localsAsCty cty.Value) ResolvedLocals {
	resolved := ResolvedLocals{}

	// Return an empty set of locals if no `locals` block was present
	if localsAsCty == cty.NilVal {
		return resolved
	}
	rawLocals := localsAsCty.AsValueMap()

	branchProjectValue, ok := rawLocals["atlantis_branch"]
	if ok {
		resolved.BranchProject = branchProjectValue.AsString()
	}

	deleteSourceBranchOnMergeProjectValue, ok := rawLocals["atlantis_delete_source_branch_on_merge"]
	if ok {
		hasValue := deleteSourceBranchOnMergeProjectValue.True()
		resolved.DeleteSourceBranchOnMergeProject = &hasValue
	}

	workflowValue, ok := rawLocals["atlantis_workflow"]
	if ok {
		resolved.AtlantisWorkflow = workflowValue.AsString()
	}

	versionValue, ok := rawLocals["atlantis_terraform_version"]
	if ok {
		resolved.TerraformVersion = versionValue.AsString()
	}

	autoPlanValue, ok := rawLocals["atlantis_autoplan"]
	if ok {
		hasValue := autoPlanValue.True()
		resolved.AutoPlan = &hasValue
	}

	repoLockingValue, ok := rawLocals["atlantis_repo_locking"]
	if ok {
		hasValue := repoLockingValue.True()
		resolved.RepoLocking = &hasValue
	}

	skipValue, ok := rawLocals["atlantis_skip"]
	if ok {
		hasValue := skipValue.True()
		resolved.Skip = &hasValue
	}

	planReqs, ok := rawLocals["atlantis_plan_requirements"]
	if ok {
		resolved.PlanRequirements = []string{}
		it := planReqs.ElementIterator()
		for it.Next() {
			_, val := it.Element()
			resolved.PlanRequirements = append(resolved.PlanRequirements, val.AsString())
		}
	}

	applyReqs, ok := rawLocals["atlantis_apply_requirements"]
	if ok {
		resolved.ApplyRequirements = []string{}
		it := applyReqs.ElementIterator()
		for it.Next() {
			_, val := it.Element()
			resolved.ApplyRequirements = append(resolved.ApplyRequirements, val.AsString())
		}
	}

	importReqs, ok := rawLocals["atlantis_immport_requirements"]
	if ok {
		resolved.ImportRequirements = []string{}
		it := importReqs.ElementIterator()
		for it.Next() {
			_, val := it.Element()
			resolved.ImportRequirements = append(resolved.ImportRequirements, val.AsString())
		}
	}

	markedProject, ok := rawLocals["atlantis_project"]
	if ok {
		hasValue := markedProject.True()
		resolved.markedProject = &hasValue
	}

	extraDependenciesAsCty, ok := rawLocals["extra_atlantis_dependencies"]
	if ok {
		it := extraDependenciesAsCty.ElementIterator()
		for it.Next() {
			_, val := it.Element()
			resolved.ExtraAtlantisDependencies = append(
				resolved.ExtraAtlantisDependencies,
				filepath.ToSlash(val.AsString()),
			)
		}
	}

	return resolved
}
