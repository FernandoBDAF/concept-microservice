"""
GraphRAG Generation Service

This module implements answer generation for GraphRAG queries using community
summaries and entity context to provide comprehensive answers.
"""

import logging
import time
from typing import Dict, List, Any, Optional
from openai import OpenAI
from src.core.models.graphrag import GraphRAGResponse, GraphRAGQuery
from src.domain.services.graphrag.query import GraphRAGQueryProcessor
from src.domain.services.graphrag.retrieval import GraphRAGRetrievalEngine
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

logger = logging.getLogger(__name__)

# Initialize GraphRAG generation metrics
_graphrag_generation_calls = Counter(
    "graphrag_generation_calls", "Number of GraphRAG generation calls", labels=["method"]
)
_graphrag_generation_errors = Counter(
    "graphrag_generation_errors", "Number of GraphRAG generation errors", labels=["method"]
)
_graphrag_generation_duration = Histogram(
    "graphrag_generation_duration_seconds", "GraphRAG generation call duration", labels=["method"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_graphrag_generation_calls)
_registry.register(_graphrag_generation_errors)
_registry.register(_graphrag_generation_duration)


class GraphRAGGenerationService:
    """
    Service for generating answers to GraphRAG queries using retrieved context.
    """

    def __init__(
        self,
        llm_client: OpenAI,
        model_name: str = "gpt-4o-mini",
        temperature: float = 0.3,
        max_tokens: Optional[int] = None,
        query_processor: Optional[GraphRAGQueryProcessor] = None,
        retrieval_engine: Optional[GraphRAGRetrievalEngine] = None,
    ):
        """
        Initialize the GraphRAG Generation Service.

        Args:
            llm_client: OpenAI client instance
            model_name: Model to use for answer generation
            temperature: Temperature for LLM generation
            max_tokens: Maximum tokens for generation
            query_processor: Optional query processor instance
            retrieval_engine: Optional retrieval engine instance
        """
        self.llm_client = llm_client
        self.model_name = model_name
        self.temperature = temperature
        self.max_tokens = max_tokens
        self.query_processor = query_processor
        self.retrieval_engine = retrieval_engine
        
        # Check if this is a newer model with API restrictions (no temperature, use max_completion_tokens)
        self._is_restricted_model = any(
            prefix in self.model_name for prefix in ["gpt-5", "o1", "o3"]
        )

        # System prompt for answer generation
        self.generation_prompt = """
You are an expert assistant that provides comprehensive answers using GraphRAG (Graph-based Retrieval-Augmented Generation).

Your task is to answer user questions using the provided context from a knowledge graph and community summaries.

## Instructions:

1. **Answer Comprehensively**: Use all relevant information from the context to provide a complete answer
2. **Be Accurate**: Only use information explicitly provided in the context
3. **Be Clear**: Structure your answer logically and clearly
4. **Be Specific**: Include specific details, names, and relationships when available
5. **Be Helpful**: Provide actionable insights and explanations
6. **YouTube Context**: Consider that this information comes from YouTube content, so focus on educational and technical aspects

## Guidelines:
- Start with a direct answer to the question
- Provide supporting details from the context
- Explain relationships between entities when relevant
- Include specific examples or use cases when available
- If the context doesn't contain enough information, say so clearly
- Use the community summaries to provide broader context
- Reference specific entities and their relationships

## Output:
Provide a comprehensive answer that directly addresses the user's question using the provided context.
"""

        logger.info(f"Initialized GraphRAGGenerationService with model {model_name}")

    @handle_errors(fallback="", log_traceback=True, reraise=False)
    def generate_answer(
        self,
        query: str,
        context: str,
        entities: List[Dict[str, Any]],
        communities: List[Dict[str, Any]],
    ) -> str:
        """
        Generate an answer using the provided context.

        Args:
            query: User's question
            context: Retrieved context string
            entities: List of relevant entities
            communities: List of relevant communities

        Returns:
            Generated answer
        """
        start_time = time.time()
        labels = {"method": "generate_answer"}
        _graphrag_generation_calls.inc(labels=labels)
        
        try:
            logger.info(f"Generating answer for query: {query}")

            # Prepare the prompt
            prompt = f"""
            Context Information:
            {context}

            Question: {query}

            Please provide a comprehensive answer using the context information above.
            """

            # Build request params with appropriate parameters for model
            request_params = {
                "model": self.model_name,
                "messages": [
                    {"role": "system", "content": self.generation_prompt},
                    {"role": "user", "content": prompt},
                ],
            }
            # Newer models don't support custom temperature
            if not self._is_restricted_model:
                request_params["temperature"] = self.temperature
            # Use appropriate token parameter - reasoning models need higher limits
            if self._is_restricted_model:
                tokens = self.max_tokens or 8000  # Higher default for reasoning models
                request_params["max_completion_tokens"] = tokens
            else:
                tokens = self.max_tokens or 2000
                request_params["max_tokens"] = tokens
                
            response = self.llm_client.chat.completions.create(**request_params)

            answer = response.choices[0].message.content.strip()

            logger.info(f"Generated answer with {len(answer)} characters")
            duration = time.time() - start_time
            _graphrag_generation_duration.observe(duration, labels=labels)
            return answer
        except Exception as e:
            _graphrag_generation_errors.inc(labels=labels)
            duration = time.time() - start_time
            _graphrag_generation_duration.observe(duration, labels=labels)
            logger.error(f"Error generating answer: {e}")
            return "I apologize, but I encountered an error while generating an answer. Please try again."

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def process_query_with_generation(
        self, query_text: str, db, use_traditional_rag: bool = False
    ) -> Optional[GraphRAGResponse]:
        """
        Process a query and generate an answer using GraphRAG.

        Args:
            query_text: User's query
            db: MongoDB database instance
            use_traditional_rag: Whether to also include traditional RAG results

        Returns:
            GraphRAGResponse object
        """
        start_time = time.time()
        labels = {"method": "process_query_with_generation"}
        _graphrag_generation_calls.inc(labels=labels)
        
        try:
            logger.info(f"Processing query with generation: {query_text}")
            # Initialize components if not provided
            if not self.query_processor:
                self.query_processor = GraphRAGQueryProcessor(
                    self.llm_client, self.model_name
                )

            if not self.retrieval_engine:
                self.retrieval_engine = GraphRAGRetrievalEngine(db)

            # Process query
            processed_query = self.query_processor.process_query(query_text, db)

            # Retrieve context
            retrieval_results = self.retrieval_engine.hybrid_graphrag_search(
                query_text, processed_query.extracted_entities
            )

            # Generate answer
            answer = self.generate_answer(
                query_text,
                retrieval_results["context"],
                retrieval_results["entities"],
                retrieval_results["communities"],
            )

            # Calculate confidence based on context quality
            confidence = self._calculate_answer_confidence(
                retrieval_results["entities"],
                retrieval_results["communities"],
                retrieval_results["context"],
            )

            processing_time = time.time() - start_time

            # Create response
            response = GraphRAGResponse(
                answer=answer,
                entities=retrieval_results["entities"],
                communities=retrieval_results["communities"],
                context_sources=[],  # Could be enhanced to track specific sources
                confidence=confidence,
                processing_time=processing_time,
            )

            logger.info(f"Query processing completed in {processing_time:.2f} seconds")
            duration = time.time() - start_time
            _graphrag_generation_duration.observe(duration, labels=labels)
            return response

        except Exception as e:
            _graphrag_generation_errors.inc(labels=labels)
            duration = time.time() - start_time
            _graphrag_generation_duration.observe(duration, labels=labels)
            logger.error(f"Error processing query: {e}")

            return GraphRAGResponse(
                answer=f"I apologize, but I encountered an error while processing your query: {str(e)}",
                entities=[],
                communities=[],
                context_sources=[],
                confidence=0.0,
                processing_time=duration,  # Use duration, not processing_time (which may be undefined)
            )

    def _calculate_answer_confidence(
        self,
        entities: List[Dict[str, Any]],
        communities: List[Dict[str, Any]],
        context: str,
    ) -> float:
        """
        Calculate confidence score for the generated answer.

        Args:
            entities: List of retrieved entities
            communities: List of retrieved communities
            context: Retrieved context string

        Returns:
            Confidence score between 0 and 1
        """
        # Base confidence on available information
        entity_score = min(len(entities) / 5.0, 1.0)  # Normalize to 5 entities
        community_score = min(len(communities) / 3.0, 1.0)  # Normalize to 3 communities
        context_score = min(len(context) / 2000.0, 1.0)  # Normalize to 2000 characters

        # Weight the scores
        confidence = 0.4 * entity_score + 0.3 * community_score + 0.3 * context_score

        return min(1.0, max(0.0, confidence))

    def generate_comparative_answer(
        self,
        query: str,
        entities: List[Dict[str, Any]],
        communities: List[Dict[str, Any]],
    ) -> str:
        """
        Generate a comparative answer when multiple entities are involved.

        Args:
            query: User's query
            entities: List of entities to compare
            communities: List of relevant communities

        Returns:
            Comparative answer
        """
        logger.info(f"Generating comparative answer for {len(entities)} entities")

        # Build comparison context
        comparison_context = "## Entity Comparison:\n"

        for i, entity in enumerate(entities[:5], 1):  # Limit to 5 entities
            comparison_context += f"\n{i}. **{entity['name']}** ({entity['type']}):\n"
            comparison_context += f"   - Description: {entity['description']}\n"
            comparison_context += (
                f"   - Trust Score: {entity.get('trust_score', 'N/A')}\n"
            )
            comparison_context += (
                f"   - Centrality: {entity.get('centrality_score', 'N/A')}\n"
            )

        # Add community context
        if communities:
            comparison_context += "\n## Community Context:\n"
            for community in communities:
                comparison_context += f"\n### {community['title']}\n"
                comparison_context += f"{community['summary']}\n"

        # Generate comparative answer
        prompt = f"""
Context Information:
{comparison_context}

Question: {query}

Please provide a comparative analysis using the entity information above.
"""

        try:
            # Build request params with appropriate parameters for model
            request_params = {
                "model": self.model_name,
                "messages": [
                    {"role": "system", "content": self.generation_prompt},
                    {"role": "user", "content": prompt},
                ],
            }
            # Newer models don't support custom temperature
            if not self._is_restricted_model:
                request_params["temperature"] = self.temperature
            # Use appropriate token parameter - reasoning models need higher limits
            if self._is_restricted_model:
                tokens = self.max_tokens or 8000
                request_params["max_completion_tokens"] = tokens
            else:
                tokens = self.max_tokens or 2000
                request_params["max_tokens"] = tokens
                
            response = self.llm_client.chat.completions.create(**request_params)

            answer = response.choices[0].message.content.strip()
            logger.info(f"Generated comparative answer with {len(answer)} characters")
            return answer

        except Exception as e:
            logger.error(f"Error generating comparative answer: {e}")
            return "I apologize, but I encountered an error while generating a comparative answer."

    def generate_explanatory_answer(
        self,
        query: str,
        entities: List[Dict[str, Any]],
        communities: List[Dict[str, Any]],
    ) -> str:
        """
        Generate an explanatory answer focusing on relationships and context.

        Args:
            query: User's query
            entities: List of relevant entities
            communities: List of relevant communities

        Returns:
            Explanatory answer
        """
        logger.info(f"Generating explanatory answer for query: {query}")

        # Build explanatory context
        explanatory_context = "## Entity Relationships:\n"

        for entity in entities[:5]:
            explanatory_context += f"\n**{entity['name']}** ({entity['type']}):\n"
            explanatory_context += f"- {entity['description']}\n"
            explanatory_context += (
                f"- Trust Score: {entity.get('trust_score', 'N/A')}\n"
            )

        # Add community context for broader explanation
        if communities:
            explanatory_context += "\n## Broader Context:\n"
            for community in communities:
                explanatory_context += f"\n### {community['title']}\n"
                explanatory_context += f"{community['summary']}\n"

        # Generate explanatory answer
        prompt = f"""
Context Information:
{explanatory_context}

Question: {query}

Please provide a detailed explanation using the context information above.
Focus on relationships, causes, and broader context.
"""

        try:
            # Build request params with appropriate parameters for model
            request_params = {
                "model": self.model_name,
                "messages": [
                    {"role": "system", "content": self.generation_prompt},
                    {"role": "user", "content": prompt},
                ],
            }
            # Newer models don't support custom temperature
            if not self._is_restricted_model:
                request_params["temperature"] = self.temperature
            # Use appropriate token parameter - reasoning models need higher limits
            if self._is_restricted_model:
                tokens = self.max_tokens or 8000
                request_params["max_completion_tokens"] = tokens
            else:
                tokens = self.max_tokens or 2000
                request_params["max_tokens"] = tokens
                
            response = self.llm_client.chat.completions.create(**request_params)

            answer = response.choices[0].message.content.strip()
            logger.info(f"Generated explanatory answer with {len(answer)} characters")
            return answer

        except Exception as e:
            logger.error(f"Error generating explanatory answer: {e}")
            return "I apologize, but I encountered an error while generating an explanatory answer."

    def get_generation_stats(self) -> Dict[str, Any]:
        """
        Get statistics about the generation service.

        Returns:
            Dictionary containing generation statistics
        """
        return {
            "model_name": self.model_name,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "has_query_processor": self.query_processor is not None,
            "has_retrieval_engine": self.retrieval_engine is not None,
        }
