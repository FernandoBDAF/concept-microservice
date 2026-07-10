from typing import Any, Dict, List, Optional, Tuple

from src.core.base.agent import BaseAgent, BaseAgentConfig
from src.lib.error_handling.decorators import handle_errors


class TopicReferenceAgent(BaseAgent):
    def __init__(self, model_name: Optional[str] = None) -> None:
        cfg = BaseAgentConfig(model_name=model_name, max_tokens=12000)
        super().__init__(name="TopicReferenceAgent", config=cfg)

    def build_prompts(
        self,
        question: str,
        topic_bundles: List[Dict[str, Any]],
    ) -> Tuple[str, str]:
        system_prompt = (
            "You are TopicReferenceAgent. Produce a DETAILED, implementation-focused answer grouped by topics.\n"
            "CRITICAL: Every bullet and reference MUST be grounded in the CONTEXT snippets provided. Do NOT use external knowledge.\n\n"
            "Structure:\n"
            "- Create 3–6 topics based on themes in CONTEXT\n"
            "- Per topic: write 3–5 substantial bullets (2-3 sentences each) with SPECIFIC facts, examples, and technical details from CONTEXT\n"
            "- Add 1–2 references per topic using ONLY the URLs provided in CONTEXT: Title — URL (with start time) — 'Where to start' hint\n"
            "- Prefer diversity across topics; avoid duplicate references\n"
            "- Use inline citations to snippets when making technical claims\n"
            "- Be comprehensive and detailed; do NOT compress or summarize excessively\n"
            "- NEVER invent URLs, facts, or details not present in CONTEXT"
        )

        rendered: List[str] = []
        for i, t in enumerate(topic_bundles[:8], start=1):
            name = t.get("topic") or f"Topic {i}"
            bullets = t.get("bullets", [])
            quotes = t.get("quotes", [])
            refs = t.get("refs", [])
            ref_lines = [
                f"- {r.get('title','')} — {r.get('url','')} ({r.get('hint','')})"
                for r in refs[:2]
            ]
            rendered.append(
                "\n".join(
                    [
                        f"[TOPIC] {name}",
                        "BULLETS:",
                        *[f"- {x}" for x in bullets[:6]],
                        "QUOTES:",
                        *[f'"{q[:200]}"' for q in quotes[:2]],
                        "CANDIDATE REFS:",
                        *ref_lines,
                    ]
                )
            )

        user_prompt = (
            "QUESTION: "
            + question
            + "\n\nCONTEXT (grouped by topics):\n"
            + "\n\n".join(rendered)
            + "\n\nREQUIREMENTS:\n"
            + "1) Group answer by 3-6 topics with descriptive headings\n"
            + "2) Write 3-5 SUBSTANTIAL bullets per topic (2-3 sentences each):\n"
            + "   - Include specific technical details, examples, and data from CONTEXT\n"
            + "   - Quote or paraphrase key points from BULLETS and QUOTES\n"
            + "   - Use concrete facts, not generic statements\n"
            + "3) Add 1-2 references per topic:\n"
            + "   - Use ONLY URLs from CANDIDATE REFS\n"
            + "   - Format: Title — URL (start mm:ss) — 'Where to start: [specific guidance]'\n"
            + "4) Avoid duplicate references across topics\n"
            + "5) Be detailed and comprehensive; aim for 1500-2000 words total\n"
            + "6) CRITICAL: Do NOT invent facts, URLs, or details not in CONTEXT"
        )
        return system_prompt, user_prompt

    @handle_errors(fallback=lambda self, question, topic_bundles: self._fallback(topic_bundles), log_traceback=True, reraise=False)
    def answer(self, question: str, topic_bundles: List[Dict[str, Any]]) -> str:
        try:
            system_prompt, user_prompt = self.build_prompts(question, topic_bundles)
            out = self.call_model(system_prompt, user_prompt)
            return out or self._fallback(topic_bundles)
        except Exception:
            return self._fallback(topic_bundles)

    def _fallback(self, topic_bundles: List[Dict[str, Any]]) -> str:
        lines: List[str] = ["(No LLM configured) Topic summary:\n"]
        for t in topic_bundles[:5]:
            lines.append(f"## {t.get('topic','Topic')}\n")
            for b in (t.get("bullets") or [])[:4]:
                lines.append(f"- {b}")
            refs = t.get("refs") or []
            if refs:
                lines.append("References:")
                for r in refs[:2]:
                    lines.append(
                        f"- {r.get('title','')} — {r.get('url','')} ({r.get('hint','')})"
                    )
            lines.append("")
        return "\n".join(lines)
