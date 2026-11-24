import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { Window } from 'happy-dom'
import { describe, expect, it } from 'vitest'

const desktopSource = readFileSync(
  resolve(process.cwd(), '../tauri-app/src-tauri/src/lib.rs'),
  'utf8'
)
const splashScript = desktopSource.match(
  /const ABOUT_BLANK_SPLASH_SCRIPT: &str = r##"([\s\S]*?)"##;/
)![1]!

function runSplash(window: Window) {
  new Function('window', 'document', splashScript)(window, window.document)
}

describe('native desktop startup splash', () => {
  it('renders after document-start execution before a body exists', () => {
    const window = new Window({ url: 'about:blank' })
    const document = window.document
    document.body.remove()

    runSplash(window)
    document.documentElement.appendChild(document.createElement('body'))
    document.dispatchEvent(new window.Event('DOMContentLoaded'))

    expect(document.querySelector('.startup-shell__composer-bar')).not.toBeNull()
  })

  it('preserves an already rendered splash when the page-load callback runs again', () => {
    const window = new Window({ url: 'about:blank' })
    runSplash(window)
    const shell = window.document.querySelector('.startup-shell')
    expect(shell).not.toBeNull()

    runSplash(window)

    expect(window.document.querySelector('.startup-shell')).toBe(shell)
  })

  it('does not overwrite the real localhost application', () => {
    const window = new Window({ url: 'http://localhost:43127/chat' })
    window.document.body.innerHTML = '<main id="app">Ready</main>'

    runSplash(window)

    expect(window.document.getElementById('app')?.textContent).toBe('Ready')
    expect(window.document.querySelector('.startup-shell')).toBeNull()
  })
})
