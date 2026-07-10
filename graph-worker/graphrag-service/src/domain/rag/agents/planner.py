from typing import Any, Dict, List, Optional, Tuple
import json

from src.core.base.agent import BaseAgent, BaseAgentConfig
from src.lib.error_handling.decorators import handle_errors


class PlannerAgent(BaseAgent):
    def __init__(self, model_name: Optional[str] = None) -> None:
        cfg = BaseAgentConfig(model_name=model_name)
        super().__init__(name="PlannerAgent", config=cfg)
        self.last_system_prompt: Optional[str] = None
        self.last_user_prompt: Optional[str] = None

    def build_prompts(
        self,
        question: str,
        catalog: Dict[str, List[Any]],
        constraints: Dict[str, Any],
        conversation_context: Optional[str] = None,
    ) -> Tuple[str, str]:
        allowed_keys: List[str] = constraints.get("allowed_keys", [])
        defaults = constraints.get("defaults", {"mode": "auto", "k": 25})

        # FUTURE OPTIMIZATION: For long conversations (20+ turns), implement:
        # - Semantic retrieval of relevant conversation history
        # - Embed previous turns and retrieve only top-2 most relevant to current query
        # - Reduces planner context size while maintaining relevance
        # - Current: last 3 short-term + 2 long-term is sufficient

        system_prompt = (
            "CRITICAL RULE #1 - FILTER CONTINUITY (MUST FOLLOW):\n"
            "When CONVERSATION CONTEXT shows previous filters, you MUST include AT LEAST ONE previous filter value.\n"
            "- If Previous filters=[RAG, embeddings] and Current query='performance', use [RAG, performance] or [embeddings, performance]\n"
            "- If Previous filters=[A, B] and Current adds C, use [A or B, C] - keep continuity!\n"
            "- ONLY use completely different filters if user explicitly changes topic or says 'new topic'\n\n"
            "You are PlannerAgent. Decide how to retrieve and answer a question using a structured decision. "
            "Return strict JSON with keys: route, retrieval, filters, notes.\n\n"
            "Routing rules (hard):\n"
            "- If query contains any of ['guide','overview','blueprint','best practices','roadmap','topics','what should I cover'], route=topic_reference.\n"
            "- If query contains ['detailed', 'comprehensive', 'every topic', 'all aspects', 'technical document'], route=topic_reference.\n"
            "- If query is a single concrete how/why/which question, route=reference_answer.\n"
            "- If retrieval is diverse across tags/concepts (low overlap) or k is very large (>120), prefer topic_reference; if tightly clustered or small k, prefer reference_answer.\n"
            "Retrieval: {mode: vector|hybrid|keyword|auto, k: positive int}.\n"
            "Filters: choose ≤2 keys strictly from ALLOWED KEYS; use ONLY values present in the catalog samples.\n"
            "  - The catalog is PRE-FILTERED to show only values semantically relevant to the query.\n"
            "  - Prefer context.tags and concepts.name for topic-based queries.\n"
            "  - Use entities.name for people/organization queries.\n"
            "  - Use relations.subject for entity-relationship queries.\n"
            "  - If NO catalog values seem relevant to the query, use filters={}. Do NOT guess or invent values.\n"
            "  - Lists are permitted (will be converted to $in).\n"
            "k guidance:\n"
            "- topic_reference: k=200-300 (comprehensive multi-topic answers)\n"
            "- reference_answer + complex multi-part query: k=100-150\n"
            "- reference_answer + simple single question: k=40-80\n"
            "- Prefer higher k for quality; we optimize later\n"
            "Before returning JSON, include a one-sentence 'notes' explaining why these choices were made (for logs only)."
        )

        def _join(key: str, lim: int = 30) -> str:
            vals = catalog.get(key) or []
            return ", ".join([str(v) for v in vals[:lim]]) or "(none)"

        cat_str = (
            "Allowed filters (PRE-FILTERED to query-relevant values only):\n"
            f"- context.tags: {_join('context.tags')}\n"
            f"- concepts.name: {_join('concepts.name')}\n"
            f"- entities.name: {_join('entities.name')}\n"
            f"- relations.subject: {_join('relations.subject')}\n"
        )
        # Add conversation context with strong continuity reminder if present
        context_section = ""
        continuity_reminder = ""
        self_check = ""

        if conversation_context:
            context_section = (
                f"CONVERSATION CONTEXT (previous turns):\n{conversation_context}\n\n"
            )
            # Extract previous filter values from context for explicit reminder
            continuity_reminder = (
                "⚠️ REMINDER: CONVERSATION CONTEXT shows previous filters above.\n"
                "You MUST include AT LEAST ONE previous filter value to maintain continuity.\n"
                "Combine previous filters with new query terms - do NOT start fresh.\n\n"
            )
            self_check = (
                "\n\nSELF-CHECK before returning JSON:\n"
                "1. ✓ Did I read the CONVERSATION CONTEXT above?\n"
                "2. ✓ Did I identify the previous filters used?\n"
                "3. ✓ Did I include AT LEAST ONE previous filter value in my decision?\n"
                "4. ✓ Are my filters a COMBINATION of previous + new terms?\n"
                "If you answered NO to questions 3-4, GO BACK and combine filters properly.\n"
            )

        user_prompt = (
            f"{continuity_reminder}"
            f"QUESTION: {question}\n\n"
            f"{context_section}"
            f"{cat_str}\n"
            f"ALLOWED KEYS: {json.dumps(allowed_keys)}\n"
            f"DEFAULTS: {json.dumps(defaults)}\n"
            f"{self_check}\n"
            "Decide route/mode/k/filters. Keep k reasonable for performance; prefer fewer filters."
        )
        return system_prompt, user_prompt

    def decide(
        self,
        question: str,
        catalog: Dict[str, List[Any]],
        constraints: Dict[str, Any],
        conversation_context: Optional[str] = None,
    ) -> Dict[str, Any]:
        try:
            system_prompt, user_prompt = self.build_prompts(
                question, catalog, constraints, conversation_context
            )
            self.last_system_prompt = system_prompt
            self.last_user_prompt = user_prompt
            # Ask for strict JSON-only output if supported
            out = self.call_model(
                system_prompt,
                user_prompt,
                response_format={"type": "json_object"},
            )
            data = json.loads(out or "{}")
            if not isinstance(data, dict) or not data:
                raise ValueError("non-json or empty")
            return data
        except Exception:
            # Retry with a repair system prompt
            try:
                repair_system = "Return STRICT JSON ONLY. No prose."
                out2 = self.call_model(
                    repair_system, user_prompt, response_format={"type": "json_object"}
                )
                data2 = json.loads(out2 or "{}")
                return (
                    data2
                    if isinstance(data2, dict) and data2
                    else self._fallback(question)
                )
            except Exception:
                return self._fallback(question)

    def _fallback(self, question: str) -> Dict[str, Any]:
        q = (question or "").lower()
        is_overview = any(
            w in q
            for w in ["guide", "overview", "blueprint", "best practices", "roadmap"]
        )
        return {
            "route": "topic_reference" if is_overview else "reference_answer",
            "retrieval": {"mode": "vector", "k": 100 if is_overview else 40},
            "filters": {},
            "notes": "fallback heuristic",
        }
