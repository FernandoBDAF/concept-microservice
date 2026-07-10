"""
Export Handler - Business Logic

Pure functions for graph export operations.
No HTTP handling - that's in router.py

Achievement 7.3: Export & Integration Features
"""

import csv
import logging
import os
import sys
from io import StringIO
from typing import Dict, Any, Optional, List

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from src.infrastructure.database.mongodb import get_mongo_client
from src.domain.services.graphrag.indexes import get_graphrag_collections

logger = logging.getLogger(__name__)


def export_json(
    db_name: str,
    entity_ids: Optional[List[str]] = None,
    community_id: Optional[str] = None,
) -> Dict[str, Any]:
    """
    Export graph as JSON.

    Args:
        db_name: Database name
        entity_ids: Optional list of entity IDs to export (subgraph)
        community_id: Optional community ID to export

    Returns:
        Dictionary with nodes and links
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    entities_collection = collections["entities"]
    relations_collection = collections["relations"]

    # Determine which entities to export
    if community_id:
        # Get community entities
        communities_collection = collections["communities"]
        community = communities_collection.find_one({"community_id": community_id})
        if not community:
            return {"error": "Community not found"}
        entity_ids = community.get("entities", [])
    elif not entity_ids:
        # Export all entities
        entity_ids = [doc["entity_id"] for doc in entities_collection.find({}, {"entity_id": 1})]

    # Get entity details
    entity_docs = list(
        entities_collection.find(
            {"entity_id": {"$in": entity_ids}},
            {
                "entity_id": 1,
                "name": 1,
                "canonical_name": 1,
                "type": 1,
                "description": 1,
                "confidence": 1,
                "source_count": 1,
            },
        )
    )

    # Get relationships between these entities
    relationships = list(
        relations_collection.find({
            "$and": [
                {"subject_id": {"$in": entity_ids}},
                {"object_id": {"$in": entity_ids}},
            ]
        })
    )

    # Build nodes
    nodes = []
    for doc in entity_docs:
        nodes.append({
            "id": doc.get("entity_id"),
            "name": doc.get("name") or doc.get("canonical_name") or doc.get("entity_id"),
            "canonical_name": doc.get("canonical_name"),
            "type": doc.get("type", "OTHER"),
            "description": doc.get("description", ""),
            "confidence": doc.get("confidence", 0.0),
            "source_count": doc.get("source_count", 0),
        })

    # Build links
    links = []
    for rel in relationships:
        links.append({
            "source": rel.get("subject_id"),
            "target": rel.get("object_id"),
            "predicate": rel.get("predicate"),
            "description": rel.get("description", ""),
            "confidence": rel.get("confidence", 0.0),
            "source_count": rel.get("source_count", 0),
        })

    return {
        "nodes": nodes,
        "links": links,
        "metadata": {
            "total_nodes": len(nodes),
            "total_links": len(links),
            "export_type": "community" if community_id else ("subgraph" if entity_ids else "full"),
        },
    }


def export_csv(
    db_name: str,
    entity_ids: Optional[List[str]] = None,
    community_id: Optional[str] = None,
) -> str:
    """
    Export graph as CSV (nodes and links as separate CSV sections).

    Args:
        db_name: Database name
        entity_ids: Optional list of entity IDs to export
        community_id: Optional community ID to export

    Returns:
        CSV string with nodes and links sections
    """
    # Get JSON data first
    json_data = export_json(db_name, entity_ids, community_id)
    if "error" in json_data:
        return f"Error: {json_data['error']}"

    output = StringIO()

    # Write nodes section
    output.write("# NODES\n")
    writer = csv.DictWriter(
        output,
        fieldnames=["id", "name", "canonical_name", "type", "description", "confidence", "source_count"],
    )
    writer.writeheader()
    for node in json_data["nodes"]:
        writer.writerow(node)

    output.write("\n# LINKS\n")
    writer = csv.DictWriter(
        output,
        fieldnames=["source", "target", "predicate", "description", "confidence", "source_count"],
    )
    writer.writeheader()
    for link in json_data["links"]:
        writer.writerow(link)

    return output.getvalue()


def export_graphml(
    db_name: str,
    entity_ids: Optional[List[str]] = None,
    community_id: Optional[str] = None,
) -> str:
    """
    Export graph as GraphML format.

    Args:
        db_name: Database name
        entity_ids: Optional list of entity IDs to export
        community_id: Optional community ID to export

    Returns:
        GraphML XML string
    """
    json_data = export_json(db_name, entity_ids, community_id)
    if "error" in json_data:
        return f"<!-- Error: {json_data['error']} -->"

    output = StringIO()
    output.write('<?xml version="1.0" encoding="UTF-8"?>\n')
    output.write('<graphml xmlns="http://graphml.graphdrawing.org/xmlns">\n')

    # Define attributes
    output.write('  <key id="name" for="node" attr.name="name" attr.type="string"/>\n')
    output.write('  <key id="type" for="node" attr.name="type" attr.type="string"/>\n')
    output.write('  <key id="confidence" for="node" attr.name="confidence" attr.type="double"/>\n')
    output.write('  <key id="predicate" for="edge" attr.name="predicate" attr.type="string"/>\n')
    output.write('  <key id="edge_confidence" for="edge" attr.name="confidence" attr.type="double"/>\n')

    output.write('  <graph id="G" edgedefault="directed">\n')

    # Write nodes
    for node in json_data["nodes"]:
        output.write(f'    <node id="{_escape_xml(node["id"])}">\n')
        output.write(f'      <data key="name">{_escape_xml(node.get("name", ""))}</data>\n')
        output.write(f'      <data key="type">{_escape_xml(node.get("type", ""))}</data>\n')
        output.write(f'      <data key="confidence">{node.get("confidence", 0.0)}</data>\n')
        output.write('    </node>\n')

    # Write edges
    for i, link in enumerate(json_data["links"]):
        output.write(f'    <edge id="e{i}" source="{_escape_xml(link["source"])}" target="{_escape_xml(link["target"])}">\n')
        output.write(f'      <data key="predicate">{_escape_xml(link.get("predicate", ""))}</data>\n')
        output.write(f'      <data key="edge_confidence">{link.get("confidence", 0.0)}</data>\n')
        output.write('    </edge>\n')

    output.write('  </graph>\n')
    output.write('</graphml>\n')

    return output.getvalue()


def export_gexf(
    db_name: str,
    entity_ids: Optional[List[str]] = None,
    community_id: Optional[str] = None,
) -> str:
    """
    Export graph as GEXF format (for Gephi).

    Args:
        db_name: Database name
        entity_ids: Optional list of entity IDs to export
        community_id: Optional community ID to export

    Returns:
        GEXF XML string
    """
    json_data = export_json(db_name, entity_ids, community_id)
    if "error" in json_data:
        return f"<!-- Error: {json_data['error']} -->"

    output = StringIO()
    output.write('<?xml version="1.0" encoding="UTF-8"?>\n')
    output.write('<gexf xmlns="http://www.gexf.net/1.2draft" version="1.2">\n')
    output.write('  <graph mode="static" defaultedgetype="directed">\n')

    # Define attributes
    output.write('    <attributes class="node">\n')
    output.write('      <attribute id="0" title="type" type="string"/>\n')
    output.write('      <attribute id="1" title="confidence" type="float"/>\n')
    output.write('    </attributes>\n')
    output.write('    <attributes class="edge">\n')
    output.write('      <attribute id="0" title="predicate" type="string"/>\n')
    output.write('      <attribute id="1" title="confidence" type="float"/>\n')
    output.write('    </attributes>\n')

    # Write nodes
    output.write('    <nodes>\n')
    for node in json_data["nodes"]:
        output.write(f'      <node id="{_escape_xml(node["id"])}" label="{_escape_xml(node.get("name", ""))}">\n')
        output.write('        <attvalues>\n')
        output.write(f'          <attvalue for="0" value="{_escape_xml(node.get("type", ""))}"/>\n')
        output.write(f'          <attvalue for="1" value="{node.get("confidence", 0.0)}"/>\n')
        output.write('        </attvalues>\n')
        output.write('      </node>\n')
    output.write('    </nodes>\n')

    # Write edges
    output.write('    <edges>\n')
    for i, link in enumerate(json_data["links"]):
        output.write(f'      <edge id="{i}" source="{_escape_xml(link["source"])}" target="{_escape_xml(link["target"])}">\n')
        output.write('        <attvalues>\n')
        output.write(f'          <attvalue for="0" value="{_escape_xml(link.get("predicate", ""))}"/>\n')
        output.write(f'          <attvalue for="1" value="{link.get("confidence", 0.0)}"/>\n')
        output.write('        </attvalues>\n')
        output.write('      </edge>\n')
    output.write('    </edges>\n')

    output.write('  </graph>\n')
    output.write('</gexf>\n')

    return output.getvalue()


def _escape_xml(text: str) -> str:
    """Escape XML special characters."""
    if not text:
        return ""
    return (
        str(text)
        .replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace('"', "&quot;")
        .replace("'", "&apos;")
    )

