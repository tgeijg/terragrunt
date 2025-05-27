package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	boilerplateoptions "github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/templates"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/gruntwork-io/terragrunt/cli/commands/scaffold"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadTemplate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	terragruntOptions, err := options.NewTerragruntOptionsForTest("scaffold_test_download_template")
	require.NoError(t, err)
	terragruntOptions.Logger = util.NewLogger("") // Use a silent logger for tests to keep output clean

	// Using terragrunt repo itself as it's relevant and likely to be available.
	// v0.53.8 is a specific version to ensure consistency.
	const testRepoBase = "https://github.com/gruntwork-io/terragrunt.git"
	const testRepoRef = "v0.53.8"

	testCases := []struct {
		name                    string
		templateURL             string
		expectedSubDirComponent string // e.g., "test/fixtures/inputs" (POSIX style, as returned by SplitSourceURL)
		expectedFileInFinalPath string // e.g., "main.tf" in the subDir or "README.md" in root
		expectedFileInRepoRoot  string // e.g., "README.md" to check if root is present
		expectError             bool
	}{
		{
			name:                    "WithSubfolder",
			templateURL:             testRepoBase + "//test/fixtures/inputs?ref=" + testRepoRef,
			expectedSubDirComponent: "test/fixtures/inputs", // This is what SplitSourceURL returns
			expectedFileInFinalPath: "main.tf",
			expectedFileInRepoRoot:  "README.md",
			expectError:             false,
		},
		{
			name:                    "WithoutSubfolder",
			templateURL:             testRepoBase + "?ref=" + testRepoRef,
			expectedSubDirComponent: "",
			expectedFileInFinalPath: "README.md",
			expectedFileInRepoRoot:  "", // Not needed to check separately, as final path is root
			expectError:             false,
		},
		{
			name:                    "InvalidSubfolder",
			templateURL:             testRepoBase + "//non-existent-folder-12345?ref=" + testRepoRef,
			expectedSubDirComponent: "non-existent-folder-12345",
			expectedFileInFinalPath: "", // Not applicable
			expectedFileInRepoRoot:  "", // Not applicable
			expectError:             true,
		},
		{
			name:                    "InvalidRepoURL",
			templateURL:             "https://invalid-url-that-does-not-exist.com/foo/bar.git//subfolder?ref=v1.0.0",
			expectedSubDirComponent: "subfolder", // Still set to test logic if somehow download succeeded partially
			expectedFileInFinalPath: "",
			expectedFileInRepoRoot:  "",
			expectError:             true,
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			baseDirForResolvingURL := t.TempDir() // Used if templateURL were a local path

			actualFinalPath, err := scaffold.DownloadTemplate(ctx, terragruntOptions, tc.templateURL, baseDirForResolvingURL)

			if tc.expectError {
				require.Error(t, err, "Expected an error for template URL: %s", tc.templateURL)
				// DownloadTemplate should clean up its own temp dir on error.
				// If actualFinalPath is non-empty, it might be a leftover if cleanup failed, but we can't assume its state.
				return
			}

			require.NoError(t, err, "Did not expect an error for template URL: %s", tc.templateURL)
			require.NotEmpty(t, actualFinalPath, "Expected a non-empty download path")
			require.DirExists(t, actualFinalPath, "Expected actualFinalPath to be an existing directory")

			var downloadedRepoRoot string
			if tc.expectedSubDirComponent == "" {
				downloadedRepoRoot = actualFinalPath
			} else {
				// Determine the downloaded repository root from actualFinalPath and subDirComponent
				// Example: actualFinalPath = /tmp/repo/sub/folder, subDirComponent = sub/folder
				// We need to get /tmp/repo
				// SplitSourceURL returns POSIX paths, so use "/" for splitting subDirComponent
				pathParts := strings.Split(tc.expectedSubDirComponent, "/")
				tempPath := actualFinalPath
				for i := 0; i < len(pathParts); i++ {
					tempPath = filepath.Dir(tempPath)
				}
				downloadedRepoRoot = tempPath
				
				// Verify that actualFinalPath indeed ends with the subfolder component
				// Use filepath.ToSlash for consistent comparison if OS is Windows
				suffixToTest := strings.ReplaceAll(tc.expectedSubDirComponent, "/", string(filepath.Separator))
				require.True(t, strings.HasSuffix(actualFinalPath, suffixToTest),
					"Expected path %s to end with subfolder %s", actualFinalPath, suffixToTest)

			}
			// Defer cleanup of the entire downloaded repository root
			defer func() {
				err := os.RemoveAll(downloadedRepoRoot)
				require.NoError(t, err, "Failed to clean up downloaded repo root: %s", downloadedRepoRoot)
			}()

			// Check for the expected file in the final path (which is actualFinalPath)
			expectedFile := filepath.Join(actualFinalPath, tc.expectedFileInFinalPath)
			require.True(t, util.FileExists(expectedFile), "Expected file not found at final path: %s", expectedFile)

			// If a subfolder was specified, and we have an expected root file, check it too
			if tc.expectedSubDirComponent != "" && tc.expectedFileInRepoRoot != "" {
				expectedRootFile := filepath.Join(downloadedRepoRoot, tc.expectedFileInRepoRoot)
				require.True(t, util.FileExists(expectedRootFile), "Expected file not found in repo root: %s", expectedRootFile)
			}
		})
	}
}

func TestDefaultTemplateVariables(t *testing.T) {
	t.Parallel()

	// set pre-defined variables
	vars := map[string]any{}
	var requiredVariables, optionalVariables []*config.ParsedVariable

	requiredVariables = append(requiredVariables, &config.ParsedVariable{
		Name:                    "required_var_1",
		Description:             "required_var_1 description",
		Type:                    "string",
		DefaultValuePlaceholder: "\"\"",
	})

	optionalVariables = append(optionalVariables, &config.ParsedVariable{
		Name:         "optional_var_2",
		Description:  "optional_ver_2 description",
		Type:         "number",
		DefaultValue: "42",
	})

	vars["requiredVariables"] = requiredVariables
	vars["optionalVariables"] = optionalVariables

	vars["sourceUrl"] = "git::https://github.com/gruntwork-io/terragrunt.git//test/fixtures/inputs?ref=v0.53.8"

	vars["EnableRootInclude"] = false
	vars["RootFileName"] = "root.hcl"

	workDir := t.TempDir()
	templateDir := util.JoinPath(workDir, "template")
	err := os.Mkdir(templateDir, 0755)
	require.NoError(t, err)

	outputDir := util.JoinPath(workDir, "output")
	err = os.Mkdir(outputDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(util.JoinPath(templateDir, "terragrunt.hcl"), []byte(scaffold.DefaultTerragruntTemplate), 0644)
	require.NoError(t, err)

	err = os.WriteFile(util.JoinPath(templateDir, "boilerplate.yml"), []byte(scaffold.DefaultBoilerplateConfig), 0644)
	require.NoError(t, err)

	boilerplateOpts := &boilerplateoptions.BoilerplateOptions{
		OutputFolder:    outputDir,
		OnMissingKey:    boilerplateoptions.DefaultMissingKeyAction,
		OnMissingConfig: boilerplateoptions.DefaultMissingConfigAction,
		Vars:            vars,
		DisableShell:    true,
		DisableHooks:    true,
		NonInteractive:  true,
		TemplateFolder:  templateDir,
	}

	emptyDep := variables.Dependency{}
	err = templates.ProcessTemplate(boilerplateOpts, boilerplateOpts, emptyDep)
	require.NoError(t, err)

	content, err := util.ReadFileAsString(filepath.Join(outputDir, "terragrunt.hcl"))
	require.NoError(t, err)
	require.Contains(t, content, "required_var_1")
	require.Contains(t, content, "optional_var_2")

	// read generated HCL file and check if it is parsed correctly
	opts, err := options.NewTerragruntOptionsForTest(filepath.Join(outputDir, "terragrunt.hcl"))
	require.NoError(t, err)

	cfg, err := config.ReadTerragruntConfig(t.Context(), opts, config.DefaultParserOptions(opts))
	require.NoError(t, err)
	require.NotEmpty(t, cfg.Inputs)
	assert.Len(t, cfg.Inputs, 1)
	_, found := cfg.Inputs["required_var_1"]
	require.True(t, found)
	require.Equal(t, "git::https://github.com/gruntwork-io/terragrunt.git//test/fixtures/inputs?ref=v0.53.8", *cfg.Terraform.Source)
}
