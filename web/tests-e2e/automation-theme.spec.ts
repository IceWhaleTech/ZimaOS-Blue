import { expect, test, type Locator } from '@playwright/test'

async function brightness(locator: Locator, property: 'backgroundColor' | 'color') {
  return locator.evaluate((element, name) => {
    const context = document.createElement('canvas').getContext('2d')!
    context.fillStyle = getComputedStyle(element)[name]
    context.fillRect(0, 0, 1, 1)
    const channels = context.getImageData(0, 0, 1, 1).data
    return (channels[0]! + channels[1]! + channels[2]!) / 3
  }, property)
}

for (const theme of ['dark', 'light'] as const) {
  test.describe(`${theme} automation surfaces`, () => {
    test.beforeEach(async ({ page }) => {
      await page.addInitScript((savedTheme) => {
        localStorage.setItem('zimaos-blue-theme', savedTheme)
        localStorage.setItem('zimaos-blue-locale', 'en')
      }, theme)
      for (const pattern of ['**/api/v1/harness/**', '**/api/cron', '**/api/cron/**', '**/api/v1/knowledge/**']) {
        await page.route(pattern, (route) => route.fulfill({ json: [] }))
      }
    })

    for (const [path, root] of [
      ['/operations', '.cron-page'],
      ['/operations/harness', '.harness-groups-page'],
      ['/operations/evolution?pane=knowledge', '.evolution-page'],
    ]) {
      test(`renders readable overview and cards at ${path}`, async ({ page }) => {
        await page.goto(path!)
        const overview = page.locator(`${root} > div > section`).first()
        const title = overview.locator('h1')
        await expect(title).toBeVisible()
        await expect(page.locator('html')).toHaveAttribute('data-theme', theme)

        if (theme === 'dark') {
          expect(await brightness(overview, 'backgroundColor')).toBeLessThan(100)
          expect(await overview.evaluate((el) => getComputedStyle(el).backgroundImage)).toBe('none')
          expect(await brightness(title, 'color')).toBeGreaterThan(180)
        } else {
          expect(await brightness(title, 'color')).toBeLessThan(100)
        }

        if (root === '.evolution-page') {
          const card = overview.locator('section').first()
          await expect(card).toBeVisible()
          const value = await brightness(card, 'backgroundColor')
          expect(theme === 'dark' ? value < 100 : value > 200).toBe(true)
          const graph = card.locator('.relative.overflow-hidden').first()
          await expect(graph).toBeVisible()
          if (theme === 'dark') {
            expect(await graph.evaluate((el) => getComputedStyle(el).backgroundImage)).toBe('none')
          }
        }
      })
    }

    test('themes the teleported live monitor without styling the document root', async ({ page }) => {
      await page.goto('/operations')
      const launcher = page.locator('.browser-monitor-launcher')
      await expect(launcher).toBeVisible()
      await launcher.click()
      const panel = page.locator('.browser-monitor')
      await expect(panel).toBeVisible()
      const value = await brightness(panel, 'backgroundColor')
      expect(theme === 'dark' ? value < 100 : value > 200).toBe(true)
      expect(await page.locator('html').evaluate((el) => getComputedStyle(el).backgroundImage)).toBe('none')
    })
  })
}
