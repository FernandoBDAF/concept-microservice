"""EnrichmentAgent: extracts structured annotations from transcript chunks.

Future Improvements:
- Video-level tags: Currently context.tags are chunk-level (generated per chunk).
  For better retrieval filtering, consider:
  1. Backfilling metadata.tags from raw_videos.keywords (YouTube video tags)
  2. Generating video-level topic tags via LLM summarization of full transcript
  3. Propagating video-level tags to all chunks during ETL
- Cross-chunk entity resolution: Link entities across chunks for better relation graphs
- Hierarchical concepts: Group concepts into broader categories for filter organization
"""

from typing import Optional, List, Dict, Any
import json
import re

from src.core.base.agent import BaseAgent, BaseAgentConfig
from src.lib.error_handling.decorators import handle_errors


class EnrichmentAgent(BaseAgent):
    def __init__(self, model_name: Optional[str] = None):
        cfg = BaseAgentConfig(model_name=model_name)
        super().__init__(name="EnrichmentAgent", config=cfg)

    def build_chunk_structured_prompts(self, chunk_text: str) -> tuple[str, str]:
        system_prompt = (
            "You are EnrichAgent — an expert information extractor and semantic enrichment model "
            "that structures knowledge from YouTube video transcripts into machine-readable annotations.\n\n"
            "Your goal is to analyze one transcript chunk and produce a precise, compact JSON object "
            "following the Enriched Chunk Schema v2.\n\n"
            "Guiding principles:\n"
            "• Grounding: Extract only what is explicitly stated or logically implied.\n"
            "• Compactness: Keep text factual and concise (no filler, no restatement).\n"
            "• Schema adherence: Keys, types, and nesting must match exactly.\n"
            "• Reliability: When unsure, omit or assign low confidence.\n"
            "• Traceability: Provide short source_fragment snippets when useful for context.\n"
            "• Output strictly valid JSON — no markdown, prose, or commentary."
        )
        user_prompt = (
            "You will receive a transcript CHUNK TEXT and minimal metadata.\n"
            "Your task is to fill every field of the Enriched Chunk Schema v2 with grounded, "
            "compact information extracted from the text.\n\n"
            "Think step-by-step before writing JSON:\n"
            "1- Identify key facts, people, organizations, concepts, and actions.\n"
            "2- Note any temporal or numerical references (dates, amounts, scores, etc.).\n"
            "3- Observe the speaker's tone or sentiment if it's clearly expressed.\n"
            "4- Determine possible relations (subject-predicate-object) when verbs connect entities or concepts. Include conceptual relations between ideas (e.g., 'algorithm defines mapping between input and output').\n"
            "5- When nothing fits a category, leave that array empty or value null — never invent data.\n\n"
            "SCHEMA (strict, keep all keys — use [] or null when empty):\n"
            "{\n"
            '  "summary": string,\n'
            '  "entities": [ { "name": string, "type": "Person|Organization|Event|Concept|Location|Product|Other", "relevance": number, "source_fragment": string|null } ],\n'
            '  "concepts": [ { "name": string, "category": "Strategy|Emotion|Technical|Theme|Topic|Other", "confidence": number } ],\n'
            '  "relations": [ { "subject": string, "predicate": string, "object": string, "confidence": number } ],\n'
            '  "temporal_references": [ { "date": string, "context": string } ],\n'
            '  "numerical_data": [ { "value": string, "type": "money|percentage|count|chips|score|other", "context": string } ],\n'
            '  "visual_cues": [ { "type": "slide_title|onscreen_text|gesture|chart|other", "text": string|null } ],\n'
            '  "context": { "speaker": string|null, "sentiment": "positive|neutral|negative|null", "tone": "analytical|casual|instructional|narrative|humorous|null", "language": string|null, "tags": [string] }\n'
            "}\n\n"
            "INSTRUCTIONS:\n"
            "• Summary: 1-3 factual sentences summarizing the chunk.\n"
            "• Entities: Proper names only (people, places, orgs, events).\n"
            "• Concepts: Technical or thematic ideas (e.g., 'range balancing', 'probability theory').\n"
            "• Relations: Use verb-like predicates such as 'explains', 'uses', 'compares'.\n"
            "• Temporal/Numerical: Include only when clearly mentioned.\n"
            "• Visual Cues: Only if explicitly referenced (e.g., 'as shown on screen').\n"
            "• Context: Assign sentiment/tone when obvious; otherwise null.\n"
            "• Confidence/Relevance: floats in [0,1].\n"
            "• Keep all keys; output valid JSON only.\n\n"
            "EXAMPLE OUTPUT (abbreviated):\n"
            "{\n"
            '  "summary": "Phil Ivey explains how to balance betting ranges in tournament play.",\n'
            '  "entities": [ {"name": "Phil Ivey", "type": "Person", "relevance": 0.98, "source_fragment": "Phil Ivey"} ],\n'
            '  "concepts": [ {"name": "range balancing", "category": "Strategy", "confidence": 0.95} ],\n'
            '  "relations": [ {"subject": "Phil Ivey", "predicate": "explains", "object": "range balancing", "confidence": 0.9} ],\n'
            '  "temporal_references": [],\n'
            '  "numerical_data": [],\n'
            '  "visual_cues": [],\n'
            '  "context": { "speaker": "Phil Ivey", "sentiment": "analytical", "tone": "instructional", "language": "en", "tags": ["poker", "strategy"] }\n'
            "}\n\n"
            "Double-check consistency before outputting. Output ONLY the final JSON. Do not include the example in your output.\n"
            "You will be graded on JSON validity and adherence to the schema; non-compliant outputs receive a score of zero.\n\n"
            "CHUNK TEXT:\n"
            f"{chunk_text[:120000]}"
        )

        return system_prompt, user_prompt

    @handle_errors(
        fallback=lambda: {
            "summary": "",
            "entities": [],
            "concepts": [],
            "relations": [],
            "temporal_references": [],
            "numerical_data": [],
            "visual_cues": [],
            "context": {
                "speaker": None,
                "sentiment": None,
                "tone": None,
                "language": None,
                "tags": [],
            },
        },
        log_traceback=True,
        reraise=False,
    )
    def annotate_chunk_structured(self, chunk_text: str) -> Dict[str, Any]:
        system_prompt, user_prompt = self.build_chunk_structured_prompts(chunk_text)
        out = self.call_model(system_prompt, user_prompt)
        try:
            data = json.loads(out) if out else {}
            if isinstance(data, dict):
                return data
        except Exception:
            pass
        # Minimal fallback (empty annotations)
        return {
            "summary": "",
            "entities": [],
            "concepts": [],
            "relations": [],
            "temporal_references": [],
            "numerical_data": [],
            "visual_cues": [],
            "context": {
                "speaker": None,
                "sentiment": None,
                "tone": None,
                "language": None,
                "tags": [],
            },
        }
