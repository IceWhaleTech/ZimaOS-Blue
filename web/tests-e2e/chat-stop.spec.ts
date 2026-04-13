import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('zimaos-blue-locale', 'zh-CN')
  })
})

test('stops an executing chat stream and renders the stopped badge', async ({ page }) => {
  await page.goto('/chat?conversationId=conv-1')

  await expect(page.locator('.chat-stream-status-rail')).toBeVisible()

  const cancelButton = page.locator('button.desktop-cancel-btn[title="取消"]').first()
  await expect(cancelButton).toBeVisible()

  await cancelButton.click()

  const stoppedBadge = page.locator('.response-stopped-indicator')
  await expect(stoppedBadge).toBeVisible()
  await expect(stoppedBadge).toContainText('已停止')
  await expect(page.locator('body')).not.toContainText('[Response stopped]')
  await expect(cancelButton).toBeHidden()
})
