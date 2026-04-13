import { defineConfig, devices } from '@playwright/test'

const appPort = Number(process.env.PLAYWRIGHT_APP_PORT || '3100')
const apiPort = Number(process.env.PLAYWRIGHT_API_PORT || '8787')
const baseURL = `http://127.0.0.1:${appPort}`
const apiProxyTarget = `http://127.0.0.1:${apiPort}`

export default defineConfig({
  testDir: './tests-e2e',
  timeout: 30_000,
  expect: {
    timeout: 10_000,
  },
  fullyParallel: false,
  retries: process.env.CI ? 2 : 0,
  reporter: 'list',
  use: {
    baseURL,
    headless: true,
    locale: 'zh-CN',
    trace: 'retain-on-failure',
  },
  webServer: [
    {
      command: `node scripts/playwright-chat-stop-mock-server.mjs`,
      port: apiPort,
      reuseExistingServer: !process.env.CI,
      timeout: 30_000,
      env: {
        ...process.env,
        PLAYWRIGHT_API_PORT: String(apiPort),
      },
    },
    {
      command: `npm run dev -- --host 127.0.0.1 --port ${appPort}`,
      port: appPort,
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
      env: {
        ...process.env,
        VITE_API_PROXY_TARGET: apiProxyTarget,
      },
    },
  ],
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
      },
    },
  ],
})
