import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('zimaos-blue-locale', 'zh-CN')
  })
})

test('renders dream controls in userdata settings and runs one dream pass', async ({ page }) => {
  await page.goto('/settings?tab=userdata')

  const dreamPanel = page.getByTestId('memory-dream-panel')
  await expect(dreamPanel).toBeVisible()
  await expect(dreamPanel).toContainText('待整理胶囊')
  await expect(dreamPanel).toContainText('2')
  await expect(dreamPanel).toContainText('5')

  const runButton = page.getByTestId('memory-dream-run')
  await expect(runButton).toBeVisible()
  await runButton.click()

  await expect(dreamPanel).toContainText('7')
  await expect(dreamPanel).toContainText('0')
  await expect(page.locator('.settings-toast')).toContainText('Dream 已完成')
})
