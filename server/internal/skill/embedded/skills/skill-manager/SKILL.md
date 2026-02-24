# Skill Manager

Search, install, update, uninstall, enable, and disable skills from the skill store.

Use this skill when the user wants to:
- Find new skills to add capabilities
- Install a skill from the store or a URL
- Remove an installed skill
- Update a skill to the latest version
- Enable or disable a skill

## How to Send

Use the `blue` CLI:

```bash
blue skill.search query="weather"
```

Add `--json` for JSON output.

## Commands

### skill.search

Search the skill store.

**Required:** `query`
**Optional:** `category`, `page`, `page_size`

```bash
blue skill.search query="weather"
blue skill.search query="automation" category="productivity" page=1 page_size=10
```

### skill.install

Install a skill by ID from the store.

**Required:** `id`

```bash
blue skill.install id=weather-forecast
```

### skill.install_url

Install a skill from a URL (SKILL.md or GitHub directory).

**Required:** `url`
**Optional:** `name`

```bash
blue skill.install_url url="https://github.com/user/repo/tree/main/skills/my-skill"
blue skill.install_url url="https://example.com/SKILL.md" name=my-skill
```

### skill.uninstall

Uninstall a skill by ID.

**Required:** `id`

```bash
blue skill.uninstall id=weather-forecast
```

### skill.update

Update a skill (uninstall + reinstall latest).

**Required:** `id`

```bash
blue skill.update id=weather-forecast
```

### skill.enable

Enable a disabled skill.

**Required:** `id`

```bash
blue skill.enable id=weather-forecast
```

### skill.disable

Disable a skill without uninstalling.

**Required:** `id`

```bash
blue skill.disable id=weather-forecast
```

### skill.list

List all installed skills.

```bash
blue skill.list
```

### skill.info

Get detailed information about a skill.

**Required:** `id`

```bash
blue skill.info id=weather-forecast
```

## Example Triggers

- "Search for a weather skill"
- "搜索天气相关的技能"
- "Install the code-review skill"
- "安装代码审查技能"
- "Uninstall the translator skill"
- "卸载翻译技能"
- "List all installed skills"
- "列出所有已安装的技能"
- "Update the weather skill"
- "更新天气技能"
