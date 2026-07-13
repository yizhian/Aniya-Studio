from src.models.schemas import DomContext


def messages_to_timeline(session: dict) -> list[dict]:
    """Convert old-format session messages to timeline entries.

    This is a backward-compat fallback for sessions saved before the unified
    timeline was introduced. Only used when the session has no 'timeline' field.
    """
    messages = session.get("messages", [])
    if not messages:
        return []

    tool_names: dict[str, str] = {}
    for msg in messages:
        for tc in msg.get("tool_calls", []) or []:
            tool_names[tc["id"]] = tc.get("function", {}).get("name", "unknown")

    entries: list[dict] = []
    for msg in messages:
        role = msg.get("role", "")

        if role == "system":
            continue

        if role == "user":
            entries.append({
                "event": "user_message",
                "data": {"content": msg.get("content", "")},
            })
            continue

        if role == "assistant":
            reasoning = msg.get("reasoning_content") or ""
            content = msg.get("content") or ""
            if reasoning:
                entries.append({
                    "event": "thinking",
                    "data": {"content": reasoning},
                })
            if content:
                entries.append({
                    "event": "text",
                    "data": {"content": content},
                })
            continue

        if role == "tool":
            call_id = msg.get("tool_call_id", "")
            name = tool_names.get(call_id, "unknown")
            content = msg.get("content", "")
            is_error = content.startswith("error")
            summary = content[:200] if len(content) > 200 else content
            entries.append({
                "event": "tool",
                "data": {
                    "phase": "result",
                    "call_id": call_id,
                    "name": name,
                    "success": not is_error,
                    "summary": summary,
                },
            })
            continue

    return entries


def format_chat_message(prompt: str, selected_dom: DomContext | None) -> str:
    """Format user prompt and optional DOM context into AgentGo message."""
    if not selected_dom:
        return prompt

    styles_str = ", ".join(
        f"{k}: {v}" for k, v in (selected_dom.styles or {}).items()
    )

    dom_prefix = (
        f"【DOM 元素选中】\n"
        f"- CSS 路径: {selected_dom.css_path}\n"
        f"- 标签: {selected_dom.tag}\n"
        f'- 文字内容: "{selected_dom.text}"\n'
        f"- 当前样式: {styles_str}\n"
        f"\n用户指令："
    )
    return dom_prefix + prompt
