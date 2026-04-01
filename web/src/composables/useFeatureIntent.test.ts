import { describe, expect, it } from 'vitest'
import { classifyFeatureIntent } from './useFeatureIntent'

describe('classifyFeatureIntent', () => {
  it('returns no intent for empty input', () => {
    expect(classifyFeatureIntent('   ')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('keeps legacy deep research wording working', () => {
    expect(classifyFeatureIntent('Please do deep research and include citations')).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('maps research mode wording to the research capability', () => {
    expect(
      classifyFeatureIntent('Please use research mode and verify sources before answering')
    ).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('maps Chinese research mode wording to the research capability', () => {
    expect(classifyFeatureIntent('这次请开启研究模式，并核验引用来源')).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('keeps explicit research mode detection stable around punctuation and case', () => {
    expect(classifyFeatureIntent('(RESEARCH MODE), please verify sources first.')).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('does not confuse research model with research mode', () => {
    expect(classifyFeatureIntent('Please use the research model selector in settings')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on quoted documentation mentions of feature labels', () => {
    expect(
      classifyFeatureIntent('The docs should mention "research mode" and "agent mode" by name.')
    ).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on backticked command and label mentions', () => {
    expect(
      classifyFeatureIntent('Document the `/research` command and the `research mode` label.')
    ).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on Chinese quoted documentation mentions of feature labels', () => {
    expect(classifyFeatureIntent('文档里写上“研究模式”和“智能体模式”这两个词')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on unquoted documentation mentions of feature labels', () => {
    expect(
      classifyFeatureIntent('The docs should mention research mode and agent mode by name.')
    ).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on unquoted help-copy mentions of research mode', () => {
    expect(classifyFeatureIntent('Please add research mode to the help text for settings.')).toEqual(
      {
        deepResearch: false,
        agentMode: false,
      }
    )
  })

  it('does not trigger on Chinese unquoted documentation mentions of feature labels', () => {
    expect(classifyFeatureIntent('把研究模式和智能体模式写进帮助文案里')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on rename-style UI copy edits for research mode', () => {
    expect(classifyFeatureIntent('Rename research mode to Research in the settings copy.')).toEqual(
      {
        deepResearch: false,
        agentMode: false,
      }
    )
  })

  it('does not trigger on button layout edits for agent mode', () => {
    expect(classifyFeatureIntent('Move the agent mode button below the composer.')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on legacy deep research rename edits for menu labels', () => {
    expect(classifyFeatureIntent('Rename Deep Research to Research in the menu label.')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on relabel-style heading edits for agent mode', () => {
    expect(classifyFeatureIntent('Relabel agent mode as Agents in the heading.')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on Chinese rename-style UI copy edits for research mode', () => {
    expect(classifyFeatureIntent('把研究模式改成研究，并更新设置页文案')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on Chinese button-copy edits for agent mode', () => {
    expect(classifyFeatureIntent('把智能体模式按钮移动到底部，并调整按钮文案')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger on Chinese relabel-style title edits for agent mode', () => {
    expect(classifyFeatureIntent('把智能体模式重命名为智能体，并更新标题')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('still honors directly requested quoted research mode', () => {
    expect(classifyFeatureIntent('Please use "research mode" for this answer')).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('still honors directly requested quoted agent mode', () => {
    expect(classifyFeatureIntent('Please run in "agent mode" for this task')).toEqual({
      deepResearch: false,
      agentMode: true,
    })
  })

  it('still honors directly requested unquoted agent mode', () => {
    expect(classifyFeatureIntent('Please run in agent mode for this task')).toEqual({
      deepResearch: false,
      agentMode: true,
    })
  })

  it('still honors direct requests even when UI copy is also mentioned', () => {
    expect(
      classifyFeatureIntent('Use research mode for this answer, then update the button copy later.')
    ).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('detects research intent from action-plus-target combinations', () => {
    expect(
      classifyFeatureIntent('Please research thoroughly and include citations plus evidence')
    ).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('detects Chinese research intent from action-plus-target combinations', () => {
    expect(classifyFeatureIntent('请帮我查阅来源和证据，并做个梳理')).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('does not trigger composite research intent when targets are missing', () => {
    expect(classifyFeatureIntent('Please research thoroughly before answering')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger Chinese composite research intent when targets are missing', () => {
    expect(classifyFeatureIntent('请帮我查阅并梳理一下')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('respects research mode negations', () => {
    expect(classifyFeatureIntent('disable research mode and just answer directly')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('respects Chinese research mode negations', () => {
    expect(classifyFeatureIntent('这次不用研究模式，直接给我简短回答')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('lets research negations override composite research cues', () => {
    expect(
      classifyFeatureIntent(
        'Please research thoroughly with citations and evidence, but without research mode'
      )
    ).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('lets mixed-language research negations override explicit research mode requests', () => {
    expect(classifyFeatureIntent('Please use research mode，但这次不用研究模式')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('can enable research and agent mode together when both are requested', () => {
    expect(
      classifyFeatureIntent(
        'Use research mode first, then run in agent mode to execute the workflow'
      )
    ).toEqual({
      deepResearch: true,
      agentMode: true,
    })
  })

  it('keeps research enabled when agent mode is explicitly disabled', () => {
    expect(
      classifyFeatureIntent('Use research mode for sources, but disable agent mode for execution')
    ).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('keeps agent mode enabled when research mode is explicitly disabled', () => {
    expect(
      classifyFeatureIntent(
        'Disable research mode this time, but run in agent mode to execute the task'
      )
    ).toEqual({
      deepResearch: false,
      agentMode: true,
    })
  })

  it('keeps Chinese research intent when Chinese agent mode is explicitly disabled', () => {
    expect(classifyFeatureIntent('请开启研究模式，但不要智能体模式')).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('keeps Chinese agent intent when Chinese research mode is explicitly disabled', () => {
    expect(classifyFeatureIntent('不要研究模式，但请自动执行这个任务')).toEqual({
      deepResearch: false,
      agentMode: true,
    })
  })

  it('does not confuse agent model with agent mode', () => {
    expect(classifyFeatureIntent('The agent model should stay on auto routing')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('detects Chinese agent intent from action-plus-target combinations', () => {
    expect(classifyFeatureIntent('请自动执行这个任务并分步完成')).toEqual({
      deepResearch: false,
      agentMode: true,
    })
  })

  it('does not trigger composite agent intent when task targets are missing', () => {
    expect(classifyFeatureIntent('Please be autonomous and careful')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('does not trigger Chinese composite agent intent when targets are missing', () => {
    expect(classifyFeatureIntent('请自动执行一下')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('suppresses definition questions about research mode', () => {
    expect(classifyFeatureIntent('what is research mode')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('suppresses Chinese definition questions about research mode', () => {
    expect(classifyFeatureIntent('什么是研究模式')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('suppresses mixed-language definition questions about research mode', () => {
    expect(classifyFeatureIntent('介绍一下 research mode')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('suppresses courtesy-prefixed definition questions about research mode', () => {
    expect(classifyFeatureIntent('请介绍一下 research mode')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('suppresses courtesy-prefixed definition questions about agent mode', () => {
    expect(classifyFeatureIntent('请介绍一下 agent mode')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('suppresses english explain-style definition questions about research mode', () => {
    expect(classifyFeatureIntent('please explain research mode')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('suppresses english tell-me-about definition questions about agent mode', () => {
    expect(classifyFeatureIntent('tell me about agent mode')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('suppresses how-to questions about enabling research mode', () => {
    expect(classifyFeatureIntent('how do i enable research mode')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })

  it('keeps a trailing research request after a definition question intentful', () => {
    expect(
      classifyFeatureIntent('what is research mode? please use research mode for this answer')
    ).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('keeps a trailing Chinese research request after a definition question intentful', () => {
    expect(classifyFeatureIntent('什么是研究模式？然后开启研究模式')).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('keeps a trailing agent request after a definition question intentful', () => {
    expect(
      classifyFeatureIntent('what is research mode? then run in agent mode for this task')
    ).toEqual({
      deepResearch: false,
      agentMode: true,
    })
  })

  it('keeps a trailing courtesy-prefixed Chinese research request after a definition question intentful', () => {
    expect(classifyFeatureIntent('请解释一下 research mode，然后开启研究模式')).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('keeps a trailing request after two chained definition clauses intentful', () => {
    expect(
      classifyFeatureIntent('what is research mode? what is agent mode? then use research mode')
    ).toEqual({
      deepResearch: true,
      agentMode: false,
    })
  })

  it('keeps a trailing request after english explain-style definition clauses intentful', () => {
    expect(
      classifyFeatureIntent('please explain research mode, then run in agent mode for this task')
    ).toEqual({
      deepResearch: false,
      agentMode: true,
    })
  })

  it('suppresses definition questions even when they also mention the other feature', () => {
    expect(classifyFeatureIntent('what is research mode and what is agent mode')).toEqual({
      deepResearch: false,
      agentMode: false,
    })
  })
})
