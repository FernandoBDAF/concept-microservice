from typing import Any, Dict, List, Optional, Tuple

from src.core.base.agent import BaseAgent, BaseAgentConfig
from src.lib.error_handling.decorators import handle_errors


class ReferenceAnswerAgent(BaseAgent):
    def __init__(self, model_name: Optional[str] = None) -> None:
        cfg = BaseAgentConfig(model_name=model_name)
        super().__init__(name="ReferenceAnswerAgent", config=cfg)

    def build_prompts(
        self,
        question: str,
        doc_bundles: List[Dict[str, Any]],
    ) -> Tuple[str, str]:
        system_prompt = (
            "You are ReferenceAnswerAgent, a precise educator.\n"
            "- Answer concisely using the provided per-document context.\n"
            "- Add 2–3 practical references total. For each reference: Title — URL (with start time if given) — a short 'Where to start' hint.\n"
            "- Prefer highest-similarity, tightly clustered chunks; avoid broad coverage.\n"
            "- Do not hallucinate links or content beyond the provided context."
        )

        # Render a compact, structured context for the model
        rendered: List[str] = []
        for i, b in enumerate(doc_bundles[:6], start=1):
            title = b.get("title") or b.get("video_id")
            url = b.get("url") or ""
            anchor = b.get("anchor_hint") or ""
            bullets = b.get("bullets", [])
            quotes = b.get("quotes", [])
            rendered.append(
                "\n".join(
                    [
                        f"[DOC {i}] {title}",
                        f"URL: {url}",
                        f"ANCHOR: {anchor}",
                        "BULLETS:",
                        *[f"- {x}" for x in bullets[:6]],
                        "QUOTES:",
                        *[f'"{q[:240]}"' for q in quotes[:2]],
                    ]
                )
            )

        user_prompt = (
            "QUESTION: "
            + question
            + "\n\nCONTEXT (per-document):\n"
            + "\n\n".join(rendered)
            + "\n\nREQUIREMENTS:\n"
            + "1) Keep the answer concise and grounded in the context.\n"
            + "2) Include a 'References' section with at most 3 items, each formatted as: \n"
            + "   - Title — URL (start mm:ss): Where to start …\n"
            + "3) If start time is unknown, omit it and provide a short hint instead.\n"
        )
        return system_prompt, user_prompt

    @handle_errors(fallback=lambda *args, **kwargs: args[1][0].get("title", "No answer available") if args[1] else "", log_traceback=True, reraise=False)
    def answer(self, question: str, doc_bundles: List[Dict[str, Any]]) -> str:
        try:
            system_prompt, user_prompt = self.build_prompts(question, doc_bundles)
            out = self.call_model(system_prompt, user_prompt)
            return out or self._fallback(doc_bundles)
        except Exception:
            return self._fallback(doc_bundles)

    def _fallback(self, doc_bundles: List[Dict[str, Any]]) -> str:
        lines: List[str] = ["(No LLM configured) Context summary:\n"]
        for b in doc_bundles[:3]:
            title = b.get("title") or b.get("video_id")
            url = b.get("url") or ""
            anchor = b.get("anchor_hint") or ""
            bullets = b.get("bullets", [])
            lines.append(f"- {title} — {url}  {anchor}")
            for bl in bullets[:3]:
                lines.append(f"  • {bl}")
        return "\n".join(lines)
