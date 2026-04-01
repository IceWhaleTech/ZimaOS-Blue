package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	harnesspkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/spf13/cobra"
)

const defaultHarnessJWTSecretPlaceholder = "change-me-in-production-use-a-strong-secret-key"

var (
	harnessOwnerUserID string

	harnessSelectorEvalSpecID  string
	harnessSelectorTitle       string
	harnessSelectorWait        bool
	harnessSelectorPollEvery   time.Duration
	harnessSelectorTimeout     time.Duration
	harnessSelectorBaselineID  string
	harnessSelectorBaseEvalID  string
	harnessSelectorCandidateID string

	harnessSelectorMinPassRate            string
	harnessSelectorMinCriticalPassRate    string
	harnessSelectorMinRouteAgreementRate  string
	harnessSelectorMaxClarifyRateDelta    string
	harnessSelectorMaxCriticalRegressions string

	harnessExecutionEvalSpecID                  string
	harnessExecutionTitle                       string
	harnessExecutionWait                        bool
	harnessExecutionPollEvery                   time.Duration
	harnessExecutionTimeout                     time.Duration
	harnessExecutionBaselineID                  string
	harnessExecutionBaseEvalID                  string
	harnessExecutionCandidateID                 string
	harnessExecutionMaxPassRateDrop             string
	harnessExecutionMaxCriticalRegressions      string
	harnessExecutionMaxVerificationPassRateDrop string
	harnessExecutionMaxEvidenceBackedRateDrop   string

	harnessBudgetBaselineID                       string
	harnessBudgetBaseEvalID                       string
	harnessBudgetMinMedianSchemaByteReductionRate string
	harnessBudgetMaxMedianLatencyIncreaseRate     string
	harnessBudgetAllowedFinalNativeTools          string

	harnessCutoverCandidateID                      string
	harnessCutoverRequiredConsecutiveRuns          int
	harnessCutoverMaxAssessments                   int
	harnessCutoverSelectorBaselineID               string
	harnessCutoverSelectorBaseEvalID               string
	harnessCutoverExecutionBaselineID              string
	harnessCutoverExecutionBaseEvalID              string
	harnessCutoverBudgetBaselineID                 string
	harnessCutoverBudgetBaseEvalID                 string
	harnessCutoverMinMedianSchemaByteReductionRate string
	harnessCutoverMaxMedianLatencyIncreaseRate     string
	harnessCutoverAllowedFinalNativeTools          string
)

var harnessCmd = &cobra.Command{
	Use:   "harness",
	Short: "Harness evaluation and release-gate tooling",
	Long: `Run Harness datasets, poll eval runs, and apply release gates.

The selector subcommands are the first stable release-gate workflow and are
designed for the curated multilingual selector dataset.`,
}

var harnessSelectorCmd = &cobra.Command{
	Use:   "selector",
	Short: "Selector curated dataset and gate automation",
	Long: `Work with the built-in selector curated dataset.

This command group is intended for non-regression gates while Blue migrates
from tool-heavy schemas to skill routing plus exec.`,
}

var harnessSelectorEnsureCmd = &cobra.Command{
	Use:   "ensure",
	Short: "Ensure the selector curated dataset, version, and eval spec exist",
	Args:  cobra.NoArgs,
	RunE:  runHarnessSelectorEnsureE,
}

var harnessSelectorRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Create a selector curated eval run",
	Args:  cobra.NoArgs,
	RunE:  runHarnessSelectorRunE,
}

var harnessSelectorGateCmd = &cobra.Command{
	Use:   "gate <eval-run-id>",
	Short: "Evaluate selector gate checks for an existing eval run",
	Args:  cobra.ExactArgs(1),
	RunE:  runHarnessSelectorGateE,
}

var harnessSelectorVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Ensure assets, run the curated selector eval, then apply the gate",
	Args:  cobra.NoArgs,
	RunE:  runHarnessSelectorVerifyE,
}

var harnessExecutionCmd = &cobra.Command{
	Use:   "execution",
	Short: "Batch-1 execution equivalence dataset and gate automation",
	Long: `Work with the built-in batch-1 execution equivalence dataset.

This command group wires the execution-side gate for batch 1 migration skills
(` + strings.Join(batch1ExecutionSkillsForHelp(), ", ") + `).`,
}

var harnessExecutionEnsureCmd = &cobra.Command{
	Use:   "ensure",
	Short: "Ensure the batch-1 execution dataset, version, and eval spec exist",
	Args:  cobra.NoArgs,
	RunE:  runHarnessExecutionEnsureE,
}

var harnessExecutionRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Create a batch-1 execution eval run",
	Args:  cobra.NoArgs,
	RunE:  runHarnessExecutionRunE,
}

var harnessExecutionGateCmd = &cobra.Command{
	Use:   "gate <eval-run-id>",
	Short: "Evaluate batch-1 execution equivalence checks for an existing eval run",
	Args:  cobra.ExactArgs(1),
	RunE:  runHarnessExecutionGateE,
}

var harnessExecutionVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Ensure assets, run the batch-1 execution eval, then apply the gate",
	Args:  cobra.NoArgs,
	RunE:  runHarnessExecutionVerifyE,
}

var harnessBudgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Native tool-surface budget gate automation",
	Long: `Evaluate first-turn native tool-surface shrinkage for selector dry-run runs.

This gate compares selector dry-run tool surfaces against a baseline and checks
schema shrinkage, latency drift, and exec-only final native exposure.`,
}

var harnessBudgetGateCmd = &cobra.Command{
	Use:   "gate <eval-run-id>",
	Short: "Evaluate budget gate checks for an existing selector eval run",
	Args:  cobra.ExactArgs(1),
	RunE:  runHarnessBudgetGateE,
}

var harnessCutoverReadinessCmd = &cobra.Command{
	Use:   "cutover-readiness",
	Short: "Assess tool -> skill cutover readiness for a candidate",
	Args:  cobra.NoArgs,
	RunE:  runHarnessCutoverReadinessE,
}

func init() {
	harnessSelectorCmd.PersistentFlags().StringVar(&harnessOwnerUserID, "owner", "", "owner user id for selector curated assets and eval runs (defaults to BLUE_USER_ID or local-cli)")

	harnessSelectorRunCmd.Flags().StringVar(&harnessSelectorEvalSpecID, "eval-spec-id", "", "reuse an existing eval spec instead of ensuring the builtin selector curated spec")
	harnessSelectorRunCmd.Flags().StringVar(&harnessSelectorTitle, "title", "", "eval run title")
	harnessSelectorRunCmd.Flags().StringVar(&harnessSelectorBaseEvalID, "base-eval-run-id", "", "persist a comparison base eval run on the new eval run")
	harnessSelectorRunCmd.Flags().StringVar(&harnessSelectorCandidateID, "candidate-id", "", "candidate id attached to the selector eval run metadata")
	harnessSelectorRunCmd.Flags().BoolVar(&harnessSelectorWait, "wait", true, "poll until the eval run reaches a terminal status")
	harnessSelectorRunCmd.Flags().DurationVar(&harnessSelectorPollEvery, "poll-interval", time.Second, "poll interval while waiting for the eval run")
	harnessSelectorRunCmd.Flags().DurationVar(&harnessSelectorTimeout, "timeout", 10*time.Minute, "maximum time to wait for the eval run")

	harnessSelectorGateCmd.Flags().StringVar(&harnessSelectorBaselineID, "baseline-id", "", "compare against a named baseline")
	harnessSelectorGateCmd.Flags().StringVar(&harnessSelectorBaseEvalID, "base-eval-run-id", "", "compare against a specific eval run")
	bindHarnessSelectorThresholdFlags(harnessSelectorGateCmd)

	harnessSelectorVerifyCmd.Flags().StringVar(&harnessSelectorEvalSpecID, "eval-spec-id", "", "reuse an existing eval spec instead of ensuring the builtin selector curated spec")
	harnessSelectorVerifyCmd.Flags().StringVar(&harnessSelectorTitle, "title", "", "eval run title")
	harnessSelectorVerifyCmd.Flags().StringVar(&harnessSelectorBaselineID, "baseline-id", "", "compare against a named baseline")
	harnessSelectorVerifyCmd.Flags().StringVar(&harnessSelectorBaseEvalID, "base-eval-run-id", "", "compare against a specific eval run")
	harnessSelectorVerifyCmd.Flags().StringVar(&harnessSelectorCandidateID, "candidate-id", "", "candidate id attached to the selector eval run metadata")
	harnessSelectorVerifyCmd.Flags().DurationVar(&harnessSelectorPollEvery, "poll-interval", time.Second, "poll interval while waiting for the eval run")
	harnessSelectorVerifyCmd.Flags().DurationVar(&harnessSelectorTimeout, "timeout", 10*time.Minute, "maximum time to wait for the eval run")
	bindHarnessSelectorThresholdFlags(harnessSelectorVerifyCmd)

	harnessSelectorCmd.AddCommand(harnessSelectorEnsureCmd)
	harnessSelectorCmd.AddCommand(harnessSelectorRunCmd)
	harnessSelectorCmd.AddCommand(harnessSelectorGateCmd)
	harnessSelectorCmd.AddCommand(harnessSelectorVerifyCmd)

	harnessExecutionCmd.PersistentFlags().StringVar(&harnessOwnerUserID, "owner", "", "owner user id for batch-1 execution assets and eval runs (defaults to BLUE_USER_ID or local-cli)")

	harnessExecutionRunCmd.Flags().StringVar(&harnessExecutionEvalSpecID, "eval-spec-id", "", "reuse an existing eval spec instead of ensuring the builtin batch-1 execution spec")
	harnessExecutionRunCmd.Flags().StringVar(&harnessExecutionTitle, "title", "", "eval run title")
	harnessExecutionRunCmd.Flags().StringVar(&harnessExecutionBaseEvalID, "base-eval-run-id", "", "persist a comparison base eval run on the new eval run")
	harnessExecutionRunCmd.Flags().StringVar(&harnessExecutionCandidateID, "candidate-id", "", "candidate id attached to the execution eval run metadata")
	harnessExecutionRunCmd.Flags().BoolVar(&harnessExecutionWait, "wait", true, "poll until the eval run reaches a terminal status")
	harnessExecutionRunCmd.Flags().DurationVar(&harnessExecutionPollEvery, "poll-interval", time.Second, "poll interval while waiting for the eval run")
	harnessExecutionRunCmd.Flags().DurationVar(&harnessExecutionTimeout, "timeout", 10*time.Minute, "maximum time to wait for the eval run")

	harnessExecutionGateCmd.Flags().StringVar(&harnessExecutionBaselineID, "baseline-id", "", "compare against a named baseline")
	harnessExecutionGateCmd.Flags().StringVar(&harnessExecutionBaseEvalID, "base-eval-run-id", "", "compare against a specific eval run")
	bindHarnessExecutionThresholdFlags(harnessExecutionGateCmd)

	harnessExecutionVerifyCmd.Flags().StringVar(&harnessExecutionEvalSpecID, "eval-spec-id", "", "reuse an existing eval spec instead of ensuring the builtin batch-1 execution spec")
	harnessExecutionVerifyCmd.Flags().StringVar(&harnessExecutionTitle, "title", "", "eval run title")
	harnessExecutionVerifyCmd.Flags().StringVar(&harnessExecutionBaselineID, "baseline-id", "", "compare against a named baseline")
	harnessExecutionVerifyCmd.Flags().StringVar(&harnessExecutionBaseEvalID, "base-eval-run-id", "", "compare against a specific eval run")
	harnessExecutionVerifyCmd.Flags().StringVar(&harnessExecutionCandidateID, "candidate-id", "", "candidate id attached to the execution eval run metadata")
	harnessExecutionVerifyCmd.Flags().DurationVar(&harnessExecutionPollEvery, "poll-interval", time.Second, "poll interval while waiting for the eval run")
	harnessExecutionVerifyCmd.Flags().DurationVar(&harnessExecutionTimeout, "timeout", 10*time.Minute, "maximum time to wait for the eval run")
	bindHarnessExecutionThresholdFlags(harnessExecutionVerifyCmd)

	harnessExecutionCmd.AddCommand(harnessExecutionEnsureCmd)
	harnessExecutionCmd.AddCommand(harnessExecutionRunCmd)
	harnessExecutionCmd.AddCommand(harnessExecutionGateCmd)
	harnessExecutionCmd.AddCommand(harnessExecutionVerifyCmd)

	harnessBudgetCmd.PersistentFlags().StringVar(&harnessOwnerUserID, "owner", "", "owner user id for budget gate requests (defaults to BLUE_USER_ID or local-cli)")
	harnessBudgetGateCmd.Flags().StringVar(&harnessBudgetBaselineID, "baseline-id", "", "compare against a named baseline")
	harnessBudgetGateCmd.Flags().StringVar(&harnessBudgetBaseEvalID, "base-eval-run-id", "", "compare against a specific eval run")
	bindHarnessBudgetThresholdFlags(harnessBudgetGateCmd)
	harnessBudgetCmd.AddCommand(harnessBudgetGateCmd)

	harnessCmd.AddCommand(harnessSelectorCmd)
	harnessCmd.AddCommand(harnessExecutionCmd)
	harnessCmd.AddCommand(harnessBudgetCmd)
	harnessCutoverReadinessCmd.Flags().StringVar(&harnessOwnerUserID, "owner", "", "owner user id for candidate readiness checks (defaults to BLUE_USER_ID or local-cli)")
	harnessCutoverReadinessCmd.Flags().StringVar(&harnessCutoverCandidateID, "candidate-id", "", "candidate id to evaluate; if omitted the newest shared candidate_id is used")
	harnessCutoverReadinessCmd.Flags().IntVar(&harnessCutoverRequiredConsecutiveRuns, "required-consecutive-runs", 2, "number of consecutive green runs required per lane")
	harnessCutoverReadinessCmd.Flags().IntVar(&harnessCutoverMaxAssessments, "max-assessments", 5, "maximum per-lane assessments returned in the report")
	harnessCutoverReadinessCmd.Flags().StringVar(&harnessCutoverSelectorBaselineID, "selector-baseline-id", "", "selector baseline id used for readiness gating")
	harnessCutoverReadinessCmd.Flags().StringVar(&harnessCutoverSelectorBaseEvalID, "selector-base-eval-run-id", "", "selector base eval run id used for readiness gating")
	harnessCutoverReadinessCmd.Flags().StringVar(&harnessCutoverExecutionBaselineID, "execution-baseline-id", "", "execution baseline id used for readiness gating")
	harnessCutoverReadinessCmd.Flags().StringVar(&harnessCutoverExecutionBaseEvalID, "execution-base-eval-run-id", "", "execution base eval run id used for readiness gating")
	harnessCutoverReadinessCmd.Flags().StringVar(&harnessCutoverBudgetBaselineID, "budget-baseline-id", "", "budget baseline id used for readiness gating")
	harnessCutoverReadinessCmd.Flags().StringVar(&harnessCutoverBudgetBaseEvalID, "budget-base-eval-run-id", "", "budget base eval run id used for readiness gating")
	bindHarnessCutoverBudgetThresholdFlags(harnessCutoverReadinessCmd)
	harnessCmd.AddCommand(harnessCutoverReadinessCmd)
	rootCmd.AddCommand(harnessCmd)
}

func bindHarnessSelectorThresholdFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessSelectorMinPassRate, "min-pass-rate", "", "override minimum curated pass rate, for example 0.98")
	cmd.Flags().StringVar(&harnessSelectorMinCriticalPassRate, "min-critical-pass-rate", "", "override minimum critical curated pass rate, for example 1.0")
	cmd.Flags().StringVar(&harnessSelectorMinRouteAgreementRate, "min-route-agreement-rate", "", "optionally require agreement with the baseline route distribution")
	cmd.Flags().StringVar(&harnessSelectorMaxClarifyRateDelta, "max-clarify-rate-delta", "", "optionally cap clarify-rate delta relative to the baseline")
	cmd.Flags().StringVar(&harnessSelectorMaxCriticalRegressions, "max-critical-regressions", "", "optionally cap the number of critical regressions")
}

func bindHarnessExecutionThresholdFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessExecutionMaxPassRateDrop, "max-pass-rate-drop", "", "override maximum allowed pass-rate drop, for example 0.01")
	cmd.Flags().StringVar(&harnessExecutionMaxCriticalRegressions, "max-critical-regressions", "", "override maximum allowed critical regressions, for example 0")
	cmd.Flags().StringVar(&harnessExecutionMaxVerificationPassRateDrop, "max-verification-pass-rate-drop", "", "optionally cap verification-pass-rate drop relative to the baseline")
	cmd.Flags().StringVar(&harnessExecutionMaxEvidenceBackedRateDrop, "max-evidence-backed-pass-rate-drop", "", "optionally cap evidence-backed pass-rate drop relative to the baseline")
}

func bindHarnessBudgetThresholdFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessBudgetMinMedianSchemaByteReductionRate, "min-median-schema-byte-reduction-rate", "", "override minimum median schema-byte reduction rate, for example 0.80")
	cmd.Flags().StringVar(&harnessBudgetMaxMedianLatencyIncreaseRate, "max-median-latency-increase-rate", "", "override maximum median latency increase rate, for example 0.10")
	cmd.Flags().StringVar(&harnessBudgetAllowedFinalNativeTools, "allowed-final-native-tools", "", "comma-separated allowed final native tools, for example exec")
}

func bindHarnessCutoverBudgetThresholdFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessCutoverMinMedianSchemaByteReductionRate, "min-median-schema-byte-reduction-rate", "", "override minimum median schema-byte reduction rate for cutover readiness")
	cmd.Flags().StringVar(&harnessCutoverMaxMedianLatencyIncreaseRate, "max-median-latency-increase-rate", "", "override maximum median latency increase rate for cutover readiness")
	cmd.Flags().StringVar(&harnessCutoverAllowedFinalNativeTools, "allowed-final-native-tools", "", "comma-separated allowed final native tools for cutover readiness, for example exec")
}

type selectorRunOutput struct {
	Assets  *harnesspkg.SelectorCuratedAssets `json:"assets,omitempty"`
	EvalRun *harnesspkg.EvalRun               `json:"eval_run,omitempty"`
}

type selectorVerifyOutput struct {
	Assets       *harnesspkg.SelectorCuratedAssets `json:"assets,omitempty"`
	EvalRun      *harnesspkg.EvalRun               `json:"eval_run,omitempty"`
	SelectorGate *harnesspkg.SelectorGateReport    `json:"selector_gate,omitempty"`
}

type executionRunOutput struct {
	Assets  *harnesspkg.Batch1ExecutionAssets `json:"assets,omitempty"`
	EvalRun *harnesspkg.EvalRun               `json:"eval_run,omitempty"`
}

type executionVerifyOutput struct {
	Assets        *harnesspkg.Batch1ExecutionAssets      `json:"assets,omitempty"`
	EvalRun       *harnesspkg.EvalRun                    `json:"eval_run,omitempty"`
	ExecutionGate *harnesspkg.ExecutionEquivalenceReport `json:"execution_gate,omitempty"`
}

type budgetGateOutput = harnesspkg.SkillCutoverBudgetReport

type cutoverReadinessOutput = harnesspkg.SkillCutoverReadinessReport

func runHarnessSelectorEnsureE(cmd *cobra.Command, args []string) error {
	ownerUserID := resolveHarnessOwnerUserID(harnessOwnerUserID)
	assets, err := ensureSelectorCuratedAssets(ownerUserID)
	if err != nil {
		return err
	}
	if jsonOutput {
		printJSON(assets)
		return nil
	}

	fmt.Println("Selector curated assets ready")
	fmt.Printf("Owner: %s\n", ownerUserID)
	if assets.Dataset != nil {
		fmt.Printf("Dataset: %s\n", assets.Dataset.ID)
	}
	if assets.DatasetVersion != nil {
		fmt.Printf("Dataset version: %s", assets.DatasetVersion.ID)
		if version := strings.TrimSpace(assets.DatasetVersion.Version); version != "" {
			fmt.Printf(" (%s)", version)
		}
		fmt.Println()
	}
	if assets.EvalSpec != nil {
		fmt.Printf("Eval spec: %s", assets.EvalSpec.ID)
		if name := strings.TrimSpace(assets.EvalSpec.Name); name != "" {
			fmt.Printf(" (%s)", name)
		}
		fmt.Println()
	}
	return nil
}

func runHarnessSelectorRunE(cmd *cobra.Command, args []string) error {
	ownerUserID := resolveHarnessOwnerUserID(harnessOwnerUserID)
	evalSpecID, assets, err := resolveSelectorEvalSpecID(ownerUserID, strings.TrimSpace(harnessSelectorEvalSpecID))
	if err != nil {
		return err
	}

	evalRun, err := createHarnessEvalRun(harnesspkg.EvalRunSpec{
		EvalSpecID:        evalSpecID,
		OwnerUserID:       ownerUserID,
		Title:             strings.TrimSpace(harnessSelectorTitle),
		BaselineEvalRunID: strings.TrimSpace(harnessSelectorBaseEvalID),
		Metadata:          buildHarnessEvalRunMetadata(harnessSelectorCandidateID),
	})
	if err != nil {
		return err
	}

	if !harnessSelectorWait {
		if jsonOutput {
			printJSON(selectorRunOutput{Assets: assets, EvalRun: evalRun})
			return nil
		}
		printSelectorEvalRunSummary("Selector eval run submitted", evalRun)
		return nil
	}

	finalRun, waitErr := waitForHarnessEvalRun(evalRun.ID, harnessSelectorTimeout, harnessSelectorPollEvery)
	if jsonOutput {
		printJSON(selectorRunOutput{Assets: assets, EvalRun: finalRun})
	} else {
		printSelectorEvalRunSummary("Selector eval run finished", finalRun)
	}
	if waitErr != nil {
		return waitErr
	}
	if finalRun == nil {
		return fmt.Errorf("selector eval run %s did not return a final state", evalRun.ID)
	}
	if finalRun.Status != harnesspkg.RunGroupStatusCompleted {
		return fmt.Errorf("selector eval run %s finished with status %s", finalRun.ID, finalRun.Status)
	}
	return nil
}

func runHarnessSelectorGateE(cmd *cobra.Command, args []string) error {
	req, err := buildHarnessSelectorGateRequest()
	if err != nil {
		return err
	}
	report, err := evaluateHarnessSelectorGate(strings.TrimSpace(args[0]), req)
	if report != nil {
		if jsonOutput {
			printJSON(report)
		} else {
			printSelectorGateSummary(report)
		}
	}
	if err != nil {
		return err
	}
	if report == nil {
		return fmt.Errorf("selector gate did not return a report")
	}
	if !report.Passed {
		return fmt.Errorf("selector gate failed: %s", strings.Join(failedSelectorGateChecks(report.Checks), ", "))
	}
	return nil
}

func runHarnessSelectorVerifyE(cmd *cobra.Command, args []string) error {
	ownerUserID := resolveHarnessOwnerUserID(harnessOwnerUserID)
	evalSpecID, assets, err := resolveSelectorEvalSpecID(ownerUserID, strings.TrimSpace(harnessSelectorEvalSpecID))
	if err != nil {
		return err
	}

	evalRun, err := createHarnessEvalRun(harnesspkg.EvalRunSpec{
		EvalSpecID:        evalSpecID,
		OwnerUserID:       ownerUserID,
		Title:             strings.TrimSpace(harnessSelectorTitle),
		BaselineEvalRunID: strings.TrimSpace(harnessSelectorBaseEvalID),
		Metadata:          buildHarnessEvalRunMetadata(harnessSelectorCandidateID),
	})
	if err != nil {
		return err
	}

	finalRun, waitErr := waitForHarnessEvalRun(evalRun.ID, harnessSelectorTimeout, harnessSelectorPollEvery)
	if waitErr != nil {
		if jsonOutput {
			printJSON(selectorVerifyOutput{Assets: assets, EvalRun: finalRun})
		} else {
			printSelectorEvalRunSummary("Selector eval run failed before gating", finalRun)
		}
		return waitErr
	}
	if finalRun == nil {
		return fmt.Errorf("selector eval run %s did not return a final state", evalRun.ID)
	}
	if finalRun.Status != harnesspkg.RunGroupStatusCompleted {
		if jsonOutput {
			printJSON(selectorVerifyOutput{Assets: assets, EvalRun: finalRun})
		} else {
			printSelectorEvalRunSummary("Selector eval run finished before gating", finalRun)
		}
		return fmt.Errorf("selector eval run %s finished with status %s", finalRun.ID, finalRun.Status)
	}

	req, err := buildHarnessSelectorGateRequest()
	if err != nil {
		return err
	}
	report, err := evaluateHarnessSelectorGate(finalRun.ID, req)
	if jsonOutput {
		printJSON(selectorVerifyOutput{Assets: assets, EvalRun: finalRun, SelectorGate: report})
	} else {
		printSelectorEvalRunSummary("Selector eval run completed", finalRun)
		if report != nil {
			fmt.Println()
			printSelectorGateSummary(report)
		}
	}
	if err != nil {
		return err
	}
	if report == nil {
		return fmt.Errorf("selector gate did not return a report")
	}
	if !report.Passed {
		return fmt.Errorf("selector gate failed: %s", strings.Join(failedSelectorGateChecks(report.Checks), ", "))
	}
	return nil
}

func runHarnessExecutionEnsureE(cmd *cobra.Command, args []string) error {
	ownerUserID := resolveHarnessOwnerUserID(harnessOwnerUserID)
	assets, err := ensureBatch1ExecutionAssets(ownerUserID)
	if err != nil {
		return err
	}
	if jsonOutput {
		printJSON(assets)
		return nil
	}

	fmt.Println("Batch-1 execution assets ready")
	fmt.Printf("Owner: %s\n", ownerUserID)
	if assets.Dataset != nil {
		fmt.Printf("Dataset: %s\n", assets.Dataset.ID)
	}
	if assets.DatasetVersion != nil {
		fmt.Printf("Dataset version: %s", assets.DatasetVersion.ID)
		if version := strings.TrimSpace(assets.DatasetVersion.Version); version != "" {
			fmt.Printf(" (%s)", version)
		}
		fmt.Println()
	}
	if assets.EvalSpec != nil {
		fmt.Printf("Eval spec: %s", assets.EvalSpec.ID)
		if name := strings.TrimSpace(assets.EvalSpec.Name); name != "" {
			fmt.Printf(" (%s)", name)
		}
		fmt.Println()
	}
	return nil
}

func runHarnessExecutionRunE(cmd *cobra.Command, args []string) error {
	ownerUserID := resolveHarnessOwnerUserID(harnessOwnerUserID)
	evalSpecID, assets, err := resolveExecutionEvalSpecID(ownerUserID, strings.TrimSpace(harnessExecutionEvalSpecID))
	if err != nil {
		return err
	}

	evalRun, err := createHarnessEvalRun(harnesspkg.EvalRunSpec{
		EvalSpecID:        evalSpecID,
		OwnerUserID:       ownerUserID,
		Title:             strings.TrimSpace(harnessExecutionTitle),
		BaselineEvalRunID: strings.TrimSpace(harnessExecutionBaseEvalID),
		Metadata:          buildHarnessEvalRunMetadata(harnessExecutionCandidateID),
	})
	if err != nil {
		return err
	}

	if !harnessExecutionWait {
		if jsonOutput {
			printJSON(executionRunOutput{Assets: assets, EvalRun: evalRun})
			return nil
		}
		printSelectorEvalRunSummary("Batch-1 execution eval run submitted", evalRun)
		return nil
	}

	finalRun, waitErr := waitForHarnessEvalRun(evalRun.ID, harnessExecutionTimeout, harnessExecutionPollEvery)
	if jsonOutput {
		printJSON(executionRunOutput{Assets: assets, EvalRun: finalRun})
	} else {
		printSelectorEvalRunSummary("Batch-1 execution eval run finished", finalRun)
	}
	if waitErr != nil {
		return waitErr
	}
	if finalRun == nil {
		return fmt.Errorf("batch-1 execution eval run %s did not return a final state", evalRun.ID)
	}
	if finalRun.Status != harnesspkg.RunGroupStatusCompleted {
		return fmt.Errorf("batch-1 execution eval run %s finished with status %s", finalRun.ID, finalRun.Status)
	}
	return nil
}

func runHarnessExecutionGateE(cmd *cobra.Command, args []string) error {
	req, err := buildHarnessExecutionGateRequest()
	if err != nil {
		return err
	}
	report, err := evaluateHarnessExecutionGate(strings.TrimSpace(args[0]), req)
	if report != nil {
		if jsonOutput {
			printJSON(report)
		} else {
			printExecutionGateSummary(report)
		}
	}
	if err != nil {
		return err
	}
	if report == nil {
		return fmt.Errorf("execution gate did not return a report")
	}
	if !report.Passed {
		return fmt.Errorf("execution gate failed: %s", strings.Join(failedExecutionGateChecks(report.Checks), ", "))
	}
	return nil
}

func runHarnessExecutionVerifyE(cmd *cobra.Command, args []string) error {
	ownerUserID := resolveHarnessOwnerUserID(harnessOwnerUserID)
	evalSpecID, assets, err := resolveExecutionEvalSpecID(ownerUserID, strings.TrimSpace(harnessExecutionEvalSpecID))
	if err != nil {
		return err
	}

	evalRun, err := createHarnessEvalRun(harnesspkg.EvalRunSpec{
		EvalSpecID:        evalSpecID,
		OwnerUserID:       ownerUserID,
		Title:             strings.TrimSpace(harnessExecutionTitle),
		BaselineEvalRunID: strings.TrimSpace(harnessExecutionBaseEvalID),
		Metadata:          buildHarnessEvalRunMetadata(harnessExecutionCandidateID),
	})
	if err != nil {
		return err
	}

	finalRun, waitErr := waitForHarnessEvalRun(evalRun.ID, harnessExecutionTimeout, harnessExecutionPollEvery)
	if waitErr != nil {
		if jsonOutput {
			printJSON(executionVerifyOutput{Assets: assets, EvalRun: finalRun})
		} else {
			printSelectorEvalRunSummary("Batch-1 execution eval run failed before gating", finalRun)
		}
		return waitErr
	}
	if finalRun == nil {
		return fmt.Errorf("batch-1 execution eval run %s did not return a final state", evalRun.ID)
	}
	if finalRun.Status != harnesspkg.RunGroupStatusCompleted {
		if jsonOutput {
			printJSON(executionVerifyOutput{Assets: assets, EvalRun: finalRun})
		} else {
			printSelectorEvalRunSummary("Batch-1 execution eval run finished before gating", finalRun)
		}
		return fmt.Errorf("batch-1 execution eval run %s finished with status %s", finalRun.ID, finalRun.Status)
	}

	req, err := buildHarnessExecutionGateRequest()
	if err != nil {
		return err
	}
	report, err := evaluateHarnessExecutionGate(finalRun.ID, req)
	if jsonOutput {
		printJSON(executionVerifyOutput{Assets: assets, EvalRun: finalRun, ExecutionGate: report})
	} else {
		printSelectorEvalRunSummary("Batch-1 execution eval run completed", finalRun)
		if report != nil {
			fmt.Println()
			printExecutionGateSummary(report)
		}
	}
	if err != nil {
		return err
	}
	if report == nil {
		return fmt.Errorf("execution gate did not return a report")
	}
	if !report.Passed {
		return fmt.Errorf("execution gate failed: %s", strings.Join(failedExecutionGateChecks(report.Checks), ", "))
	}
	return nil
}

func runHarnessBudgetGateE(cmd *cobra.Command, args []string) error {
	req, err := buildHarnessBudgetGateRequest()
	if err != nil {
		return err
	}
	report, err := evaluateHarnessBudgetGate(strings.TrimSpace(args[0]), req)
	if report != nil {
		if jsonOutput {
			printJSON(report)
		} else {
			printBudgetGateSummary(report)
		}
	}
	if err != nil {
		return err
	}
	if report == nil {
		return fmt.Errorf("budget gate did not return a report")
	}
	if !report.Passed {
		return fmt.Errorf("budget gate failed: %s", strings.Join(failedBudgetGateChecks(report.Checks), ", "))
	}
	return nil
}

func runHarnessCutoverReadinessE(cmd *cobra.Command, args []string) error {
	ownerUserID := resolveHarnessOwnerUserID(harnessOwnerUserID)
	budgetReq, err := buildHarnessCutoverBudgetRequest()
	if err != nil {
		return err
	}
	report, err := evaluateHarnessCutoverReadiness(harnesspkg.SkillCutoverReadinessRequest{
		OwnerUserID:             ownerUserID,
		CandidateID:             strings.TrimSpace(harnessCutoverCandidateID),
		RequiredConsecutiveRuns: harnessCutoverRequiredConsecutiveRuns,
		MaxAssessments:          harnessCutoverMaxAssessments,
		Selector: harnesspkg.SelectorGateRequest{
			BaseEvalRunID: strings.TrimSpace(harnessCutoverSelectorBaseEvalID),
			BaselineID:    strings.TrimSpace(harnessCutoverSelectorBaselineID),
		},
		Execution: harnesspkg.ExecutionEquivalenceRequest{
			BaseEvalRunID: strings.TrimSpace(harnessCutoverExecutionBaseEvalID),
			BaselineID:    strings.TrimSpace(harnessCutoverExecutionBaselineID),
		},
		Budget: budgetReq,
	})
	if report != nil {
		if jsonOutput {
			printJSON(report)
		} else {
			printCutoverReadinessSummary(report)
		}
	}
	if err != nil {
		return err
	}
	if report == nil {
		return fmt.Errorf("cutover readiness did not return a report")
	}
	if !report.Ready {
		return fmt.Errorf("cutover readiness not met: %s", strings.Join(failedCutoverReadinessReasons(report), ", "))
	}
	return nil
}

func resolveSelectorEvalSpecID(ownerUserID string, explicitEvalSpecID string) (string, *harnesspkg.SelectorCuratedAssets, error) {
	if explicitEvalSpecID = strings.TrimSpace(explicitEvalSpecID); explicitEvalSpecID != "" {
		return explicitEvalSpecID, nil, nil
	}
	assets, err := ensureSelectorCuratedAssets(ownerUserID)
	if err != nil {
		return "", nil, err
	}
	if assets == nil || assets.EvalSpec == nil || strings.TrimSpace(assets.EvalSpec.ID) == "" {
		return "", nil, fmt.Errorf("selector curated ensure response did not include an eval spec id")
	}
	return strings.TrimSpace(assets.EvalSpec.ID), assets, nil
}

func resolveExecutionEvalSpecID(ownerUserID string, explicitEvalSpecID string) (string, *harnesspkg.Batch1ExecutionAssets, error) {
	if explicitEvalSpecID = strings.TrimSpace(explicitEvalSpecID); explicitEvalSpecID != "" {
		return explicitEvalSpecID, nil, nil
	}
	assets, err := ensureBatch1ExecutionAssets(ownerUserID)
	if err != nil {
		return "", nil, err
	}
	if assets == nil || assets.EvalSpec == nil || strings.TrimSpace(assets.EvalSpec.ID) == "" {
		return "", nil, fmt.Errorf("batch-1 execution ensure response did not include an eval spec id")
	}
	return strings.TrimSpace(assets.EvalSpec.ID), assets, nil
}

func ensureSelectorCuratedAssets(ownerUserID string) (*harnesspkg.SelectorCuratedAssets, error) {
	resp, err := doHarnessRequest(http.MethodPost, "/selector-curated/ensure", map[string]string{
		"owner_user_id": ownerUserID,
	})
	if err != nil {
		return nil, err
	}
	var assets harnesspkg.SelectorCuratedAssets
	if err := decodeInto(resp, &assets); err != nil {
		return nil, err
	}
	return &assets, nil
}

func ensureBatch1ExecutionAssets(ownerUserID string) (*harnesspkg.Batch1ExecutionAssets, error) {
	resp, err := doHarnessRequest(http.MethodPost, "/execution-batch1/ensure", map[string]string{
		"owner_user_id": ownerUserID,
	})
	if err != nil {
		return nil, err
	}
	var assets harnesspkg.Batch1ExecutionAssets
	if err := decodeInto(resp, &assets); err != nil {
		return nil, err
	}
	return &assets, nil
}

func createHarnessEvalRun(spec harnesspkg.EvalRunSpec) (*harnesspkg.EvalRun, error) {
	resp, err := doHarnessRequest(http.MethodPost, "/eval-runs", spec)
	if err != nil {
		return nil, err
	}
	var evalRun harnesspkg.EvalRun
	if err := decodeInto(resp, &evalRun); err != nil {
		return nil, err
	}
	return &evalRun, nil
}

func getHarnessEvalRun(evalRunID string) (*harnesspkg.EvalRun, error) {
	resp, err := doHarnessRequest(http.MethodGet, "/eval-runs/"+strings.TrimSpace(evalRunID), nil)
	if err != nil {
		return nil, err
	}
	var evalRun harnesspkg.EvalRun
	if err := decodeInto(resp, &evalRun); err != nil {
		return nil, err
	}
	return &evalRun, nil
}

func evaluateHarnessSelectorGate(evalRunID string, req harnesspkg.SelectorGateRequest) (*harnesspkg.SelectorGateReport, error) {
	resp, err := doHarnessRequest(http.MethodPost, "/eval-runs/"+strings.TrimSpace(evalRunID)+"/selector-gate", req)
	if err != nil {
		return nil, err
	}
	var report harnesspkg.SelectorGateReport
	if err := decodeInto(resp, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func evaluateHarnessExecutionGate(evalRunID string, req harnesspkg.ExecutionEquivalenceRequest) (*harnesspkg.ExecutionEquivalenceReport, error) {
	resp, err := doHarnessRequest(http.MethodPost, "/eval-runs/"+strings.TrimSpace(evalRunID)+"/execution-gate", req)
	if err != nil {
		return nil, err
	}
	var report harnesspkg.ExecutionEquivalenceReport
	if err := decodeInto(resp, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func evaluateHarnessBudgetGate(evalRunID string, req harnesspkg.SkillCutoverBudgetRequest) (*harnesspkg.SkillCutoverBudgetReport, error) {
	resp, err := doHarnessRequest(http.MethodPost, "/eval-runs/"+strings.TrimSpace(evalRunID)+"/budget-gate", req)
	if err != nil {
		return nil, err
	}
	var report harnesspkg.SkillCutoverBudgetReport
	if err := decodeInto(resp, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func evaluateHarnessCutoverReadiness(req harnesspkg.SkillCutoverReadinessRequest) (*harnesspkg.SkillCutoverReadinessReport, error) {
	resp, err := doHarnessRequest(http.MethodPost, "/cutover-readiness", req)
	if err != nil {
		return nil, err
	}
	var report harnesspkg.SkillCutoverReadinessReport
	if err := decodeInto(resp, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func waitForHarnessEvalRun(evalRunID string, timeout time.Duration, pollEvery time.Duration) (*harnesspkg.EvalRun, error) {
	if strings.TrimSpace(evalRunID) == "" {
		return nil, fmt.Errorf("eval run id is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be greater than 0")
	}
	if pollEvery <= 0 {
		pollEvery = time.Second
	}

	deadline := time.Now().Add(timeout)
	for {
		evalRun, err := getHarnessEvalRun(evalRunID)
		if err != nil {
			return nil, err
		}
		if evalRun != nil && isTerminalHarnessEvalStatus(evalRun.Status) {
			return evalRun, nil
		}
		if time.Now().After(deadline) {
			return evalRun, fmt.Errorf("timed out waiting for selector eval run %s", evalRunID)
		}
		time.Sleep(pollEvery)
	}
}

func isTerminalHarnessEvalStatus(status harnesspkg.RunGroupStatus) bool {
	switch status {
	case harnesspkg.RunGroupStatusCompleted,
		harnesspkg.RunGroupStatusPartial,
		harnesspkg.RunGroupStatusFailed,
		harnesspkg.RunGroupStatusCancelled:
		return true
	default:
		return false
	}
}

func buildHarnessSelectorGateRequest() (harnesspkg.SelectorGateRequest, error) {
	thresholds, err := parseHarnessSelectorThresholds()
	if err != nil {
		return harnesspkg.SelectorGateRequest{}, err
	}
	return harnesspkg.SelectorGateRequest{
		BaseEvalRunID: strings.TrimSpace(harnessSelectorBaseEvalID),
		BaselineID:    strings.TrimSpace(harnessSelectorBaselineID),
		Thresholds:    thresholds,
	}, nil
}

func buildHarnessExecutionGateRequest() (harnesspkg.ExecutionEquivalenceRequest, error) {
	thresholds, err := parseHarnessExecutionThresholds()
	if err != nil {
		return harnesspkg.ExecutionEquivalenceRequest{}, err
	}
	return harnesspkg.ExecutionEquivalenceRequest{
		BaseEvalRunID: strings.TrimSpace(harnessExecutionBaseEvalID),
		BaselineID:    strings.TrimSpace(harnessExecutionBaselineID),
		Thresholds:    thresholds,
	}, nil
}

func buildHarnessBudgetGateRequest() (harnesspkg.SkillCutoverBudgetRequest, error) {
	thresholds, err := parseHarnessBudgetThresholds(
		harnessBudgetMinMedianSchemaByteReductionRate,
		harnessBudgetMaxMedianLatencyIncreaseRate,
		harnessBudgetAllowedFinalNativeTools,
	)
	if err != nil {
		return harnesspkg.SkillCutoverBudgetRequest{}, err
	}
	return harnesspkg.SkillCutoverBudgetRequest{
		BaseEvalRunID: strings.TrimSpace(harnessBudgetBaseEvalID),
		BaselineID:    strings.TrimSpace(harnessBudgetBaselineID),
		Thresholds:    thresholds,
	}, nil
}

func buildHarnessCutoverBudgetRequest() (harnesspkg.SkillCutoverBudgetRequest, error) {
	thresholds, err := parseHarnessBudgetThresholds(
		harnessCutoverMinMedianSchemaByteReductionRate,
		harnessCutoverMaxMedianLatencyIncreaseRate,
		harnessCutoverAllowedFinalNativeTools,
	)
	if err != nil {
		return harnesspkg.SkillCutoverBudgetRequest{}, err
	}
	return harnesspkg.SkillCutoverBudgetRequest{
		BaseEvalRunID: strings.TrimSpace(harnessCutoverBudgetBaseEvalID),
		BaselineID:    strings.TrimSpace(harnessCutoverBudgetBaselineID),
		Thresholds:    thresholds,
	}, nil
}

func parseHarnessSelectorThresholds() (harnesspkg.SelectorGateThresholds, error) {
	var thresholds harnesspkg.SelectorGateThresholds
	zero := 0.0
	one := 1.0

	value, err := parseOptionalFloat64Flag(harnessSelectorMinPassRate, "min-pass-rate", &zero, &one)
	if err != nil {
		return thresholds, err
	}
	thresholds.MinPassRate = value

	value, err = parseOptionalFloat64Flag(harnessSelectorMinCriticalPassRate, "min-critical-pass-rate", &zero, &one)
	if err != nil {
		return thresholds, err
	}
	thresholds.MinCriticalPassRate = value

	value, err = parseOptionalFloat64Flag(harnessSelectorMinRouteAgreementRate, "min-route-agreement-rate", &zero, &one)
	if err != nil {
		return thresholds, err
	}
	thresholds.MinRouteAgreementRate = value

	value, err = parseOptionalFloat64Flag(harnessSelectorMaxClarifyRateDelta, "max-clarify-rate-delta", nil, nil)
	if err != nil {
		return thresholds, err
	}
	thresholds.MaxClarifyRateDelta = value

	intValue, err := parseOptionalIntFlag(harnessSelectorMaxCriticalRegressions, "max-critical-regressions", 0)
	if err != nil {
		return thresholds, err
	}
	thresholds.MaxCriticalRegressionCount = intValue

	return thresholds, nil
}

func parseHarnessExecutionThresholds() (harnesspkg.ExecutionEquivalenceThresholds, error) {
	var thresholds harnesspkg.ExecutionEquivalenceThresholds
	zero := 0.0
	one := 1.0

	value, err := parseOptionalFloat64Flag(harnessExecutionMaxPassRateDrop, "max-pass-rate-drop", &zero, &one)
	if err != nil {
		return thresholds, err
	}
	thresholds.MaxPassRateDrop = value

	intValue, err := parseOptionalIntFlag(harnessExecutionMaxCriticalRegressions, "max-critical-regressions", 0)
	if err != nil {
		return thresholds, err
	}
	thresholds.MaxCriticalRegressionCount = intValue

	value, err = parseOptionalFloat64Flag(harnessExecutionMaxVerificationPassRateDrop, "max-verification-pass-rate-drop", &zero, &one)
	if err != nil {
		return thresholds, err
	}
	thresholds.MaxVerificationPassRateDrop = value

	value, err = parseOptionalFloat64Flag(harnessExecutionMaxEvidenceBackedRateDrop, "max-evidence-backed-pass-rate-drop", &zero, &one)
	if err != nil {
		return thresholds, err
	}
	thresholds.MaxEvidenceBackedPassRateDrop = value

	return thresholds, nil
}

func parseHarnessBudgetThresholds(rawReductionRate string, rawLatencyIncreaseRate string, rawAllowedTools string) (harnesspkg.SkillCutoverBudgetThresholds, error) {
	var thresholds harnesspkg.SkillCutoverBudgetThresholds
	zero := 0.0
	one := 1.0

	value, err := parseOptionalFloat64Flag(rawReductionRate, "min-median-schema-byte-reduction-rate", &zero, &one)
	if err != nil {
		return thresholds, err
	}
	thresholds.MinMedianSchemaByteReductionRate = value

	value, err = parseOptionalFloat64Flag(rawLatencyIncreaseRate, "max-median-latency-increase-rate", &zero, &one)
	if err != nil {
		return thresholds, err
	}
	thresholds.MaxMedianLatencyIncreaseRate = value
	thresholds.AllowedFinalNativeTools = parseCommaSeparatedValues(rawAllowedTools)

	return thresholds, nil
}

func parseOptionalFloat64Flag(raw string, name string, min *float64, max *float64) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q", name, raw)
	}
	if min != nil && value < *min {
		return nil, fmt.Errorf("%s must be at least %.2f", name, *min)
	}
	if max != nil && value > *max {
		return nil, fmt.Errorf("%s must be at most %.2f", name, *max)
	}
	return &value, nil
}

func parseOptionalIntFlag(raw string, name string, min int) (*int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q", name, raw)
	}
	if value < min {
		return nil, fmt.Errorf("%s must be at least %d", name, min)
	}
	return &value, nil
}

func parseCommaSeparatedValues(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func resolveHarnessOwnerUserID(explicit string) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	if envUserID := strings.TrimSpace(os.Getenv("BLUE_USER_ID")); envUserID != "" {
		return envUserID
	}
	return "local-cli"
}

func getHarnessBaseURL() string {
	return getServiceAPIBaseURL("/api/v1/harness")
}

func harnessHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func doHarnessRequest(method, path string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	endpoint := getHarnessBaseURL()
	if strings.TrimSpace(path) != "" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		endpoint += path
	}

	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader, err := localHarnessAuthorizationHeader(); err == nil && authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	return harnessHTTPClient().Do(req)
}

func localHarnessAuthorizationHeader() (string, error) {
	if token := strings.TrimSpace(os.Getenv("BLUE_HARNESS_BEARER_TOKEN")); token != "" {
		return "Bearer " + token, nil
	}
	if !shouldUseLocalHarnessAuth() {
		return "", nil
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", err
	}
	if !usesDefaultHarnessJWTSecret(cfg) {
		return buildHarnessAuthorizationHeaderFromConfig(cfg, harnessOwnerUserID)
	}
	authHeader, ok, err := localHarnessAuthorizationHeaderViaIPC()
	if err != nil {
		return "", err
	}
	if ok {
		return authHeader, nil
	}
	return "", nil
}

func localHarnessAuthorizationHeaderViaIPC() (string, bool, error) {
	resp, err := ipcRoundTrip(&sockipc.Request{
		Cmd: "cli.harness_auth_header",
		Params: map[string]string{
			"owner_user_id": resolveHarnessOwnerUserID(harnessOwnerUserID),
		},
	})
	if err != nil {
		if isConnectionError(err) {
			return "", false, fmt.Errorf("running Blue service is required for local harness auth over IPC: %w", err)
		}
		return "", false, err
	}
	if resp.Status != "ok" {
		return "", false, fmt.Errorf("%s", resp.Error)
	}
	return strings.TrimSpace(resp.Data["authorization"]), true, nil
}

func usesDefaultHarnessJWTSecret(cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	secret := strings.TrimSpace(cfg.Security.JWT.Secret)
	return secret == "" || secret == defaultHarnessJWTSecretPlaceholder
}

func shouldUseLocalHarnessAuth() bool {
	switch strings.ToLower(strings.TrimSpace(resolveServerHost())) {
	case "", "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	default:
		return false
	}
}

func printSelectorEvalRunSummary(prefix string, evalRun *harnesspkg.EvalRun) {
	if evalRun == nil {
		fmt.Println(prefix)
		fmt.Println("Eval run: <nil>")
		return
	}
	fmt.Println(prefix)
	fmt.Printf("Eval run: %s\n", evalRun.ID)
	fmt.Printf("Status: %s\n", evalRun.Status)
	if evalRun.EvalSpecID != "" {
		fmt.Printf("Eval spec: %s\n", evalRun.EvalSpecID)
	}
	if evalRun.GroupID != "" {
		fmt.Printf("Group: %s\n", evalRun.GroupID)
	}
	if evalRun.Title != "" {
		fmt.Printf("Title: %s\n", evalRun.Title)
	}
}

func printSelectorGateSummary(report *harnesspkg.SelectorGateReport) {
	if report == nil {
		fmt.Println("Selector gate report unavailable")
		return
	}
	status := "FAILED"
	if report.Passed {
		status = "PASSED"
	}
	fmt.Printf("Selector gate %s\n", status)
	fmt.Printf("Target eval run: %s\n", report.TargetEvalRunID)
	if report.BaseEvalRunID != "" {
		fmt.Printf("Base eval run: %s\n", report.BaseEvalRunID)
	}
	if report.BaselineID != "" {
		fmt.Printf("Baseline: %s\n", report.BaselineID)
	}
	if report.ComparisonReportID != "" {
		fmt.Printf("Comparison report: %s\n", report.ComparisonReportID)
	}
	fmt.Printf("Curated pass rate: %.2f (%d/%d)\n", report.Metrics.PassRate, report.Metrics.PassedCount, report.Metrics.CaseCount)
	fmt.Printf("Critical pass rate: %.2f (%d/%d)\n", report.Metrics.CriticalPassRate, report.Metrics.CriticalPassedCount, report.Metrics.CriticalCaseCount)
	if report.Metrics.RouteCaseCount > 0 {
		fmt.Printf("Route agreement: %.2f (%d/%d)\n", report.Metrics.RouteAgreementRate, report.Metrics.RouteAgreementCount, report.Metrics.RouteCaseCount)
	}
	fmt.Printf("Clarify delta: %.2f\n", report.Metrics.ClarifyRateDelta)
	printHarnessBreakdownSummary("Canonical skills", report.Metrics.SelectedCanonicalSkillBreakdown)
	printHarnessBreakdownSummary("Native surface modes", report.Metrics.NativeSurfaceModeBreakdown)
	printHarnessBreakdownSummary("Native surface reasons", report.Metrics.NativeSurfaceReasonBreakdown)
	printHarnessBreakdownSummary("Execution profiles", report.Metrics.ExecutionProfileBreakdown)
	if failed := failedSelectorGateChecks(report.Checks); len(failed) > 0 {
		fmt.Printf("Failed checks: %s\n", strings.Join(failed, ", "))
	}
}

func printExecutionGateSummary(report *harnesspkg.ExecutionEquivalenceReport) {
	if report == nil {
		fmt.Println("Execution gate report unavailable")
		return
	}
	status := "FAILED"
	if report.Passed {
		status = "PASSED"
	}
	fmt.Printf("Execution gate %s\n", status)
	fmt.Printf("Target eval run: %s\n", report.TargetEvalRunID)
	if report.BaseEvalRunID != "" {
		fmt.Printf("Base eval run: %s\n", report.BaseEvalRunID)
	}
	if report.BaselineID != "" {
		fmt.Printf("Baseline: %s\n", report.BaselineID)
	}
	if report.ComparisonReportID != "" {
		fmt.Printf("Comparison report: %s\n", report.ComparisonReportID)
	}
	fmt.Printf("Pass rate: %.2f (%d/%d)\n", report.Metrics.PassRate, report.Metrics.PassedCount, report.Metrics.CaseCount)
	fmt.Printf("Critical pass rate: %.2f (%d/%d)\n", report.Metrics.CriticalPassRate, report.Metrics.CriticalPassedCount, report.Metrics.CriticalCaseCount)
	fmt.Printf("Pass-rate delta: %.2f\n", report.Metrics.PassRateDelta)
	fmt.Printf("Critical regressions: %d\n", report.Metrics.CriticalRegressionCount)
	if failed := failedExecutionGateChecks(report.Checks); len(failed) > 0 {
		fmt.Printf("Failed checks: %s\n", strings.Join(failed, ", "))
	}
}

func printBudgetGateSummary(report *harnesspkg.SkillCutoverBudgetReport) {
	if report == nil {
		fmt.Println("Budget gate report unavailable")
		return
	}
	status := "FAILED"
	if report.Passed {
		status = "PASSED"
	}
	fmt.Printf("Budget gate %s\n", status)
	fmt.Printf("Target eval run: %s\n", report.TargetEvalRunID)
	if report.BaseEvalRunID != "" {
		fmt.Printf("Base eval run: %s\n", report.BaseEvalRunID)
	}
	if report.BaselineID != "" {
		fmt.Printf("Baseline: %s\n", report.BaselineID)
	}
	fmt.Printf("Comparable cases: %d/%d\n", report.Metrics.ComparableCaseCount, report.Metrics.CaseCount)
	fmt.Printf("Median schema-byte reduction: %.2f\n", report.Metrics.MedianSchemaByteReductionRate)
	fmt.Printf("Median latency increase: %.2f\n", report.Metrics.MedianLatencyIncreaseRate)
	fmt.Printf("Non-allowed native tool cases: %d\n", report.Metrics.NonAllowedNativeToolCaseCount)
	printHarnessBreakdownSummary("Canonical skills", report.Metrics.SelectedCanonicalSkillBreakdown)
	printHarnessBreakdownSummary("Native surface modes", report.Metrics.NativeSurfaceModeBreakdown)
	printHarnessBreakdownSummary("Native surface reasons", report.Metrics.NativeSurfaceReasonBreakdown)
	printHarnessBreakdownSummary("Execution profiles", report.Metrics.ExecutionProfileBreakdown)
	if failed := failedBudgetGateChecks(report.Checks); len(failed) > 0 {
		fmt.Printf("Failed checks: %s\n", strings.Join(failed, ", "))
	}
}

func printCutoverReadinessSummary(report *harnesspkg.SkillCutoverReadinessReport) {
	if report == nil {
		fmt.Println("Cutover readiness report unavailable")
		return
	}
	status := "NOT READY"
	if report.Ready {
		status = "READY"
	}
	fmt.Printf("Cutover readiness %s\n", status)
	if report.CandidateID != "" {
		fmt.Printf("Candidate: %s\n", report.CandidateID)
	}
	fmt.Printf("Evaluated gates ready: %t\n", report.EvaluatedGatesReady)
	fmt.Printf("Selector streak: %d/%d\n", report.Selector.ConsecutivePassCount, report.RequiredConsecutiveRuns)
	fmt.Printf("Execution streak: %d/%d\n", report.Execution.ConsecutivePassCount, report.RequiredConsecutiveRuns)
	fmt.Printf("Budget streak: %d/%d\n", report.Budget.ConsecutivePassCount, report.RequiredConsecutiveRuns)
	if len(report.UnverifiedRequirements) > 0 {
		fmt.Printf("Unverified requirements: %s\n", strings.Join(report.UnverifiedRequirements, ", "))
	}
	if len(report.BlockingReasons) > 0 {
		fmt.Printf("Blocking reasons: %s\n", strings.Join(report.BlockingReasons, "; "))
	}
}

func buildHarnessEvalRunMetadata(candidateID string) map[string]interface{} {
	candidateID = strings.TrimSpace(candidateID)
	if candidateID == "" {
		return nil
	}
	return map[string]interface{}{
		"candidate_id": candidateID,
	}
}

func failedSelectorGateChecks(checks []harnesspkg.SelectorGateCheck) []string {
	out := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.Passed {
			continue
		}
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "unnamed_check"
		}
		out = append(out, name)
	}
	return out
}

func failedExecutionGateChecks(checks []harnesspkg.ExecutionEquivalenceCheck) []string {
	out := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.Passed {
			continue
		}
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "unnamed_check"
		}
		out = append(out, name)
	}
	return out
}

func failedBudgetGateChecks(checks []harnesspkg.SkillCutoverBudgetCheck) []string {
	out := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.Passed {
			continue
		}
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "unnamed_check"
		}
		out = append(out, name)
	}
	return out
}

func failedCutoverReadinessReasons(report *harnesspkg.SkillCutoverReadinessReport) []string {
	if report == nil {
		return []string{"report_unavailable"}
	}
	out := make([]string, 0, len(report.BlockingReasons)+len(report.UnverifiedRequirements))
	out = append(out, report.BlockingReasons...)
	out = append(out, report.UnverifiedRequirements...)
	if len(out) == 0 {
		out = append(out, "cutover_not_ready")
	}
	return out
}

func printHarnessBreakdownSummary(label string, breakdown map[string]int) {
	formatted := formatHarnessBreakdown(breakdown)
	if formatted == "" {
		return
	}
	fmt.Printf("%s: %s\n", label, formatted)
}

func formatHarnessBreakdown(breakdown map[string]int) string {
	if len(breakdown) == 0 {
		return ""
	}
	keys := make([]string, 0, len(breakdown))
	for key := range breakdown {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, breakdown[key]))
	}
	return strings.Join(parts, ", ")
}

func batch1ExecutionSkillsForHelp() []string {
	return []string{"web_query", "analyze", "reminder"}
}
