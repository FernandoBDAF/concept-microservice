from typing import Optional, Tuple

from src.core.base.agent import BaseAgent, BaseAgentConfig
from src.lib.error_handling.decorators import handle_errors


class TranscriptCleanAgent(BaseAgent):
    """
    Agent for cleaning raw transcripts into fluent, readable text.
    
    Supports dynamic prompt selection via prompt_id parameter.
    Falls back to hardcoded prompts if no registry prompt is available.
    """
    
    def __init__(
        self,
        model_name: Optional[str] = None,
        prompt_id: Optional[str] = None
    ):
        cfg = BaseAgentConfig(model_name=model_name)
        super().__init__(
            name="TranscriptCleanAgent",
            config=cfg,
            prompt_id=prompt_id  # Enable dynamic prompt selection
        )

    def build_prompts(self, **kwargs) -> Tuple[str, str]:
        """
        Build hardcoded prompts for transcript cleaning.
        
        This is the fallback when no registry prompt is available.
        
        Args:
            raw_text: The raw transcript text to clean
            
        Returns:
            Tuple of (system_prompt, user_prompt)
        """
        raw_text = kwargs.get("raw_text", "")
        
        system_prompt = (
            "You are CleanerAgent — an expert linguistic editor specialized in transforming "
            "raw or auto-generated YouTube transcripts into fluent, readable text while "
            "preserving meaning and language fidelity.\n\n"
            "Your goal: normalize and clean the transcript while keeping all technical content, "
            "mathematical notation, and speaker cues intact.\n\n"
            "Guiding principles:\n"
            "• Fidelity: Never paraphrase, summarize, or add new information.\n"
            "• Structure: Produce coherent paragraphs with natural flow.\n"
            "• Integrity: Preserve all code, formulas, numbers, and symbols exactly as written.\n"
            "• Speaker Cues: Keep or standardize '[Name]:' or '[Speaker X]:' markers.\n"
            "• Clarity: Fix casing, punctuation, and spacing; remove fillers and noise.\n"
            "• Output: Clean plain text only — no markdown, no commentary, no labels."
        )
        user_prompt = (
            "You will receive a raw transcript of a YouTube video. "
            "Your task is to clean and normalize it into fluent, readable text while maintaining complete fidelity to meaning.\n\n"
            "Think step-by-step before writing the cleaned text:\n"
            "1- Normalize structure — merge broken lines into full sentences when punctuation or capitalization suggests continuation.\n"
            "2- Remove disfluencies and filler words ('uh', 'um', 'you know') and single-word interjections ('ok', 'yeah', 'morning'), and stage cues ([APPLAUSE], [MUSIC], etc.).\n"
            "3- Preserve speaker cues: if names exist, keep them (e.g., '[John Doe]: ...'); if missing, retain placeholders like '[Speaker 1]: ...'. "
            "Always use brackets and colons.\n"
            "4- Preserve all technical, code, and math expressions exactly as given — do not alter symbols, equations, or code blocks.\n"
            "5- Rebuild paragraphs for natural readability: group sentences by topic or idea. "
            "Aim for 6-10 sentences per paragraph when possible; insert double newlines between paragraphs.\n"
            "6- Keep the same language as the input. Do not translate.\n"
            "7- Output ONLY the final cleaned text — plain UTF-8 with no markdown, no JSON, and no comments.\n\n"
            "EXAMPLE OUTPUT (abbreviated):\n"
            "[Speaker 1]: Big-O notation describes the asymptotic behavior of algorithms. "
            "It measures how runtime grows as input size increases.\n\n"
            "[Speaker 2]: Exactly. So when we say O(n²), we're talking about how the algorithm's time complexity scales "
            "quadratically with input size.\n\n"
            "BAD EXAMPLES:\n"
            "- Adding explanations, summaries, or extra commentary.\n"
            "- Wrapping text in markdown fences or JSON.\n"
            "- Altering or removing code, math, or technical expressions.\n\n"
            "Now process the transcript below and output only the cleaned text:\n\n"
            f"{str(raw_text)[:120000]}"
        )
        return system_prompt, user_prompt

    @handle_errors(fallback="", log_traceback=True, reraise=False)
    def clean(self, raw_text: str) -> str:
        """
        Clean a raw transcript into fluent, readable text.
        
        Uses dynamic prompt selection via get_prompts() which:
        1. Tries registry prompt with specific prompt_id
        2. Falls back to registry default for TranscriptCleanAgent
        3. Falls back to hardcoded build_prompts()
        
        Args:
            raw_text: The raw transcript text to clean
            
        Returns:
            Cleaned transcript text
        """
        # Guard against extremely long inputs by trimming with a clear marker.
        # Splitting is handled at the stage level; this trim protects single-call usage.
        max_chars = 16000
        text = raw_text if len(raw_text) <= max_chars else raw_text[:max_chars]
        
        # Use get_prompts() for dynamic prompt selection with fallback
        system_prompt, user_prompt = self.get_prompts(raw_text=text)
        out = self.call_model(system_prompt, user_prompt)
        return out or ""
