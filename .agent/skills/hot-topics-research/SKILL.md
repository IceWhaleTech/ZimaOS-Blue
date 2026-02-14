---
name: hot-topics-research
description: |
  Use when conducting market research on social media platforms (X/Twitter, YouTube, Reddit)
  for product intelligence, competitive analysis, and trend discovery.

  Triggers: product planning, marketing strategy, user research, competitor analysis,
  feature prioritization, content strategy, opportunity identification.

  This skill provides a systematic approach to discovering high-engagement content,
  extracting user stories, and analyzing market sentiment from social platforms.
---

# Hot Topics Research Skill

This skill provides a systematic methodology for researching high-engagement topics on social media platforms to inform product decisions and marketing strategies.

## When to Use This Skill

- **Product Planning**: Discover user needs and pain points before building features
- **Marketing Strategy**: Identify viral content patterns and messaging hooks
- **Competitive Analysis**: Understand how competitor products are discussed
- **User Research**: Extract real user stories from social conversations
- **Feature Prioritization**: Validate demand through social engagement metrics
- **Content Strategy**: Learn what content resonates with target audiences

---

## Research Framework

### Phase 1: Initial Discovery (Platform Agnostic)

**Goal**: Broad understanding of the topic landscape

**Search Queries Template**:
```
"{product_name}" viral 2024 2025
"{product_name}" popular posts
"{product_name}" user stories
"{product_name}" use cases examples
"{product_name}" problems complaints
```

**Data Points to Collect**:
- Overall mention volume
- Key platforms where discussion happens
- Primary sentiment (positive/neutral/negative)
- Emerging themes or patterns

---

### Phase 2: Platform-Specific Deep Dive

#### 2.1 X.com (Twitter) Research

**Search Strategies**:

**Basic Searches**:
```
site:x.com "{product_name}" viral
site:x.com "{product_name}" likes
site:x.com "{product_name}" trending
```

**User Story Discovery**:
```
site:x.com "{product_name}" "I use it for"
site:x.com "{product_name}" "my experience"
site:x.com "{product_name}" "helped me"
```

**Influencer Identification**:
```
site:x.com "{product_name}" from:{known_influencer}
site:x.com "{product_name}" developers
site:x.com "{product_name}" official
```

**Engagement Filters**:
- Look for posts with 1,000+ likes/retweets
- Prioritize threads with 100+ replies
- Focus on posts with media (images, videos, demos)

**Key Metrics to Record**:
- Engagement rate (likes + retweets / followers)
- Reply sentiment analysis
- Thread depth and branching
- Follower count of poster

#### 2.2 YouTube Research

**Search Strategies**:
```
"{product_name}" tutorial 2024 2025
"{product_name}" use cases
"{product_name}" review
"{product_name}" setup guide
"{product_name}" vs "{competitor}"
```

**Content Analysis**:
- View count (10K+ = notable, 100K+ = viral)
- Upload date (focus on last 6 months)
- Comment sentiment (read top 20 comments)
- Creator expertise level

**Key Metrics to Record**:
- View count growth rate
- Like-to-view ratio (>5% is good)
- Comment themes and questions
- Creator subscriber count

#### 2.3 Reddit Research

**Search Strategies**:
```
site:reddit.com/r/{relevant_subreddit} "{product_name}"
site:reddit.com "{product_name}" experience
site:reddit.com "{product_name}" vs
```

**Key Subreddits by Product Category**:
- AI/ML: r/LocalLLaMA, r/LocalLLM, r/MachineLearning
- Dev Tools: r/programming, r/devtools, r/SideProject
- Smart Home: r/homeassistant, r/smarthome, r/MyAutomation
- Productivity: r/productivity, r/selfhosted
- Crypto: r/defi, r/cryptocurrency

**Key Metrics to Record**:
- Upvote ratio (>70% positive)
- Award count
- Comment depth of discussion
- User flair analysis (expert vs newbie)

---

### Phase 3: User Story Extraction

**The 2-Action Rule**: After every 2 view/search operations, save key findings to organized files.

**User Story Template**:
```
BEFORE: [State the user's situation before using the product]
  - Pain points: ...
  - Time spent: ...
  - Frustrations: ...

ACTION: [What they did with the product]
  - Specific use case: ...
  - Platform/channel: ...
  - Trigger: ...

AFTER: [The outcome and benefit]
  - Time saved: ...
  - Problems solved: ...
  - Emotional state: ...
```

**Story Classification**:
- **Viral Stories**: High sharing potential, emotional resonance
- **Practical Stories**: Clear utility, specific use cases
- **Warning Stories**: Risk awareness, failures to avoid
- **Comparison Stories**: vs alternatives, migration stories

---

### Phase 4: Kano Model Analysis

For each identified feature/use case, categorize user sentiment:

**Categories**:

| Type | Definition | User Reaction | Examples |
|------|-----------|--------------|----------|
| **Basic (Must-Have)** | Expected features, absence causes dissatisfaction | "This doesn't work?" | Core functionality |
| **Performance (More is Better)** | Linear satisfaction with quality/quantity | "This is great" | Speed, accuracy |
| **Excitement (Delighters)** | Unexpected capabilities that wow users | "Wow!" features | Proactive features |
| **Indifferent** | Presence doesn't matter | "Oh, okay" | Nice-to-haves |
| **Reverse** | More is worse | "Stop doing that" | Spam, nagging |

**Analysis Framework**:
- **User Attention Points**: What do users care about most?
- **Viral Points**: What makes users want to share?
- **Concern Points**: What worries or stops users?

---

### Phase 5: Three-Part Value Proposition

For each high-priority use case, structure as:

**1. One-Sentence Story**
> A compelling headline that captures the transformation

**2. Scenario Description & Frequency**
- Detailed description of how it works
- How often users engage with it
- Context and triggers

**3. Before & After Comparison**

| Dimension | Before | After |
|-----------|--------|-------|
| Time spent | [X minutes/hours] | [Y minutes/hours] |
| Pain level | [Description] | [Description] |
| Outcome | [Result] | [Result] |
| Emotional state | [Feeling] | [Feeling] |

---

## Output Structure

Create research artifacts in the `MKT/` directory:

```
MKT/
├── {product}-overview.md           # Product overview and key findings
├── {product}-hot-topics.md          # Viral topics and trending content
├── {product}-influencer-analysis.md  # Key opinion leaders and their impact
├── {product}-use-cases.md            # User stories and use cases
└── {product}-kano-analysis.md        # Feature analysis and sentiment
```

---

## Search Query Templates

### Viral Content Discovery
```
site:x.com "{product}" viral
site:x.com "{product}" blew up
site:x.com "{product}" trending
"{product}" went viral
"{product}" took off
```

### User Experience Research
```
site:x.com "{product}" "I use"
site:x.com "{product}" "my workflow"
site:x.com "{product}" "helped me"
site:x.com "{product}" "saved me"
```

### Problem & Complaint Discovery
```
site:x.com "{product}" "problem"
site:x.com "{product}" "issue"
site:x.com "{product}" "doesn't work"
site:x.com "{product}" "frustrating"
site:x.com "{product}" "confusing"
```

### Comparison Research
```
site:x.com "{product}" vs "{competitor}"
site:x.com "{product}" alternative
site:x.com "{product}" better than
site:x.com "{product}" instead of
```

### YouTube Tutorial Search
```
"{product}" tutorial 2024
"{product}" walkthrough
"{product}" setup guide
"{product}" use cases
"{product}" examples
```

### Reddit Discussion Search
```
site:reddit.com/r/{subreddit} "{product}"
"{product}" reddit
"{product}" experience
"{product}" thoughts on
```

---

## Data Collection Checklist

For each research session, document:

**Source Metadata**:
- [ ] Platform (X/YouTube/Reddit)
- [ ] Search query used
- [ ] Date range
- [ ] Result count

**Content Analysis**:
- [ ] Top posts/titles
- [ ] Engagement metrics (likes, views, upvotes)
- [ ] Key themes identified
- [ ] Notable quotes (verbatim)
- [ ] User sentiment distribution

**Stakeholder Mapping**:
- [ ] Key influencers identified
- [ ] Their follower counts
- [ ] Their engagement rates
- [ ] Their relationship to product

**Output Files**:
- [ ] Raw data saved to MKT/
- [ ] Analysis document created
- [ ] Key findings summarized
- [ ] Action items identified

---

## Quality Criteria

**Red Flags (Low-Quality Signals)**:
- Sample size < 30 mentions
- Engagement rate < 1%
- Posts older than 12 months
- Obvious bot/astroturfing activity
- Single-platform data only

**Green Flags (High-Quality Signals)**:
- Cross-platform validation
- Multiple independent sources
- Recent activity (last 3 months)
- Specific, detailed user stories
- Verifiable engagement metrics
- Influencer diversity (not just one person)

---

## Common Pitfalls to Avoid

1. **Confirmation Bias**: Don't only search for positive feedback
2. **Recency Bias**: Balance trending topics with evergreen content
3. **Vocal Minority**: Don't mistake loud complaints for majority opinion
4. **Platform Bubble**: Different platforms have different demographics
5. **Engagement Gaming**: High numbers don't always mean genuine interest

---

## Tips for Effective Research

1. **Start Broad, Then Narrow**: Begin with general searches, then drill down
2. **Follow the Thread**: Click through to see full conversations
3. **Check Dates**: Prioritize recent content but validate with historical data
4. **Look for Patterns**: Single data points are noise, patterns are signal
5. **Verify Claims**: Cross-reference across multiple sources
6. **Document Everything**: You won't remember context later
7. **Update Regularly**: Social trends change quickly

---

## Integrating with Project Work

When applying this skill to ZimaOS-Blue:

1. **Target Subreddits**: r/homeassistant, r/smarthome, r/selfhosted, r/LocalLLaMA
2. **Key Competitors**: Clawdbot/Moltbot, Home Assistant, n8n, Zapier
3. **Content Themes**: Local AI, privacy, automation, smart home, NAS-based solutions
4. **Value Proposition**: Emphasize what Blue does better/differently

---

## Example Research Workflow

**Scenario**: Researching local AI assistant market trends

```
1. Initial Discovery
   → Search "local AI assistant" viral 2024
   → Search "self-hosted AI" trending
   → Identify: Clawdbot/Moltbot as hot topic

2. X.com Deep Dive
   → site:x.com Clawdbot viral
   → Find key posts with 10K+ likes
   → Extract user stories from replies

3. YouTube Analysis
   → "Clawdbot tutorial 2024"
   → "Clawdbot use cases"
   → Watch top 5 videos for patterns

4. Reddit Validation
   → r/LocalLLM Clawdbot
   → Read top 20 posts
   → Validate themes from X/YouTube

5. Synthesis
   → Create MKT/clawdbot-hot-topics.md
   → Create MKT/clawdbot-use-cases.md
   → Identify opportunities for Blue
```

---

## Sources

**Primary Research Platforms**:
- X.com (Twitter) - Real-time trends and influencer opinions
- YouTube - Tutorial content and visual demonstrations
- Reddit - In-depth discussions and user experiences
- Hacker News - Technical community feedback
- Medium - Thought leadership and case studies

**Secondary Sources**:
- Tech blogs and news sites
- Product documentation and official showcases
- GitHub repositories and discussions
- Industry analyst reports

---

**Last Updated**: 2026-01-29
**Version**: 1.0
