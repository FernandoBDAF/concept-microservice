import re
from typing import Any, Dict, List, Tuple

from src.core.types.text import normalize_newlines, strip_stray_backslashes


def split_units(text: str) -> List[str]:
    t = (text or "").replace("\r\n", "\n").replace("\r", "\n")
    if re.search(r"\n{2,}", t):
        return [p.strip() for p in re.split(r"\n{2,}", t) if p.strip()]
    if "\n" in t:
        return [p.strip() for p in t.split("\n") if p.strip()]
    sents = [s.strip() for s in re.split(r"(?<=[.!?])\s+", t) if s.strip()]
    return sents if sents else ([t] if t else [])


def pack_units(units: List[str], target_chars: int = 1800) -> List[str]:
    buffer: List[str] = []
    size = 0
    packed: List[str] = []
    for u in units:
        if size + len(u) + 2 <= target_chars:
            buffer.append(u)
            size += len(u) + 2
        else:
            if buffer:
                packed.append("\n\n".join(buffer))
            buffer = [u]
            size = len(u)
    if buffer:
        packed.append("\n\n".join(buffer))
    return packed


def normalize_llm_segments(raw_segments: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    segments: List[Dict[str, Any]] = []
    for s in raw_segments:
        seg_text = normalize_newlines(s.get("text", "") or "")
        seg_text = strip_stray_backslashes(seg_text)
        segments.append(
            {
                "start": float(s.get("start", 0.0) or 0.0),
                "end": float(s.get("end", 0.0) or 0.0),
                "text": seg_text,
                "tags": s.get("tags", []),
                "named_entities": s.get("named_entities", []),
                "topics": s.get("topics", []),
                "keyphrases": s.get("keyphrases", []),
                "code_blocks": s.get("code_blocks", []),
                "difficulty": s.get("difficulty"),
                "entities": s.get("entities", []),
            }
        )
    return segments


# --- Structured payload normalization for per-chunk schema ---
def _clamp01(x: float) -> float:
    try:
        v = float(x)
    except Exception:
        return 0.0
    return 0.0 if v < 0 else 1.0 if v > 1 else v


_ENTITY_TYPES = {
    "Person",
    "Organization",
    "Event",
    "Concept",
    "Location",
    "Product",
    "Other",
}
_CONCEPT_CATS = {"Strategy", "Emotion", "Technical", "Theme", "Topic", "Other"}
_SENTIMENTS = {"positive", "neutral", "negative", None}
_TONES = {"analytical", "casual", "instructional", "narrative", "humorous", None}


def normalize_enrich_payload_for_chunk(raw: Dict[str, Any]) -> Dict[str, Any]:
    data = dict(raw or {})
    out: Dict[str, Any] = {}
    out["summary"] = str(data.get("summary", "") or "")[:5000]

    ents: List[Dict[str, Any]] = []
    for e in data.get("entities", []) or []:
        name = str(e.get("name", "") or "").strip()
        if not name:
            continue
        et = e.get("type")
        et = et if et in _ENTITY_TYPES else "Other"
        rel = _clamp01(e.get("relevance", 0.0))
        src = e.get("source_fragment")
        ents.append(
            {
                "name": name,
                "type": et,
                "relevance": rel,
                "source_fragment": (src or None),
            }
        )
    out["entities"] = ents

    cons: List[Dict[str, Any]] = []
    for c in data.get("concepts", []) or []:
        name = str(c.get("name", "") or "").strip()
        if not name:
            continue
        cat = c.get("category")
        cat = cat if cat in _CONCEPT_CATS else "Other"
        conf = _clamp01(c.get("confidence", 0.0))
        cons.append({"name": name, "category": cat, "confidence": conf})
    out["concepts"] = cons

    rels: List[Dict[str, Any]] = []
    for r in data.get("relations", []) or []:
        sub = str(r.get("subject", "") or "").strip()
        pred = str(r.get("predicate", "") or "").strip()
        obj = str(r.get("object", "") or "").strip()
        if not (sub and pred and obj):
            continue
        conf = _clamp01(r.get("confidence", 0.0))
        rels.append(
            {"subject": sub, "predicate": pred, "object": obj, "confidence": conf}
        )
    out["relations"] = rels

    out["temporal_references"] = [
        {
            "date": str(t.get("date", "") or ""),
            "context": str(t.get("context", "") or ""),
        }
        for t in (data.get("temporal_references", []) or [])
        if (t.get("date") or t.get("context"))
    ]

    out["numerical_data"] = [
        {
            "value": str(n.get("value", "") or ""),
            "type": str(n.get("type", "") or "other"),
            "context": str(n.get("context", "") or ""),
        }
        for n in (data.get("numerical_data", []) or [])
        if (n.get("value") or n.get("context"))
    ]

    out["visual_cues"] = [
        {
            "type": str(v.get("type", "other") or "other"),
            "text": (v.get("text") or None),
        }
        for v in (data.get("visual_cues", []) or [])
        if v.get("type") or v.get("text")
    ]

    ctx = data.get("context", {}) or {}
    sentiment = ctx.get("sentiment") if ctx.get("sentiment") in _SENTIMENTS else None
    tone = ctx.get("tone") if ctx.get("tone") in _TONES else None
    out["context"] = {
        "speaker": ctx.get("speaker") or None,
        "sentiment": sentiment,
        "tone": tone,
        "language": ctx.get("language") or None,
        "tags": [str(t).strip() for t in (ctx.get("tags") or []) if str(t).strip()],
    }
    return out
