export function formatImageConversationTitle(title: string, prompt = "") {
  return (title || prompt || "未命名会话")
    .replace(/^(生成|编辑)\s*[·•]\s*/u, "")
    .trim() || "未命名会话";
}
