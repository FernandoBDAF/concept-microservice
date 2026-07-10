from typing import Optional, Dict, Any
import json

from src.core.base.agent import BaseAgent, BaseAgentConfig
from src.lib.error_handling.decorators import handle_errors


class TrustRankAgent(BaseAgent):
    def __init__(self, model_name: Optional[str] = None) -> None:
        cfg = BaseAgentConfig(model_name=model_name)
        super().__init__(name="TrustRankAgent", config=cfg)

    def build_prompts(self, payload: Dict[str, Any]) -> tuple[str, str]:
        system_prompt = (
            "You are TrustRankAgent. Estimate chunk trustworthiness for retrieval, considering: consensus "
            "(duplication/agreement), recency, engagement, and code validity. Be concise and evidence-based."
        )
        user_prompt = (
            "You will read the INPUT JSON and output strict JSON with a normalized trust score.\n\n"
            "INSTRUCTIONS (think step-by-step):\n"
            "1) Consensus: higher redundancy indicates wider agreement, but downrank if content is trivial.\n"
            "2) Recency: newer is generally better; apply a smooth advantage for recent content.\n"
            "3) Engagement: normalize likes/comments/views proxy if available.\n"
            "4) Code validity: prefer chunks mentioning valid or runnable code.\n"
            '5) Output: VALID JSON ONLY: {"trust_score": float 0-1, "reason": "short evidence"}.\n\n'
            "GOOD EXAMPLE:\n"
            '{"trust_score": 0.62, "reason": "Recent chunk with matching explanations across videos; simple code example"}\n\n'
            "BAD EXAMPLES:\n"
            "- Prose explanations without JSON.\n"
            "- Hallucinated metrics or sources.\n\n"
            "INPUT JSON:\n" + json.dumps(payload)[:120000]
        )
        return system_prompt, user_prompt

    @handle_errors(fallback=lambda *args, **kwargs: {"trust_score": None, "reason": "fallback"}, log_traceback=True, reraise=False)
    def score(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        try:
            system_prompt, user_prompt = self.build_prompts(payload)
            out = self.call_model(system_prompt, user_prompt)
            if not out:
                return {"trust_score": None, "reason": "no-llm"}
            data = json.loads(out)
            score = data.get("trust_score")
            reason = data.get("reason", "")
            try:
                score_f = float(score)
            except Exception:
                score_f = None
            return {"trust_score": score_f, "reason": reason}
        except Exception:
            return {"trust_score": None, "reason": "fallback"}
