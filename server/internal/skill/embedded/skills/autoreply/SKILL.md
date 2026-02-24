# Auto Reply

Create and manage auto-reply rules. When a message matches a trigger, an automatic response is sent.

## Usage

- "Add auto-reply: when someone says 'hello', reply 'Hi there!'"
- "List auto-reply rules"
- "Delete auto-reply rule #3"

## Trigger Types

- `keyword`: Exact match
- `contains`: Substring match
- `regex`: Regular expression
- `prefix`: Starts with
- `suffix`: Ends with

## Template Variables

- `{{user}}`: Sender name
- `{{message}}`: Original message
- `{{time}}`: Current time

## Parameters

- `action`: add, list, delete, enable, disable
- `trigger`: Trigger text/pattern (required for add)
- `trigger_type`: keyword, contains, regex, prefix, suffix (default: contains)
- `response`: Reply template (required for add)
