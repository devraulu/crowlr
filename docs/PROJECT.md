The initial phase of this project involved writing a concurrent web crawler in Go. The crawled data is stored in Postgres and indexed for full-text search.

The idea now is to add a RAG on top of it and allow the users to ask questions about the crawled set. Let's first try to outline the functionality more clearly.
The user should be able to ask questions about the crawled set, the agent should gather the necessary context (preferably with hybrid search), and then answer the question, if possible, and show results. If the user has a short and vague search or question, the agent should try to answer from the context, we should first try to search as the user has specified and try to find context in the crawled set, if it doesn't return any meaningful results, the agent should check the user's query for mistakes and typos, fix them and try again to find context.

To do this we generate embeddings of the crawled set using the `nomic-embed-text` model and store them in Postgres using the pgvector extension and create an HNSW index with cosine distance for aproximate nearest neighbor search.


