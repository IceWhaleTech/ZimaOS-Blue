package main

import "github.com/spf13/cobra"

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate shell completion scripts",
}

func newCompletionShellCmd(use, short string, run func(*cobra.Command) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd)
		},
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	completionCmd.AddCommand(
		newCompletionShellCmd("bash", "Generate bash completion script", func(cmd *cobra.Command) error {
			return rootCmd.GenBashCompletionV2(cmd.OutOrStdout(), true)
		}),
		newCompletionShellCmd("zsh", "Generate zsh completion script", func(cmd *cobra.Command) error {
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		}),
		newCompletionShellCmd("fish", "Generate fish completion script", func(cmd *cobra.Command) error {
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		}),
		newCompletionShellCmd("powershell", "Generate PowerShell completion script", func(cmd *cobra.Command) error {
			return rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		}),
	)

	rootCmd.AddCommand(completionCmd)
}
