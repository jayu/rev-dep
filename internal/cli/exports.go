package cli

import (
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/model"

	"github.com/spf13/cobra"
)

type JSONOutput = jsonOutput
type JSONRuleResult = jsonRuleResult
type JSONCheckResult = jsonCheckResult
type JSONFixSummary = jsonFixSummary
type JSONCircularDependencyIssue = jsonCircularDependencyIssue
type JSONOrphanFileIssue = jsonOrphanFileIssue
type JSONModuleBoundaryIssue = jsonModuleBoundaryIssue
type JSONUnusedNodeModuleIssue = jsonUnusedNodeModuleIssue
type JSONMissingNodeModuleIssue = jsonMissingNodeModuleIssue
type JSONImportConventionIssue = jsonImportConventionIssue
type JSONUnresolvedImportIssue = jsonUnresolvedImportIssue
type JSONUnusedExportIssue = jsonUnusedExportIssue
type JSONRestrictedDevDepsIssue = jsonRestrictedDevDepsIssue
type JSONRestrictedImportIssue = jsonRestrictedImportIssue
type JSONLocationFields = jsonLocationFields
type JSONLocation = jsonLocation
type FileLocationResolver = fileLocationResolver

func BuildJSONRuleResult(ruleResult config.RuleResult, cwd string, locator *FileLocationResolver) jsonRuleResult {
	return buildJSONRuleResult(ruleResult, cwd, locator)
}

func SetRunConfigRules(rules []string) {
	runConfigRules = rules
}

func SetRunConfigCwd(cwd string) {
	runConfigCwd = cwd
}

func SetRunConfigListAll(value bool) {
	runConfigListAll = value
}

func SetRunConfigFix(value bool) {
	runConfigFix = value
}

func SetRunConfigRecheck(value bool) {
	runConfigRecheck = value
}

func ConfigRunCommand() *cobra.Command {
	return configRunCmd
}

func FormatAndPrintConfigResults(result *config.ConfigProcessingResult, cwd string, listAll bool) {
	formatAndPrintConfigResults(result, cwd, listAll)
}

func InitConfigFileCore(cwd string) (string, []config.Rule, bool, error) {
	result, err := initConfigFileCore(cwd)
	if err != nil {
		return "", nil, false, err
	}
	return result.configPath, result.rules, result.createdForMonorepoSubPackage, nil
}

func PrintRestrictedImportsResolveHint(ruleResult config.RuleResult, cwd string) {
	printRestrictedImportsResolveHint(ruleResult, cwd)
}

func AddSharedFlags(command *cobra.Command) {
	addSharedFlags(command)
}

func GetFollowMonorepoPackagesValue(cmd *cobra.Command) (model.FollowMonorepoPackagesValue, error) {
	return getFollowMonorepoPackagesValue(cmd)
}

func SetFollowMonorepoPackages(values []string) {
	followMonorepoPackages = values
}

func FollowMonorepoPackagesAllSentinel() string {
	return followMonorepoPackagesAllSentinel
}

func SanitizeFlagSentinelInHelpOutput(helpOutput string) string {
	return sanitizeFlagSentinelInHelpOutput(helpOutput)
}

func CircularCmdFn(cwd string, ignoreType bool, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) (int, error) {
	return circularCmdFn(cwd, ignoreType, tsconfigJsonPath, conditionNames, followMonorepoPackages)
}

func ListCwdFilesCmdFn(cwd string, include, exclude []string, listFilesCount bool) error {
	return listCwdFilesCmdFn(cwd, include, exclude, listFilesCount)
}

func EntryPointsCmdFn(cwd string, ignoreType, entryPointsCount, entryPointsDependenciesCount bool, graphExclude, processIgnoredFiles, resultExclude, resultInclude []string, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) error {
	return entryPointsCmdFn(cwd, ignoreType, entryPointsCount, entryPointsDependenciesCount, graphExclude, processIgnoredFiles, resultExclude, resultInclude, tsconfigJsonPath, conditionNames, followMonorepoPackages)
}

func FilesCmdFn(cwd, entryPoint string, ignoreType, filesCount bool, processIgnoredFiles []string, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) error {
	return filesCmdFn(cwd, entryPoint, ignoreType, filesCount, processIgnoredFiles, tsconfigJsonPath, conditionNames, followMonorepoPackages)
}

func ResolveCmdFn(cwd, filePath, moduleName string, entryPoints, graphExclude, processIgnoredFiles []string, ignoreType, resolveAll, resolveCompactSummary bool, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) error {
	return resolveCmdFn(cwd, filePath, moduleName, entryPoints, graphExclude, processIgnoredFiles, ignoreType, resolveAll, resolveCompactSummary, tsconfigJsonPath, conditionNames, followMonorepoPackages)
}

func ImportedByCmdFn(cwd, filePath string, count, listImports bool, processIgnoredFiles []string, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) error {
	return importedByCmdFn(cwd, filePath, count, listImports, processIgnoredFiles, tsconfigJsonPath, conditionNames, followMonorepoPackages)
}

func LinesOfCodeCmdFn(cwd string) error {
	return linesOfCodeCmdFn(cwd)
}

func GetUnresolvedOutput(cwd, tsconfigJson string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue, options *config.UnresolvedImportsOptions, customAssetExtensions []string, processIgnoredFiles []string) (string, error) {
	return getUnresolvedOutput(cwd, tsconfigJson, conditionNames, followMonorepoPackages, options, customAssetExtensions, processIgnoredFiles)
}
