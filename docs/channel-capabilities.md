# Channel Capabilities Matrix

This document tracks the normalized behaviors implemented in `server/internal/channel` so reply routing and rich payload handling stay predictable across channels.

## Reply Target Routing

| Channel | Default reply target | Single-use | Notes |
| --- | --- | --- | --- |
| `googlechat` | `thread_name` | no | Reply stays inside the source thread. |
| `line` | `reply_token` | yes | Only the first split chunk can reuse the token. |
| `slack` | `thread_ts -> reply_to_id -> message_id` | no | Keeps replies in an existing thread when present. |
| `mattermost` | `reply_to_id -> message_id` | no | Falls back to root post creation. |
| `nextcloudtalk` | `message_id` when replyable | no | Skips reply target when `is_replyable=false`. |
| `messenger` / `instagram` / `twitter` / `signal` / `viber` / `zalo` / `wechat_work` | unsupported | n/a | Manager intentionally leaves `ReplyToID` empty. |
| other channels | `message_id` | no | Default manager fallback. |

## Rich Payload Coverage

| Channel | Outbound coverage | Inbound normalization |
| --- | --- | --- |
| `slack` | text, attachments, blocks metadata | `thread_ts`, mentions, interaction actions |
| `discord` | text, attachments, embeds, components, `allowed_mentions` | mentions, embeds/cards, components, reply references |
| `teams` | text, attachments, cards, `channelData`, `entities` | attachments/cards, entities, mentions, timestamps |
| `whatsapp` | text, media, runtime `wacli` install path | media metadata and decrypted downloads |
| `nextcloudtalk` | text, reply target, Nextcloud file-share fallback | parent replies, `messageParameters`, reactions, markdown/edit metadata |
| `bluebubbles` | text fallback for attachments | subject fallback, attachment metadata/count, read timestamp |

## Practical Guardrails

- Prefer metadata passthrough over lossy channel-specific parsing.
- Only mark reply targets as single-use when the provider requires it.
- Keep unsupported reply channels explicit so split sends do not create broken references.
- Add a focused unit test whenever a channel gains new normalized metadata.
