import { describe, expect, it } from 'vitest'
import { parseFrontmatter } from '@/utils/frontmatter'

describe('parseFrontmatter', () => {
  it('returns original body when frontmatter is missing', () => {
    const src = '# Title\n\nHello world'
    const parsed = parseFrontmatter(src)
    expect(parsed.hasFrontmatter).toBe(false)
    expect(parsed.entries).toEqual([])
    expect(parsed.body).toBe(src)
  })

  it('extracts key-value frontmatter and markdown body', () => {
    const src = `---
name: demo
description: "Skill desc"
tags: [search, web]
---
# Demo

Body`

    const parsed = parseFrontmatter(src)
    expect(parsed.hasFrontmatter).toBe(true)
    expect(parsed.body).toContain('# Demo')
    expect(parsed.entries).toContainEqual({ key: 'name', value: 'demo' })
    expect(parsed.entries).toContainEqual({ key: 'description', value: 'Skill desc' })
    expect(parsed.entries).toContainEqual({ key: 'tags', value: 'search, web' })
  })

  it('extracts multiline list values', () => {
    const src = `---
os:
  - darwin
  - linux
---
Doc`

    const parsed = parseFrontmatter(src)
    expect(parsed.entries).toContainEqual({ key: 'os', value: 'darwin, linux' })
    expect(parsed.body).toBe('Doc')
  })
})
