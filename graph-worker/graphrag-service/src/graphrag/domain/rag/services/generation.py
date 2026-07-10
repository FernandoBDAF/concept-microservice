import os
import time
from typing import Any, Dict, Iterable, List
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize generation service metrics
_rag_generation_calls = Counter(
    "rag_generation_calls", "Number of generation calls", labels=["method"]
)
_rag_generation_errors = Counter(
    "rag_generation_errors", "Number of generation errors", labels=["method"]
)
_rag_generation_duration = Histogram(
    "rag_generation_duration_seconds", "Generation call duration", labels=["method"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_rag_generation_calls)
_registry.register(_rag_generation_errors)
_registry.register(_rag_generation_duration)


@handle_errors(fallback=lambda contexts, question: "\n\n".join(f"[ctx] {c.get('text','')[:500]}" for c in contexts), log_traceback=True, reraise=False)
def answer_with_openai(contexts: List[Dict[str, Any]], question: str) -> str:
    start_time = time.time()
    labels = {"method": "answer_with_openai"}
    _rag_generation_calls.inc(labels=labels)
    
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        joined = "\n\n".join(f"[ctx] {c.get('text','')[:500]}" for c in contexts)
        duration = time.time() - start_time
        _rag_generation_duration.observe(duration, labels=labels)
        return f"Context (no LLM configured):\n\n{joined}"
    try:
        from openai import OpenAI

        client = OpenAI(api_key=api_key)
        messages = [
            {
                "role": "system",
                "content": "You are a precise educational assistant. Answer using only the provided context. Cite (video_id:chunk_id).",
            },
            {
                "role": "user",
                "content": "Question: "
                + question
                + "\n\nContext:\n"
                + "\n\n".join(
                    f"({c.get('video_id')}:{c.get('chunk_id')})\n{c.get('embedding_text','')[:1200]}"
                    for c in contexts
                ),
            },
        ]
        resp = client.chat.completions.create(
            model=os.getenv("OPENAI_DEFAULT_MODEL", "gpt-5-nano"), messages=messages
        )
        result = resp.choices[0].message.content.strip()
        duration = time.time() - start_time
        _rag_generation_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _rag_generation_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_generation_duration.observe(duration, labels=labels)
        joined = "\n\n".join(f"[ctx] {c.get('text','')[:500]}" for c in contexts)
        return f"Context (LLM error: {e}):\n\n{joined}"


def stream_answer_with_openai(
    contexts: List[Dict[str, Any]], question: str
) -> Iterable[str]:
    start_time = time.time()
    labels = {"method": "stream_answer_with_openai"}
    _rag_generation_calls.inc(labels=labels)
    
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        duration = time.time() - start_time
        _rag_generation_duration.observe(duration, labels=labels)
        yield "[Streaming disabled: OPENAI_API_KEY not set]"
        return
    try:
        from openai import OpenAI

        client = OpenAI(api_key=api_key)
        messages = [
            {
                "role": "system",
                "content": "You are a precise educational assistant. Answer using only the provided context. Cite (video_id:chunk_id).",
            },
            {
                "role": "user",
                "content": "Question: "
                + question
                + "\n\nContext:\n"
                + "\n\n".join(
                    f"({c.get('video_id')}:{c.get('chunk_id')})\n{c.get('text','')[:1200]}"
                    for c in contexts
                ),
            },
        ]
        stream = client.chat.completions.create(
            model=os.getenv("OPENAI_DEFAULT_MODEL", "gpt-4o-mini"),
            messages=messages,
            stream=True,
        )
        for event in stream:
            delta = getattr(getattr(event, "choices", [None])[0], "delta", None)
            if delta and getattr(delta, "content", None):
                yield delta.content
        duration = time.time() - start_time
        _rag_generation_duration.observe(duration, labels=labels)
    except Exception as e:
        _rag_generation_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_generation_duration.observe(duration, labels=labels)
        yield f"[Streaming error: {e}]"
