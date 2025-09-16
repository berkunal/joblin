package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script for your shell",
	Long: `To load completions:

Bash:

  $ source <(joblin completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ joblin completion bash > /etc/bash_completion.d/joblin
  # macOS:
  $ joblin completion bash > $(brew --prefix)/etc/bash_completion.d/joblin

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ joblin completion zsh > "${fpath[1]}/_joblin"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ joblin completion fish | source

  # To load completions for each session, execute once:
  $ joblin completion fish > ~/.config/fish/completions/joblin.fish

PowerShell:

  PS> joblin completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> joblin completion powershell > joblin.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

// Completion functions for dynamic values - these need to be accessible from other CLI files

// jobIDCompletion provides completion for job IDs
func jobIDCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if cliContext != nil && cliContext.JobService != nil {
		// Get available job IDs from the job service
		jobs, err := cliContext.JobService.ListJobs(nil)
		if err == nil {
			var jobIDs []string
			for _, job := range jobs {
				if strings.HasPrefix(job.ID, toComplete) {
					// Add job ID with description
					jobIDs = append(jobIDs, job.ID+"\t"+job.Name+" ("+string(job.Status)+")")
				}
			}
			return jobIDs, cobra.ShellCompDirectiveDefault
		}
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// namespaceCompletion provides completion for Kubernetes namespaces
func namespaceCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Return common namespace names
	commonNamespaces := []string{"default", "kube-system", "kube-public", "kube-node-lease"}
	var filtered []string
	for _, ns := range commonNamespaces {
		if strings.HasPrefix(ns, toComplete) {
			filtered = append(filtered, ns+"\tKubernetes namespace")
		}
	}
	return filtered, cobra.ShellCompDirectiveDefault
}

// logLevelCompletion provides completion for log levels
func logLevelCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	logLevels := []string{"debug", "info", "warn", "error"}
	var filtered []string
	for _, level := range logLevels {
		if strings.HasPrefix(level, toComplete) {
			filtered = append(filtered, level+"\tSet log level to "+level)
		}
	}
	return filtered, cobra.ShellCompDirectiveDefault
}

// jobStatusCompletion provides completion for job status values
func jobStatusCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	statuses := []string{"Pending", "Running", "Succeeded", "Failed", "Terminated"}
	var filtered []string
	for _, status := range statuses {
		if strings.HasPrefix(status, toComplete) {
			filtered = append(filtered, status+"\tJobs with "+status+" status")
		}
	}
	return filtered, cobra.ShellCompDirectiveDefault
}

// fileCompletion provides file completion for script paths
func scriptFileCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Complete with .py files
	return nil, cobra.ShellCompDirectiveFilterFileExt
}

func init() {
	// Note: The completion command will be added to rootCmd in root.go
	// This init function is called before rootCmd is fully initialized
}
