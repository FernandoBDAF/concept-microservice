import os

from .vector_search import main

query = "What are the most important graph algorithms patterns?"
os.environ["QUERY"] = query
main()