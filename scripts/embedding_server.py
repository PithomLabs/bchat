#!/usr/bin/env python3
"""
Local Embedding Server for RAG Pipeline Testing

This server provides a local embedding endpoint using sentence-transformers.
It's designed for testing and development of the RAG pipeline.

Usage:
    pip install fastapi uvicorn sentence-transformers
    python embedding_server.py

Or with task:
    task embedding:server

The server exposes:
    POST /embed - Generate embeddings for a list of texts
    GET /health - Health check endpoint
    GET /info - Model information

Environment Variables:
    EMBEDDING_MODEL: Model to use (default: all-MiniLM-L6-v2)
    EMBEDDING_PORT: Port to listen on (default: 8001)
"""

import os
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer
import uvicorn
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Configuration
MODEL_NAME = os.getenv("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
PORT = int(os.getenv("EMBEDDING_PORT", "8001"))

# Initialize FastAPI
app = FastAPI(
    title="Local Embedding Server",
    description="Provides text embeddings using sentence-transformers",
    version="1.0.0"
)

# Load model at startup
logger.info(f"Loading embedding model: {MODEL_NAME}")
model = SentenceTransformer(MODEL_NAME)
logger.info(f"Model loaded. Dimension: {model.get_sentence_embedding_dimension()}")


class EmbedRequest(BaseModel):
    """Request body for embedding endpoint."""
    texts: list[str]
    model: str | None = None  # Optional, ignored (uses configured model)


class EmbedResponse(BaseModel):
    """Response body for embedding endpoint."""
    embeddings: list[list[float]]
    model: str
    dimension: int


class ModelInfo(BaseModel):
    """Model information response."""
    model: str
    dimension: int
    max_seq_length: int


@app.get("/health")
def health_check():
    """Health check endpoint."""
    return {"status": "healthy", "model": MODEL_NAME}


@app.get("/info", response_model=ModelInfo)
def model_info():
    """Get model information."""
    return ModelInfo(
        model=MODEL_NAME,
        dimension=model.get_sentence_embedding_dimension(),
        max_seq_length=model.max_seq_length
    )


@app.post("/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest):
    """Generate embeddings for a list of texts."""
    if not req.texts:
        raise HTTPException(status_code=400, detail="texts list cannot be empty")

    if len(req.texts) > 100:
        raise HTTPException(status_code=400, detail="Maximum 100 texts per request")

    try:
        logger.debug(f"Embedding {len(req.texts)} texts")
        embeddings = model.encode(req.texts, convert_to_numpy=True)
        embeddings_list = embeddings.tolist()

        return EmbedResponse(
            embeddings=embeddings_list,
            model=MODEL_NAME,
            dimension=model.get_sentence_embedding_dimension()
        )
    except Exception as e:
        logger.error(f"Embedding error: {e}")
        raise HTTPException(status_code=500, detail=str(e))


if __name__ == "__main__":
    logger.info(f"Starting embedding server on port {PORT}")
    uvicorn.run(app, host="0.0.0.0", port=PORT)
