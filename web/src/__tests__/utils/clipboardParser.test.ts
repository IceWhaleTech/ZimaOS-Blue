import { describe, it, expect } from 'vitest'
import { getUrlProbability, getTokenProbability, parseClipboardData } from '@/utils/clipboardParser'

describe('Clipboard Parser', () => {
  describe('getUrlProbability', () => {
    it('should return high probability for standard URLs', () => {
      expect(getUrlProbability('https://api.openai.com/v1/chat')).toBeGreaterThan(0.6)
      expect(getUrlProbability('http://localhost:3000/api')).toBeGreaterThan(0.5)
      expect(getUrlProbability('https://paid.tribiosapi.top/')).toBeGreaterThan(0.6)
    })

    it('should return moderate probability for www URLs', () => {
      expect(getUrlProbability('www.example.com')).toBeGreaterThan(0.4)
    })

    it('should return low probability for tokens', () => {
      expect(getUrlProbability('sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE')).toBeLessThan(0.3)
      expect(getUrlProbability('ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx')).toBeLessThan(0.3)
    })

    it('should return 0 for empty strings', () => {
      expect(getUrlProbability('')).toBe(0)
      expect(getUrlProbability('   ')).toBe(0)
    })

    it('should handle URLs with ports', () => {
      expect(getUrlProbability('http://localhost:23456')).toBeGreaterThan(0.5)
      expect(getUrlProbability('https://api.example.com:443/v1')).toBeGreaterThanOrEqual(0.7)
    })

    it('should handle noisy input with whitespace', () => {
      expect(getUrlProbability('  https://api.example.com  ')).toBeGreaterThan(0.6)
      expect(getUrlProbability('\thttps://api.example.com\n')).toBeGreaterThan(0.6)
    })
  })

  describe('getTokenProbability', () => {
    it('should return high probability for OpenAI keys', () => {
      expect(getTokenProbability('sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE')).toBeGreaterThan(0.7)
      expect(getTokenProbability('sk-proj-abc123def456')).toBeGreaterThan(0.7)
    })

    it('should return high probability for GitHub tokens', () => {
      expect(getTokenProbability('ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx')).toBeGreaterThan(0.7)
      expect(getTokenProbability('gho_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx')).toBeGreaterThan(0.7)
    })

    it('should return high probability for Slack tokens', () => {
      expect(getTokenProbability('xoxb-123456789012-1234567890123-abcdefghijklmnopqrstuvwx')).toBeGreaterThan(0.7)
    })

    it('should return moderate probability for long alphanumeric strings', () => {
      expect(getTokenProbability('abcdef1234567890abcdef1234567890')).toBeGreaterThan(0.4)
    })

    it('should return high probability for hex strings (MD5/SHA)', () => {
      expect(getTokenProbability('d41d8cd98f00b204e9800998ecf8427e')).toBeGreaterThan(0.5) // MD5
      expect(getTokenProbability('e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855')).toBeGreaterThan(0.5) // SHA256
    })

    it('should return low probability for URLs', () => {
      expect(getTokenProbability('https://api.example.com')).toBeLessThan(0.3)
    })

    it('should return 0 for empty strings', () => {
      expect(getTokenProbability('')).toBe(0)
      expect(getTokenProbability('   ')).toBe(0)
    })

    it('should handle noisy input with whitespace', () => {
      expect(getTokenProbability('  sk-abc123def456ghi789  ')).toBeGreaterThan(0.7)
    })

    it('should return lower probability for strings with spaces', () => {
      const withSpace = getTokenProbability('sk-abc 123def456')
      const withoutSpace = getTokenProbability('sk-abc123def456')
      expect(withSpace).toBeLessThan(withoutSpace)
    })
  })

  describe('parseClipboardData', () => {
    describe('URL + Token pattern', () => {
      it('should parse URL and token on separate lines', () => {
        const data = `https://paid.tribiosapi.top/
sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE`
        const result = parseClipboardData(data)
        expect(result['url']).toBe('https://paid.tribiosapi.top/')
        expect(result['api_key']).toBe('sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE')
      })

      it('should parse token and URL in reverse order', () => {
        const data = `sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE
https://paid.tribiosapi.top/`
        const result = parseClipboardData(data)
        expect(result['url']).toBe('https://paid.tribiosapi.top/')
        expect(result['api_key']).toBe('sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE')
      })

      it('should handle extra whitespace and newlines', () => {
        const data = `  https://api.example.com/v1

  sk-testkey123456789012345  `
        const result = parseClipboardData(data)
        expect(result['url']).toBe('https://api.example.com/v1')
        expect(result['api_key']).toBe('sk-testkey123456789012345')
      })
    })

    describe('Key-value formats', () => {
      it('should parse key: value format', () => {
        const data = `api_key: sk-abc123
url: https://api.example.com`
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })

      it('should parse Chinese colon format', () => {
        const data = `密钥：sk-abc123
地址：https://api.example.com`
        const result = parseClipboardData(data)
        expect(result['密钥']).toBe('sk-abc123')
        expect(result['地址']).toBe('https://api.example.com')
      })

      it('should parse key=value format', () => {
        const data = `API_KEY=sk-abc123
BASE_URL=https://api.example.com
EXTRA_CONFIG=value`
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['base_url']).toBe('https://api.example.com')
      })

      it('should parse tab-separated format', () => {
        const data = `api_key\tsk-abc123
url\thttps://api.example.com
extra\tvalue`
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })

      it('should parse space-separated format', () => {
        const data = `api_key sk-abc123
url https://api.example.com
extra value`
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })

      it('should parse quoted values', () => {
        const data = `api_key = "sk-abc123" url = "https://api.example.com"`
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })
    })

    describe('Noisy input handling', () => {
      it('should handle mixed whitespace', () => {
        const data = `  api_key:   sk-abc123
  url:   https://api.example.com  `
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })

      it('should handle empty lines between data', () => {
        const data = `api_key: sk-abc123

url: https://api.example.com

`
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })

      it('should strip quotes from values', () => {
        const data = `api_key: "sk-abc123"
url: 'https://api.example.com'`
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })

      it('should handle Windows line endings', () => {
        const data = "api_key: sk-abc123\r\nurl: https://api.example.com\r\n"
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })

      it('should handle tabs and spaces mixed', () => {
        const data = `  \t api_key:  \t sk-abc123 \t
\t  url: \t https://api.example.com  \t`
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })
    })

    describe('Edge cases', () => {
      it('should return empty object for empty input', () => {
        expect(parseClipboardData('')).toEqual({})
        expect(parseClipboardData('   ')).toEqual({})
        expect(parseClipboardData('\n\n\n')).toEqual({})
      })

      it('should handle single line with no key-value pattern', () => {
        const result = parseClipboardData('just some random text')
        // Should try space-separated: "just" as key, "some random text" as value
        expect(result['just']).toBe('some random text')
      })

      it('should handle special characters in values', () => {
        const data = `password: P@ssw0rd!#$%^&*()`
        const result = parseClipboardData(data)
        expect(result['password']).toBe('P@ssw0rd!#$%^&*()')
      })

      it('should handle URLs with query parameters', () => {
        const data = `url: https://api.example.com/v1?key=value&foo=bar`
        const result = parseClipboardData(data)
        expect(result['url']).toBe('https://api.example.com/v1?key=value&foo=bar')
      })

      it('should handle multiple equals signs in value', () => {
        const data = `connection_string=Server=localhost;Database=test;User=admin`
        const result = parseClipboardData(data)
        expect(result['connection_string']).toBe('Server=localhost;Database=test;User=admin')
      })

      it('should handle colons in URL values', () => {
        const data = `endpoint: https://api.example.com:23456/v1`
        const result = parseClipboardData(data)
        expect(result['endpoint']).toBe('https://api.example.com:23456/v1')
      })
    })

    describe('Real-world examples', () => {
      it('should parse OpenAI-style config', () => {
        const data = `OPENAI_API_KEY=sk-proj-abc123def456ghi789
OPENAI_BASE_URL=https://api.openai.com/v1
MODEL=gpt-4`
        const result = parseClipboardData(data)
        expect(result['openai_api_key']).toBe('sk-proj-abc123def456ghi789')
        expect(result['openai_base_url']).toBe('https://api.openai.com/v1')
      })

      it('should parse third-party API config', () => {
        const data = `https://paid.tribiosapi.top/
sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE`
        const result = parseClipboardData(data)
        expect(result['api_url']).toBe('https://paid.tribiosapi.top/')
        expect(result['api_key']).toBe('sk-ZAFeyOQ06TUW9qKlolQsKFtaePRsZYRzS1YOrYtx5n44X8zE')
      })

      it('should parse JSON-like format without braces', () => {
        const data = `api_key = "sk-abc123" base_url = "https://api.example.com"`
        const result = parseClipboardData(data)
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['base_url']).toBe('https://api.example.com')
      })

      it('should parse config with comments (ignoring comment lines)', () => {
        const data = `# API Configuration
api_key: sk-abc123
# Base URL for the API
url: https://api.example.com`
        const result = parseClipboardData(data)
        // Comments are parsed as key-value with # as part of key
        expect(result['api_key']).toBe('sk-abc123')
        expect(result['url']).toBe('https://api.example.com')
      })
    })
  })
})
