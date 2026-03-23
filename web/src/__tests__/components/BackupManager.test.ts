import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import BackupManager from '@/components/BackupManager.vue'
import { i18n } from '@/i18n'

function mountBackupManager(props: Record<string, unknown> = {}) {
  return mount(BackupManager, {
    props: {
      backups: [
        {
          id: 'backup-1',
          created_at: '2026-03-01T00:00:00Z',
          size_bytes: 1024,
          type: 'full',
        },
      ],
      ...props,
    },
    global: {
      plugins: [i18n],
      stubs: {
        teleport: true,
      },
    },
  })
}

describe('BackupManager', () => {
  it('disables the create action while a backup is being created', () => {
    const wrapper = mountBackupManager({ creating: true })

    const createButton = wrapper.get('[data-testid="backup-create-button"]')
    expect(createButton.attributes('disabled')).toBeDefined()

    wrapper.unmount()
  })

  it('shows per-row pending states for restore and delete actions', () => {
    const wrapper = mountBackupManager({
      restoringId: 'backup-1',
      deletingId: 'backup-1',
    })

    expect(
      wrapper.get('[data-testid="backup-restore-backup-1"]').find('.animate-spin').exists()
    ).toBe(true)
    expect(
      wrapper.get('[data-testid="backup-delete-backup-1"]').find('.animate-spin').exists()
    ).toBe(true)

    wrapper.unmount()
  })
})
