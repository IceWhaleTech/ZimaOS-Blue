export type HarnessFailureHintTranslator = (key: string, fallback: string) => string

export function harnessFailureLabelHint(label: string, tr: HarnessFailureHintTranslator): string {
  switch (String(label || '').trim()) {
    case 'missing_artifact':
      return tr(
        'harness.group.remediationMissingArtifact',
        'Attach the expected artifact path or label to the run, or align expected_artifacts with the actual output contract.'
      )
    case 'required_check_missing':
      return tr(
        'harness.group.remediationRequiredCheckMissing',
        'Emit a stable completion marker in events or results, or align required_checks with the observable success signal.'
      )
    case 'tool_selection_error':
      return tr(
        'harness.group.remediationToolSelectionError',
        'Make the required tool available in this profile and bias planning so the run selects it explicitly.'
      )
    case 'forbidden_tool_used':
      return tr(
        'harness.group.remediationForbiddenToolUsed',
        'Tighten tool policy, allowlists, or prompt constraints so the forbidden tool cannot be selected for this case.'
      )
    case 'run_missing':
      return tr(
        'harness.group.remediationRunMissing',
        'Check scheduler persistence and run linking so the produced run is stored before scoring begins.'
      )
    case 'run_failed':
      return tr(
        'harness.group.remediationRunFailed',
        'Inspect the linked run error and logs, fix the runtime failure, then retry the case.'
      )
    case 'run_cancelled':
      return tr(
        'harness.group.remediationRunCancelled',
        'Check what cancelled the run and keep approvals or user-input waits from terminating the attempt early.'
      )
    case 'run_aborted':
      return tr(
        'harness.group.remediationRunAborted',
        'Inspect abort reasons from the driver or runtime and restore the execution path before retrying.'
      )
    case 'run_not_completed':
      return tr(
        'harness.group.remediationRunNotCompleted',
        'Reduce task scope or raise runtime budgets so the run can reach completed status and publish its evidence.'
      )
    case 'verification_failed':
      return tr(
        'harness.group.remediationVerificationFailed',
        'Add stronger observable evidence or artifacts for success, or relax the contract if it is stricter than the intended outcome.'
      )
    case 'missing_clarification':
      return tr(
        'harness.group.remediationMissingClarification',
        'Update instructions or planning so the run asks for missing user details before committing to a brittle answer.'
      )
    case 'question_left_unresolved':
      return tr(
        'harness.group.remediationQuestionLeftUnresolved',
        'Resolve requested user questions before ending the run, or replan so blocked questions do not strand the task.'
      )
    case 'approval_blocked_without_replan':
      return tr(
        'harness.group.remediationApprovalBlockedWithoutReplan',
        'Handle approval waits explicitly: get the approval, downgrade the action, or replan instead of letting the run die blocked.'
      )
    case 'tool_failed_without_fallback':
      return tr(
        'harness.group.remediationToolFailedWithoutFallback',
        'Add a fallback path after tool errors so the run retries with another tool, another strategy, or a narrower scope.'
      )
    case 'missing_evidence_collection':
      return tr(
        'harness.group.remediationMissingEvidenceCollection',
        'Call evidence-gathering tools during research-style runs so the answer is backed by observable retrieval steps.'
      )
    case 'timeout':
      return tr(
        'harness.group.remediationTimeout',
        'Shorten the task or increase timeout-related budgets so required steps can finish before scoring.'
      )
    default:
      return tr(
        'harness.group.remediationGeneric',
        'Inspect linked runs, checks, and artifacts to align the runtime output with the declared contract.'
      )
  }
}
