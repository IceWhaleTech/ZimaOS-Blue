import type { LocaleMessages } from './merge'

const harnessV3TailLocaleOverrides: Record<string, LocaleMessages> = {
  'en-GB': {
    harness: {
      compare: {
        verificationPassRateDelta: 'Validation pass-rate delta',
        evidenceBackedPassRateDelta: 'Evidence-backed pass-rate delta',
        retryRecoveredDelta: 'Retry recovery delta',
        failureLabelDelta: 'Failure tag delta',
        noFailureLabelDelta: 'No failure-tag changes recorded.',
      },
      group: {
        verificationPassRate: 'Validation pass-rate',
        evidenceBackedPassRate: 'Evidence-backed pass-rate figure',
        retryRecovered: 'Recovered on retry',
        failureLabels: 'Failure tags',
        noFailureLabels: 'No failure tags recorded.',
        verification: 'Validation',
        evidenceScore: 'Evidence mark',
        failureLabel: 'Failure tag',
      },
      quickEval: {
        caseRequired: 'Add at least one genuine case before starting a quick evaluation.',
        caseTemplateHint: 'Template only. Paste genuine cases here before starting.',
        conversationCaseCount: '{count} draft cases from this chat',
        conversationEmpty: 'This chat has not produced any draft cases yet.',
        conversationHint:
          'Generate draft cases by pairing each user message with the following assistant reply.',
        conversationLoadFailed: 'Quick evaluation could not load chat data.',
        conversationRequired: 'Choose a conversation before starting a quick evaluation.',
        draftCases: 'Draft case set',
        editManifest: 'Edit cases JSON',
        launchHint:
          'Start from pasted cases, an earlier chat or an existing dataset. Harness fills in the object model in the background.',
        previewManifest: 'Preview cases JSON',
        regressionLabel: 'Regression check',
        researchLabel: 'Research mode',
        selectConversation: 'Choose a conversation',
        smokeDatasetName: 'Smoke-check dataset',
        smokeLabel: 'Smoke check',
        untitledConversation: 'Untitled chat',
        useConversation: 'Use chat',
      },
    },
  },
  'ga-IE': {
    harness: {
      datasets: {
        total: 'Tacair sonraí',
      },
      evalRun: {
        baseline: 'Rith bunlíne',
      },
      baseline: {
        total: 'Bunlínte',
      },
    },
  },
  'pl-PL': {
    harness: {
      baseline: {
        total: 'Linie bazowe',
      },
      quickEval: {
        preset: 'Ustawienie wstępne',
        presetChip: '2. Ustawienie wstępne',
      },
    },
  },
  'pt-BR': {
    harness: {
      datasets: {
        total: 'Conjuntos de dados',
      },
      evalRuns: {
        title: 'Execucoes de avaliacao',
        total: 'Execucoes de avaliacao',
      },
      baseline: {
        total: 'Linhas de base',
      },
    },
  },
  'pt-PT': {
    harness: {
      datasets: {
        total: 'Conjuntos de dados',
      },
      evalRuns: {
        title: 'Execucoes de avaliacao',
        total: 'Execucoes de avaliacao',
      },
      baseline: {
        total: 'Linhas de base',
      },
    },
  },
  'ro-RO': {
    harness: {
      evalRuns: {
        title: 'Rulari de evaluare',
        total: 'Rulari de evaluare',
      },
      baseline: {
        total: 'Linii de baza',
      },
    },
  },
  'ru-RU': {
    harness: {
      baseline: {
        total: 'Etalony',
      },
    },
  },
  'sk-SK': {
    harness: {
      evalRun: {
        baseline: 'Referencny run',
      },
      baseline: {
        total: 'Zakladne linie',
      },
      quickEval: {
        autoChip: 'Auto: verzia + spec + run',
        preset: 'Predvolba',
        presetChip: '2. Predvolba',
      },
    },
  },
  'sv-SE': {
    harness: {
      baseline: {
        total: 'Baslinjer',
      },
      quickEval: {
        autoChip: 'Auto: version + spec + korning',
        regressionLabel: 'Regressionskontroll',
      },
    },
  },
}

export default harnessV3TailLocaleOverrides
