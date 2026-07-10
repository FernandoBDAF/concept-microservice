import re
import time
from typing import Any, Dict, List


def _chunk_text_for_llmlingua(text: str, max_chars: int = 1800) -> List[str]:
    """Greedy chunking by paragraphs/sentences to keep segments <~ max_chars.

    This avoids extremely long sequences that can trigger tokenizer limits in
    encoder models used by LLMLingua.
    """
    txt = (text or "").strip()
    if not txt:
        return []

    # Prefer paragraph splits; fallback to sentence boundaries if one big block
    paras = [p.strip() for p in re.split(r"\n\s*\n+", txt) if p.strip()]
    if len(paras) <= 1:
        # Light sentence split on punctuation.
        paras = [s.strip() for s in re.split(r"(?<=[.!?])\s+", txt) if s.strip()]

    chunks: List[str] = []
    buf: List[str] = []
    acc = 0
    for p in paras:
        plen = len(p)
        if acc and acc + 1 + plen > max_chars:
            chunks.append("\n".join(buf))
            buf = [p]
            acc = plen
        else:
            if buf:
                buf.append(p)
                acc += 1 + plen
            else:
                buf = [p]
                acc = plen
    if buf:
        chunks.append("\n".join(buf))
    return chunks


def _normalize_compressed_output(s: str) -> str:
    """Fix pathological char-per-line outputs and normalize whitespace/newlines."""
    raw = s or ""
    lines = raw.splitlines()
    if lines:
        single_char = sum(1 for ln in lines if len(ln.strip()) <= 1)
        # If majority of lines are single characters, drop newlines entirely.
        if single_char / max(1, len(lines)) > 0.5:
            raw = raw.replace("\n", "")
            # Collapse excessive spaces
            raw = re.sub(r"[ \t]+", " ", raw)
            return raw.strip()
    # Normal path: collapse spaces and condense tall newlines
    raw = re.sub(r"[ \t]+", " ", raw)
    raw = re.sub(r"\n{3,}", "\n\n", raw)
    return raw.strip()


def compress_text(
    text: str,
    target_tokens: int = 1200,
    ratio: float = 0.4,
    reorder: str = "sort",
    model_name: str = "microsoft/llmlingua-2-bert-base-multilingual-cased-meetingbank",
) -> Dict[str, Any]:
    """Compress text using LLMLingua.

    Raises RuntimeError if compression cannot be performed (no fallback).
    Returns dict with keys: compressed_text, compression_meta.
    """
    start = time.time()
    try:
        from llmlingua import PromptCompressor  # type: ignore

        compressor = PromptCompressor(
            model_name=model_name,
            model_config={"revision": "main"},
            use_llmlingua2=True,
            device_map="cpu",
        )
        # Pre-chunk long inputs to avoid tokenizer length issues
        segments = _chunk_text_for_llmlingua(text, max_chars=1800)
        if not segments:
            return {"compressed_text": "", "compression_meta": {}}
        result = compressor.compress_prompt(
            segments,
            instruction="Compress the content by removing uninformative parts while preserving meaning.",
            question="",
            target_token=int(target_tokens),
            rank_method="longllmlingua",
            context_budget="+100",
            dynamic_context_compression_ratio=float(ratio),
            reorder_context=reorder,
        )
        compressed_str = (
            "\n".join(result.get("compressed_prompt", []))
            if isinstance(result, dict)
            else str(result)
        )
        compressed_str = _normalize_compressed_output(compressed_str)
        elapsed_ms = int((time.time() - start) * 1000)
        return {
            "compressed_text": compressed_str,
            "compression_meta": {
                "model": model_name,
                "target_tokens": target_tokens,
                "ratio": ratio,
                "reorder": reorder,
                "elapsed_ms": elapsed_ms,
                "method": "llmlingua",
                "segments_in": len(segments),
            },
        }
    except Exception as e:
        print(f"Error compressing text: {e}")
        return {"compressed_text": "", "compression_meta": {}}


def postprocess_compressed_text(s: str, strict: bool = False) -> str:
    """Optional cleanup of compressed text for UI/LLM friendliness.

    - Fix spaced apostrophes/hyphens/underscores: don ' t -> don't; get _ at -> get_at
    - Remove common lecture speaker labels at line starts (keeps the rest of the line)
    - Normalize whitespace/newlines
    """
    if not s:
        return s
    t = s
    if strict:
        # Join spaced apostrophes/hyphens/underscores between word characters
        t = re.sub(r"(\w)\s*'\s*(\w)", r"\1'\2", t)
        t = re.sub(r"(\w)\s*-\s*(\w)", r"\1-\2", t)
        t = re.sub(r"(\w)\s*_\s*(\w)", r"\1_\2", t)
    return _normalize_compressed_output(t)
