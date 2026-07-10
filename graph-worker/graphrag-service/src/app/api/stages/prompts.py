"""
Prompt Management API

Provides endpoints for listing, retrieving, and testing agent prompts.

Endpoints:
- GET /prompts - List all prompts grouped by agent type
- GET /prompts/{agent_type} - List prompts for specific agent
- GET /prompts/detail/{prompt_id} - Get full prompt details
- POST /prompts/{prompt_id}/test - Test prompt with sample input
"""

import logging
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple

logger = logging.getLogger(__name__)


def list_prompts(agent_type: Optional[str] = None) -> Tuple[Dict[str, Any], int]:
    """
    List available prompts, optionally filtered by agent type.
    
    Args:
        agent_type: Filter by agent type (e.g., "TranscriptCleanAgent")
        
    Returns:
        Response dict with prompts grouped by agent type
    """
    try:
        from src.lib.prompts import get_prompt_registry
        
        registry = get_prompt_registry()
        
        if agent_type:
            # List prompts for specific agent
            prompts = registry.list_prompts_for_agent(agent_type)
            return {
                "agent_type": agent_type,
                "prompts": prompts,
                "total": len(prompts),
                "timestamp": datetime.utcnow().isoformat() + "Z",
            }, 200
        else:
            # List all prompts grouped by agent
            all_prompts = registry.list_all_prompts()
            return {
                "prompts_by_agent": all_prompts,
                "agent_types": list(all_prompts.keys()),
                "total": sum(len(p) for p in all_prompts.values()),
                "cache_info": registry.get_cache_info(),
                "timestamp": datetime.utcnow().isoformat() + "Z",
            }, 200
            
    except Exception as e:
        logger.exception("Error listing prompts")
        return {"error": str(e)}, 500


def get_prompt_detail(prompt_id: str) -> Tuple[Dict[str, Any], int]:
    """
    Get full details for a specific prompt.
    
    Args:
        prompt_id: Unique prompt identifier
        
    Returns:
        Full prompt document including templates
    """
    try:
        from src.lib.prompts import get_prompt_registry
        
        registry = get_prompt_registry()
        prompt = registry.get_prompt_detail(prompt_id)
        
        if prompt:
            return {
                "prompt": prompt,
                "timestamp": datetime.utcnow().isoformat() + "Z",
            }, 200
        else:
            return {
                "error": f"Prompt not found: {prompt_id}",
                "prompt_id": prompt_id,
            }, 404
            
    except Exception as e:
        logger.exception(f"Error getting prompt detail: {prompt_id}")
        return {"error": str(e)}, 500


def test_prompt(
    prompt_id: str,
    test_input: Dict[str, Any],
    model_name: Optional[str] = None
) -> Tuple[Dict[str, Any], int]:
    """
    Test a prompt with sample input.
    
    Runs the prompt through the LLM and returns the output.
    Useful for validating prompts before using them in production.
    
    Args:
        prompt_id: Prompt to test
        test_input: Dictionary of template variables (e.g., {"raw_text": "..."})
        model_name: Optional model override
        
    Returns:
        Test result including input, output, and timing
    """
    try:
        from src.lib.prompts import get_prompt_registry
        import time
        
        registry = get_prompt_registry()
        prompt = registry.get_prompt_detail(prompt_id)
        
        if not prompt:
            return {
                "error": f"Prompt not found: {prompt_id}",
                "prompt_id": prompt_id,
            }, 404
        
        # Get the agent type to instantiate the right agent
        agent_type = prompt.get("agent_type")
        
        # Get prompts from registry
        system_prompt = prompt.get("system_prompt", "")
        user_template = prompt.get("user_prompt_template", "")
        
        # Render user template with test input
        import re
        def replace_placeholder(match):
            key = match.group(1)
            return str(test_input.get(key, f"[{key}]"))
        
        user_prompt = re.sub(r'\{(\w+)\}', replace_placeholder, user_template)
        
        # Truncate for display
        max_display_chars = 500
        system_display = system_prompt[:max_display_chars] + ("..." if len(system_prompt) > max_display_chars else "")
        user_display = user_prompt[:max_display_chars] + ("..." if len(user_prompt) > max_display_chars else "")
        
        # Call the LLM
        start_time = time.time()
        
        try:
            # Use the base agent for testing
            from src.core.base.agent import BaseAgent, BaseAgentConfig
            
            class TestAgent(BaseAgent):
                def build_prompts(self, **kwargs):
                    return system_prompt, user_prompt
            
            config = BaseAgentConfig(model_name=model_name) if model_name else BaseAgentConfig()
            agent = TestAgent(name="PromptTestAgent", config=config)
            
            output = agent.call_model(system_prompt, user_prompt)
            elapsed = time.time() - start_time
            
            return {
                "prompt_id": prompt_id,
                "agent_type": agent_type,
                "model_used": agent.config.model_name,
                "test_input": test_input,
                "rendered_prompts": {
                    "system_prompt": system_display,
                    "user_prompt": user_display,
                },
                "output": output,
                "output_length": len(output) if output else 0,
                "elapsed_seconds": round(elapsed, 2),
                "success": True,
                "timestamp": datetime.utcnow().isoformat() + "Z",
            }, 200
            
        except Exception as e:
            elapsed = time.time() - start_time
            logger.exception(f"LLM call failed for prompt test: {prompt_id}")
            return {
                "prompt_id": prompt_id,
                "agent_type": agent_type,
                "test_input": test_input,
                "rendered_prompts": {
                    "system_prompt": system_display,
                    "user_prompt": user_display,
                },
                "output": None,
                "error": str(e),
                "elapsed_seconds": round(elapsed, 2),
                "success": False,
                "timestamp": datetime.utcnow().isoformat() + "Z",
            }, 200  # Return 200 even on LLM failure (test completed, just failed)
            
    except Exception as e:
        logger.exception(f"Error testing prompt: {prompt_id}")
        return {"error": str(e)}, 500


def reload_prompts() -> Tuple[Dict[str, Any], int]:
    """
    Force reload prompts from database.
    
    Useful after adding or updating prompts in the database.
    
    Returns:
        Cache info after reload
    """
    try:
        from src.lib.prompts import get_prompt_registry
        
        registry = get_prompt_registry()
        registry.reload()
        
        return {
            "message": "Prompts reloaded successfully",
            "cache_info": registry.get_cache_info(),
            "timestamp": datetime.utcnow().isoformat() + "Z",
        }, 200
        
    except Exception as e:
        logger.exception("Error reloading prompts")
        return {"error": str(e)}, 500

