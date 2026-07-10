"""
Prompt Registry for Agent Prompt Management

Provides:
- Dynamic prompt loading from database
- Fallback to hardcoded prompts
- Prompt versioning and A/B testing support
- Cache management for performance

Usage:
    from src.lib.prompts import get_prompt_registry
    
    registry = get_prompt_registry()
    prompts = registry.get_prompt("TranscriptCleanAgent", prompt_id="clean_v2")
    if prompts:
        system_prompt, user_template = prompts
"""

import logging
from typing import Any, Dict, List, Optional, Tuple
from datetime import datetime

logger = logging.getLogger(__name__)

# System database and collection for prompts
SYSTEM_DB = "system_data"
PROMPTS_COLLECTION = "agent_prompts"


class PromptRegistry:
    """
    Singleton registry for managing agent prompts.
    
    Loads prompts from MongoDB on first access and caches them in memory.
    Supports prompt versioning, A/B testing, and default fallbacks.
    """
    
    _instance: Optional["PromptRegistry"] = None
    _prompts: Dict[str, Dict[str, Any]] = {}
    _prompts_by_agent: Dict[str, List[Dict[str, Any]]] = {}
    _loaded: bool = False
    _last_load: Optional[datetime] = None
    
    def __new__(cls) -> "PromptRegistry":
        """Singleton pattern - ensure only one instance exists."""
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance
    
    def _ensure_loaded(self) -> None:
        """Load prompts from database if not already loaded."""
        if not self._loaded:
            self._load_prompts()
    
    def _load_prompts(self) -> None:
        """
        Load all prompts from the database into memory.
        
        Organizes prompts by:
        - prompt_id for direct lookup
        - agent_type for listing available prompts per agent
        """
        try:
            from src.infrastructure.database.mongodb import get_mongo_client
            
            client = get_mongo_client()
            collection = client[SYSTEM_DB][PROMPTS_COLLECTION]
            
            # Fetch all active prompts
            cursor = collection.find({"active": {"$ne": False}})
            
            self._prompts = {}
            self._prompts_by_agent = {}
            
            for doc in cursor:
                prompt_id = doc.get("prompt_id")
                agent_type = doc.get("agent_type")
                
                if not prompt_id or not agent_type:
                    logger.warning(f"Skipping prompt without prompt_id or agent_type: {doc.get('_id')}")
                    continue
                
                # Store full prompt document
                prompt_data = {
                    "prompt_id": prompt_id,
                    "agent_type": agent_type,
                    "name": doc.get("name", prompt_id),
                    "description": doc.get("description", ""),
                    "version": doc.get("version", "1.0"),
                    "system_prompt": doc.get("system_prompt", ""),
                    "user_prompt_template": doc.get("user_prompt_template", ""),
                    "is_default": doc.get("is_default", False),
                    "tags": doc.get("tags", []),
                    "performance_metrics": doc.get("performance_metrics", {}),
                    "created_at": doc.get("created_at"),
                    "updated_at": doc.get("updated_at"),
                }
                
                # Index by prompt_id
                self._prompts[prompt_id] = prompt_data
                
                # Index by agent_type
                if agent_type not in self._prompts_by_agent:
                    self._prompts_by_agent[agent_type] = []
                self._prompts_by_agent[agent_type].append(prompt_data)
            
            self._loaded = True
            self._last_load = datetime.utcnow()
            
            total_prompts = len(self._prompts)
            total_agents = len(self._prompts_by_agent)
            logger.info(
                f"PromptRegistry loaded {total_prompts} prompts for {total_agents} agent types"
            )
            
        except Exception as e:
            logger.warning(f"Failed to load prompts from database: {e}")
            # Mark as loaded to avoid repeated failures
            self._loaded = True
            self._prompts = {}
            self._prompts_by_agent = {}
    
    def get_prompt(
        self,
        agent_type: str,
        prompt_id: Optional[str] = None
    ) -> Optional[Tuple[str, str]]:
        """
        Get prompt templates for an agent.
        
        Lookup priority:
        1. Specific prompt_id if provided
        2. Default prompt for agent_type (is_default=True)
        3. None (caller should fall back to hardcoded)
        
        Args:
            agent_type: Class name of the agent (e.g., "TranscriptCleanAgent")
            prompt_id: Specific prompt ID to retrieve (optional)
            
        Returns:
            Tuple of (system_prompt, user_prompt_template) or None if not found
        """
        self._ensure_loaded()
        
        # 1. Try specific prompt_id
        if prompt_id and prompt_id in self._prompts:
            prompt = self._prompts[prompt_id]
            # Verify it's for the correct agent type
            if prompt.get("agent_type") == agent_type:
                return (prompt["system_prompt"], prompt["user_prompt_template"])
            else:
                logger.warning(
                    f"Prompt {prompt_id} is for {prompt.get('agent_type')}, "
                    f"not {agent_type}. Falling back to default."
                )
        
        # 2. Try default prompt for agent_type
        agent_prompts = self._prompts_by_agent.get(agent_type, [])
        for prompt in agent_prompts:
            if prompt.get("is_default"):
                return (prompt["system_prompt"], prompt["user_prompt_template"])
        
        # 3. No prompt found
        return None
    
    def get_prompt_detail(self, prompt_id: str) -> Optional[Dict[str, Any]]:
        """
        Get full prompt details by ID.
        
        Args:
            prompt_id: Unique prompt identifier
            
        Returns:
            Full prompt document or None if not found
        """
        self._ensure_loaded()
        return self._prompts.get(prompt_id)
    
    def list_prompts_for_agent(self, agent_type: str) -> List[Dict[str, Any]]:
        """
        List all available prompts for an agent type.
        
        Args:
            agent_type: Class name of the agent
            
        Returns:
            List of prompt summaries (id, name, description, is_default, etc.)
        """
        self._ensure_loaded()
        
        prompts = self._prompts_by_agent.get(agent_type, [])
        
        # Return summary info (not full templates)
        return [
            {
                "prompt_id": p["prompt_id"],
                "name": p["name"],
                "description": p["description"],
                "version": p["version"],
                "is_default": p["is_default"],
                "tags": p["tags"],
                "performance_metrics": p.get("performance_metrics", {}),
            }
            for p in prompts
        ]
    
    def list_all_prompts(self) -> Dict[str, List[Dict[str, Any]]]:
        """
        List all prompts grouped by agent type.
        
        Returns:
            Dictionary mapping agent_type to list of prompt summaries
        """
        self._ensure_loaded()
        
        result = {}
        for agent_type in self._prompts_by_agent:
            result[agent_type] = self.list_prompts_for_agent(agent_type)
        return result
    
    def reload(self) -> None:
        """
        Force reload prompts from database.
        
        Use after adding/updating prompts in the database.
        """
        self._loaded = False
        self._prompts = {}
        self._prompts_by_agent = {}
        self._load_prompts()
        logger.info("PromptRegistry reloaded")
    
    def get_cache_info(self) -> Dict[str, Any]:
        """
        Get information about the prompt cache.
        
        Returns:
            Cache statistics including loaded status, prompt count, etc.
        """
        return {
            "loaded": self._loaded,
            "last_load": self._last_load.isoformat() if self._last_load else None,
            "total_prompts": len(self._prompts),
            "agent_types": list(self._prompts_by_agent.keys()),
            "prompts_per_agent": {
                agent: len(prompts) 
                for agent, prompts in self._prompts_by_agent.items()
            },
        }


def get_prompt_registry() -> PromptRegistry:
    """
    Get the singleton PromptRegistry instance.
    
    Returns:
        The global PromptRegistry instance
    """
    return PromptRegistry()

